package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Repository loads tenant records from persistent storage.
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
}

type postgresRepository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by PostgreSQL.
func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

// GetByID implements Repository.
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	var (
		t            Tenant
		featuresJSON []byte
	)

	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, slug, tier, rate_limit_per_minute, rate_limit_per_hour,
		       max_users, features, is_active, created_at, updated_at, deleted_at
		FROM tenants
		WHERE id = $1
	`, id).Scan(
		&t.ID, &t.Name, &t.Slug, &t.Tier, &t.RateLimitPerMinute, &t.RateLimitPerHour,
		&t.MaxUsers, &featuresJSON, &t.IsActive, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, id)
		}
		return nil, fmt.Errorf("query tenant: %w", err)
	}

	if len(featuresJSON) > 0 {
		if err := json.Unmarshal(featuresJSON, &t.Features); err != nil {
			return nil, fmt.Errorf("unmarshal tenant features: %w", err)
		}
	}

	return &t, nil
}
