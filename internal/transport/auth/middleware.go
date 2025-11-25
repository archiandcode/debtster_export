package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"debtster-export/internal/domain"
	"debtster-export/internal/repository"
)

type ctxKey string

const UserIDKey ctxKey = "userID"

func SanctumMiddleware(tokenRepo *repository.PersonalAccessTokenRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Debug: request info
			fmt.Printf("[AUTH] request %s %s from %s, UA=%q\n", r.Method, r.URL.String(), r.RemoteAddr, r.UserAgent())

			// Try Authorization header first
			authHeader := r.Header.Get("Authorization")
			var pat *domain.PersonalAccessToken
			if authHeader != "" {
				fmt.Printf("[AUTH] Authorization header present: %q\n", authHeader)
			}
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				plainToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				fmt.Printf("[AUTH] trying token from header: %q\n", plainToken)
				if plainToken != "" {
					p, err := tokenRepo.FindTokenByPlainToken(r.Context(), plainToken)
					if err != nil {
						fmt.Printf("[AUTH] token lookup (header) error: %v\n", err)
					} else {
						pat = p
						fmt.Printf("[AUTH] token found by header: id=%d userID=%d expiresAt=%v abilities=%q\n", pat.ID, pat.UserID, pat.ExpiresAt, pat.Abilities)
					}
				}
			}

			// If not found in header, try token query parameter (useful for websocket connections)
			if pat == nil {
				token := r.URL.Query().Get("token")
				if token != "" {
					fmt.Printf("[AUTH] trying token from query param: %q\n", token)
					p, err := tokenRepo.FindTokenByPlainToken(r.Context(), token)
					if err != nil {
						fmt.Printf("[AUTH] token lookup (query) error: %v\n", err)
					} else {
						pat = p
						fmt.Printf("[AUTH] token found by query: id=%d userID=%d expiresAt=%v abilities=%q\n", pat.ID, pat.UserID, pat.ExpiresAt, pat.Abilities)
					}
				} else {
					fmt.Printf("[AUTH] no token in query param\n")
				}
			}

			if pat == nil {
				fmt.Printf("[AUTH] no valid token found -> 401\n")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if pat.ExpiresAt != nil && pat.ExpiresAt.Before(time.Now()) {
				fmt.Printf("[AUTH] token expired at %v -> 401\n", pat.ExpiresAt)
				http.Error(w, "Token expired", http.StatusUnauthorized)
				return
			}

			fmt.Printf("[AUTH] authenticated user=%d (token id=%d)\n", pat.UserID, pat.ID)

			ctx := context.WithValue(r.Context(), UserIDKey, pat.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) (int64, error) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	if !ok {
		return 0, errors.New("userID not found in context")
	}
	return userID, nil
}
