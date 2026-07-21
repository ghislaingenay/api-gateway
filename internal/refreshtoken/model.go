package refreshtoken

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a single issued refresh token, stored hashed.
type RefreshToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash string     `json:"-" db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// TableName returns the database table name for RefreshToken.
func (RefreshToken) TableName() string { return "refresh_tokens" }

// Valid reports whether the token is neither revoked nor expired as of now.
func (t RefreshToken) Valid(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	return now.Before(t.ExpiresAt)
}
