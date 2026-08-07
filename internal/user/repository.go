package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

// Repository loads and provisions user records from persistent storage.
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByKeycloakSub(ctx context.Context, sub string) (*User, error)
	// EnsureByKeycloakSub returns the existing users row for sub, or creates
	// one if this is the first time sub has been seen (JIT provisioning,
	// FEAT-012 FR-3). No tenant membership is granted by this call.
	EnsureByKeycloakSub(ctx context.Context, sub, email string) (*User, error)
	// ListTenantAccess returns every tenant userID belongs to, joined with
	// tenant name and role name, for GET /auth/me without X-Tenant-ID.
	ListTenantAccess(ctx context.Context, userID uuid.UUID) ([]TenantAccess, error)
}

type postgresRepository struct {
	db *sql.DB
}

// NewRepository returns a Repository backed by PostgreSQL.
func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const selectUserColumns = `
	id, keycloak_sub, email, created_at, updated_at
`

// GetByID implements Repository.
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectUserColumns+`
		FROM users
		WHERE id = $1
	`, id)
	return scanUser(row)
}

// GetByKeycloakSub implements Repository.
func (r *postgresRepository) GetByKeycloakSub(ctx context.Context, sub string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectUserColumns+`
		FROM users
		WHERE keycloak_sub = $1
	`, sub)
	return scanUser(row)
}

// EnsureByKeycloakSub implements Repository.
func (r *postgresRepository) EnsureByKeycloakSub(ctx context.Context, sub, email string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO users (keycloak_sub, email)
		VALUES ($1, $2)
		ON CONFLICT (keycloak_sub) DO UPDATE SET email = EXCLUDED.email
		RETURNING `+selectUserColumns+`
	`, sub, email)
	return scanUser(row)
}

// ListTenantAccess implements Repository.
func (r *postgresRepository) ListTenantAccess(ctx context.Context, userID uuid.UUID) ([]TenantAccess, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, r.name
		FROM tenant_users tu
		JOIN tenants t ON t.id = tu.tenant_id
		JOIN roles r ON r.id = tu.role_id
		WHERE tu.user_id = $1
		ORDER BY t.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query tenant access: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.FromContext(ctx).Error("user: failed to close tenant access rows", "error", cerr.Error())
		}
	}()

	var access []TenantAccess
	for rows.Next() {
		var a TenantAccess
		if err := rows.Scan(&a.TenantID, &a.TenantName, &a.RoleName); err != nil {
			return nil, fmt.Errorf("scan tenant access: %w", err)
		}
		access = append(access, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant access: %w", err)
	}
	return access, nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.KeycloakSub, &u.Email, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}
