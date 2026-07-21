package refreshtoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Repository persists and looks up refresh tokens.
type Repository interface {
	Create(ctx context.Context, t RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
}

type postgresRepository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by PostgreSQL.
func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

// Create implements Repository.
func (r *postgresRepository) Create(ctx context.Context, t RefreshToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, t.ID, t.UserID, t.TokenHash, t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// GetByHash implements Repository.
func (r *postgresRepository) GetByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query refresh token: %w", err)
	}
	return &t, nil
}

// Revoke implements Repository.
func (r *postgresRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1
	`, id, time.Now())
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
