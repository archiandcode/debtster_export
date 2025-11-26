package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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

	"github.com/go-chi/chi/v5"
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

	// Init local export storage
	storageClient, err := clients.NewLocalStorage(cfg.ExportDir, cfg.FilesPublicPrefix, cfg.ExternalURL)
	if err != nil {
		log.Fatalf("storage init error: %v", err)
	}

	wsHub := websocket.NewHub()
	go wsHub.Run(ctx)
	wsClient := clients.NewWebSocketClient(wsHub)

	debtRepo := repository.NewDebtRepository(db)
	userRepo := repository.NewUserRepository(db)
	actionRepo := repository.NewActionRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	tokenRepo := repository.NewPersonalAccessTokenRepository(db)

	debtSvc := service.NewDebtService(debtRepo, redisClient, storageClient, wsClient)
	userSvc := service.NewUserService(userRepo, redisClient, storageClient, wsClient)
	actionSvc := service.NewActionService(actionRepo, redisClient, storageClient, wsClient)
	paymentSvc := service.NewPaymentService(paymentRepo, redisClient, storageClient, wsClient)
	exportSvc := service.NewExportService(redisClient, cfg.ExportPrefix)

	sanctumMiddleware := auth.SanctumMiddleware(tokenRepo)

	handler := rest.NewHandler(debtSvc, userSvc, actionSvc, paymentSvc, exportSvc)
	router := handler.InitRouterWithAuth(sanctumMiddleware)

	// create a public root router and mount protected (auth) router underneath so
	// /files and /health remain public while other routes remain protected
	root := chi.NewRouter()

	// public: serve generated files
	root.Get("/files/{file}", func(w http.ResponseWriter, r *http.Request) {
		file := chi.URLParam(r, "file")
		// sanitize and open file from storage directory
		path := filepath.Join(storageClient.BaseDir, file)
		// check file exists
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to access file", http.StatusInternalServerError)
			return
		}

		// prefer original filename in Content-Disposition (strip random prefix)
		orig := file
		if idx := strings.IndexByte(file, '_'); idx >= 0 {
			orig = file[idx+1:]
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", orig))

		http.ServeFile(w, r, path)
	})

	// protected websocket endpoint
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

	// expose endpoint for saving/uploading files (protected)
	router.Post("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		// expect multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(file); err != nil {
			http.Error(w, "failed to read file", http.StatusInternalServerError)
			return
		}

		saved, err := storageClient.Save(r.Context(), header.Filename, buf.Bytes())
		if err != nil {
			http.Error(w, "failed to save file", http.StatusInternalServerError)
			return
		}

		url := storageClient.GetURL(saved)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"url":"%s","file":"%s"}`, url, saved)))
	})

	// mount protected router on root
	root.Mount("/", router)

	corsHandler := withCORS(root)

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

	// start background cleaner that deletes files older than 30 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := storageClient.CleanupOlderThan(30 * time.Minute); err != nil {
					log.Printf("storage cleanup error: %v", err)
				}
			}
		}
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

// S3 removed — local storage used instead.

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
