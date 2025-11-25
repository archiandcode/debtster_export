package domain

import "time"

type PersonalAccessToken struct {
	ID        int64
	TokenHash string
	UserID    int64
	Abilities string
	ExpiresAt *time.Time
}
