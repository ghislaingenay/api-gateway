package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"api-gateway/internal/user"

	"github.com/google/uuid"
)

// TenantUser is a caller's verified role assignment within a tenant, as
// stored in tenant_users.
type TenantUser struct {
	UserID uuid.UUID
	RoleID uuid.UUID
}

// Resolver JIT-provisions local users rows for previously-unseen Keycloak
// subjects and resolves a caller's verified tenant membership.
type Resolver interface {
	// EnsureUser returns the local user id for keycloakSub, creating a new
	// users row (with no tenant membership) the first time this sub is seen,
	// and reusing the existing row on every subsequent call.
	EnsureUser(ctx context.Context, keycloakSub, email string) (uuid.UUID, error)
	// ResolveTenantUser returns the caller's tenant_users role assignment
	// for tenantID, or ErrNotMember if userID has no such row.
	ResolveTenantUser(ctx context.Context, userID, tenantID uuid.UUID) (*TenantUser, error)
}

type postgresResolver struct {
	db    *sql.DB
	users user.Repository
}

// NewResolver returns a Resolver backed by PostgreSQL.
func NewResolver(db *sql.DB, users user.Repository) Resolver {
	return &postgresResolver{db: db, users: users}
}

// EnsureUser implements Resolver.
func (r *postgresResolver) EnsureUser(ctx context.Context, keycloakSub, email string) (uuid.UUID, error) {
	u, err := r.users.EnsureByKeycloakSub(ctx, keycloakSub, email)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure user for keycloak sub: %w", err)
	}
	return u.ID, nil
}

// ResolveTenantUser implements Resolver.
func (r *postgresResolver) ResolveTenantUser(ctx context.Context, userID, tenantID uuid.UUID) (*TenantUser, error) {
	var roleID uuid.UUID
	err := r.db.QueryRowContext(ctx, `
		SELECT tu.role_id
		FROM tenant_users tu
		JOIN roles r ON r.id = tu.role_id
		WHERE tu.tenant_id = $1 AND tu.user_id = $2
	`, tenantID, userID).Scan(&roleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotMember
		}
		return nil, fmt.Errorf("query tenant membership: %w", err)
	}
	return &TenantUser{UserID: userID, RoleID: roleID}, nil
}
