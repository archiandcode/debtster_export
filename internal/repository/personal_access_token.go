package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"debtster-export/internal/domain"
)

const userTokenableType = "App\\Infrastructure\\Persistence\\Models\\User"

type PersonalAccessTokenRepository struct {
	db *sql.DB
}

func NewPersonalAccessTokenRepository(db *sql.DB) *PersonalAccessTokenRepository {
	return &PersonalAccessTokenRepository{db: db}
}

func (r *PersonalAccessTokenRepository) FindTokenByPlainToken(ctx context.Context, plainToken string) (*domain.PersonalAccessToken, error) {
	plainToken = strings.TrimSpace(plainToken)
	if plainToken == "" {
		return nil, errors.New("empty token")
	}

	log.Printf("[TOKEN] plainToken=%q", plainToken)

	var (
		tokenID   *int64
		tokenPart string
	)

	if idx := strings.Index(plainToken, "|"); idx > 0 {
		idStr := plainToken[:idx]
		tokenPart = plainToken[idx+1:]

		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			tokenID = &id
		} else {
			log.Printf("[TOKEN] failed to parse id %q: %v", idStr, err)
		}
	} else {
		tokenPart = plainToken
	}

	log.Printf("[TOKEN] parsed id=%v tokenPart=%q", tokenID, tokenPart)

	sum := sha256.Sum256([]byte(tokenPart))
	hashStr := fmt.Sprintf("%x", sum)

	log.Printf("[TOKEN] computed sha256=%s", hashStr)

	var pat domain.PersonalAccessToken

	if tokenID != nil {
		query := `
			SELECT id, token, tokenable_id, abilities, expires_at
			FROM personal_access_tokens
			WHERE id = $1
			  AND tokenable_type = $2
			  AND (expires_at IS NULL OR expires_at > $3)
		`

		log.Printf("[TOKEN] query by id=%d", *tokenID)

		err := r.db.QueryRowContext(ctx, query, *tokenID, userTokenableType, time.Now()).Scan(
			&pat.ID,
			&pat.TokenHash,
			&pat.UserID,
			&pat.Abilities,
			&pat.ExpiresAt,
		)
		if err != nil {
			log.Printf("[TOKEN] query by id error: %v", err)
		} else {
			log.Printf("[TOKEN] DB row: id=%d dbToken=%q userID=%d abilities=%q expiresAt=%v",
				pat.ID, pat.TokenHash, pat.UserID, pat.Abilities, pat.ExpiresAt)

			if pat.TokenHash == hashStr || pat.TokenHash == tokenPart {
				log.Printf("[TOKEN] token match (hash or plain) for id=%d", pat.ID)
				return &pat, nil
			}
			log.Printf("[TOKEN] token mismatch: dbToken=%q, hashStr=%q, plain=%q",
				pat.TokenHash, hashStr, tokenPart)
		}
	}

	query := `
		SELECT id, token, tokenable_id, abilities, expires_at
		FROM personal_access_tokens
		WHERE tokenable_type = $1
		  AND token IN ($2, $3)
		  AND (expires_at IS NULL OR expires_at > $4)
		ORDER BY created_at DESC
		LIMIT 1
	`

	log.Printf("[TOKEN] fallback query by token IN (hash, plain)")

	err := r.db.QueryRowContext(ctx, query, userTokenableType, hashStr, tokenPart, time.Now()).Scan(
		&pat.ID,
		&pat.TokenHash,
		&pat.UserID,
		&pat.Abilities,
		&pat.ExpiresAt,
	)
	if err != nil {
		log.Printf("[TOKEN] fallback query error: %v", err)
		return nil, errors.New("token not found")
	}

	log.Printf("[TOKEN] fallback row: id=%d dbToken=%q userID=%d abilities=%q expiresAt=%v",
		pat.ID, pat.TokenHash, pat.UserID, pat.Abilities, pat.ExpiresAt)

	return &pat, nil
}
