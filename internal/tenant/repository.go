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
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)
}

type postgresRepository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by PostgreSQL.
func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const selectTenantColumns = `
	id, name, slug, tier, rate_limit_per_minute, rate_limit_per_hour,
	max_users, features, is_active, created_at, updated_at, deleted_at
`

// GetByID implements Repository.
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+selectTenantColumns+` FROM tenants WHERE id = $1`, id)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, id)
		}
		return nil, err
	}
	return t, nil
}

// GetBySlug implements Repository.
func (r *postgresRepository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+selectTenantColumns+` FROM tenants WHERE slug = $1`, slug)
	t, err := scanTenant(row)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrTenantNotFound, slug)
		}
		return nil, err
	}
	return t, nil
}

func scanTenant(row *sql.Row) (*Tenant, error) {
	var (
		t            Tenant
		featuresJSON []byte
	)

	err := row.Scan(
		&t.ID, &t.Name, &t.Slug, &t.Tier, &t.RateLimitPerMinute, &t.RateLimitPerHour,
		&t.MaxUsers, &featuresJSON, &t.IsActive, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTenantNotFound
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
