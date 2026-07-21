package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Repository loads and updates user records from persistent storage.
type Repository interface {
	GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateLastLoginAt(ctx context.Context, id uuid.UUID, at time.Time) error
}

type postgresRepository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by PostgreSQL.
func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const selectUserColumns = `
	id, tenant_id, role_id, email, password_hash, is_active, email_verified,
	last_login_at, created_at, updated_at, deleted_at
`

// GetByEmail implements Repository.
func (r *postgresRepository) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectUserColumns+`
		FROM users
		WHERE tenant_id = $1 AND email = $2 AND deleted_at IS NULL
	`, tenantID, email)
	return scanUser(row)
}

// GetByID implements Repository.
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectUserColumns+`
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return scanUser(row)
}

// UpdateLastLoginAt implements Repository.
func (r *postgresRepository) UpdateLastLoginAt(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET last_login_at = $2, updated_at = NOW() WHERE id = $1
	`, id, at)
	if err != nil {
		return fmt.Errorf("update last_login_at: %w", err)
	}
	return nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.TenantID, &u.RoleID, &u.Email, &u.PasswordHash, &u.IsActive, &u.EmailVerified,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}
