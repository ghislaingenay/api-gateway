package user

import (
	"time"

	"github.com/google/uuid"
)

// User represents one person, identified by their Keycloak subject.
// Independent of any tenant — tenant membership and role are resolved
// separately via tenant_users (see internal/identity), not stored here.
type User struct {
	ID          uuid.UUID `json:"id" db:"id"`
	KeycloakSub string    `json:"-" db:"keycloak_sub" validate:"required"`
	Email       string    `json:"email" db:"email" validate:"required,email"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name for User.
func (User) TableName() string { return "users" }

// TenantAccess is one of a user's tenant_users rows, joined with the
// tenant's name and the assigned role's name, for listing the tenants a
// caller belongs to (GET /auth/me without X-Tenant-ID).
type TenantAccess struct {
	TenantID   uuid.UUID
	TenantName string
	RoleName   string
}
