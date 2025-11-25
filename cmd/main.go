package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"debtster-export/internal/clients"
	"debtster-export/internal/config"
	"debtster-export/internal/repository"
	"debtster-export/internal/service"
	"debtster-export/internal/transport/auth"
	"debtster-export/internal/transport/rest"
	"debtster-export/internal/transport/websocket"
	"debtster-export/pkg/database/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system env or defaults")
	}

	// top-level context which we can cancel on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.Load()

	db := mustInitPostgres(cfg.Postgres)
	defer postgres.Close(db)

	redisClient := mustInitRedis(cfg.Redis)
	defer redisClient.Close()

	s3Client := mustInitS3(ctx, cfg.S3)

	wsHub := websocket.NewHub()
	go wsHub.Run(ctx)
	wsClient := clients.NewWebSocketClient(wsHub)

	debtRepo := repository.NewDebtRepository(db)
	userRepo := repository.NewUserRepository(db)
	actionRepo := repository.NewActionRepository(db)
	tokenRepo := repository.NewPersonalAccessTokenRepository(db)

	debtSvc := service.NewDebtService(debtRepo, redisClient, s3Client, wsClient)
	userSvc := service.NewUserService(userRepo, redisClient, s3Client, wsClient)
	actionSvc := service.NewActionService(actionRepo, redisClient, s3Client, wsClient)
	exportSvc := service.NewExportService(redisClient, cfg.ExportPrefix)

	sanctumMiddleware := auth.SanctumMiddleware(tokenRepo)

	handler := rest.NewHandler(debtSvc, userSvc, actionSvc, exportSvc)
	router := handler.InitRouterWithAuth(sanctumMiddleware)

	router.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		userID, err := auth.GetUserID(r.Context())
		if err != nil {
			token := r.URL.Query().Get("token")
			if token != "" {
				pat, err2 := tokenRepo.FindTokenByPlainToken(r.Context(), token)
				if err2 != nil {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				if pat.ExpiresAt != nil && pat.ExpiresAt.Before(time.Now()) {
					http.Error(w, "Token expired", http.StatusUnauthorized)
					return
				}
				userID = pat.UserID
			} else {
				// fallback for tests: allow ?user_id=1
				userIDStr := r.URL.Query().Get("user_id")
				if userIDStr == "" {
					http.Error(w, "user_id required", http.StatusBadRequest)
					return
				}
				parsed, err2 := strconv.ParseInt(userIDStr, 10, 64)
				if err2 != nil {
					http.Error(w, "invalid user_id", http.StatusBadRequest)
					return
				}
				userID = parsed
			}
		}

		log.Printf("WS connected: user_id=%d", userID)
		wsHub.HandleWebSocket(w, r, userID)
	})

	corsHandler := withCORS(router)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      corsHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run HTTP server in goroutine so we can listen for shutdown signals
	srvErr := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on :%s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
			return
		}
		srvErr <- nil
	}()

	// Listen for OS shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-srvErr:
		if err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	case sig := <-stop:
		log.Printf("Shutdown signal received: %v", sig)

		// Give server up to 10 seconds to finish ongoing requests
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server Shutdown error: %v", err)
		}

		// Cancel top-level context so background services (websocket hub) stop
		cancel()

		// Close database & redis explicitly to free resources promptly
		postgres.Close(db)
		redisClient.Close()

		log.Println("Shutdown complete")
	}
}

func mustInitPostgres(cfg config.PostgresConfig) *sql.DB {
	db, err := postgres.NewPostgresConnection(postgres.ConnectionInfo{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.User,
		DBName:   cfg.DBName,
		SSLMode:  cfg.SSLMode,
		Password: cfg.Password,
	})
	if err != nil {
		log.Fatalf("postgres init error: %v", err)
	}
	return db
}

func mustInitRedis(cfg config.RedisConfig) *clients.RedisClient {
	client, err := clients.NewRedisClient(clients.RedisConfig{
		Addr:        cfg.Addr,
		Password:    cfg.Password,
		DB:          cfg.DB,
		MaxRetries:  cfg.MaxRetries,
		DialTimeout: time.Duration(cfg.DialTimeout) * time.Second,
		Timeout:     time.Duration(cfg.Timeout) * time.Second,
		Prefix:      cfg.Prefix,
	})
	if err != nil {
		log.Fatalf("redis init error: %v", err)
	}
	return client
}

func mustInitS3(ctx context.Context, cfg config.S3Config) *clients.S3Client {
	client, err := clients.NewS3Client(ctx, clients.S3Config{
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		Bucket:          cfg.Bucket,
		UseSSL:          cfg.UseSSL,
		Region:          cfg.Region,
		Prefix:          cfg.Prefix,
	})
	if err != nil {
		log.Fatalf("s3 init error: %v", err)
	}
	return client
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")

			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
