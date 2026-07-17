package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user account scoped to a tenant.
type User struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id" validate:"required"`
	RoleID        uuid.UUID  `json:"role_id" db:"role_id" validate:"required"`
	Email         string     `json:"email" db:"email" validate:"required,email"`
	PasswordHash  string     `json:"-" db:"password_hash"`
	IsActive      bool       `json:"is_active" db:"is_active"`
	EmailVerified bool       `json:"email_verified" db:"email_verified"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`

	// Relationships (not stored directly on the users row; populated via JOIN).
	Tenant  *Tenant  `json:"tenant,omitempty" db:"-"`
	Role    *Role    `json:"role,omitempty" db:"-"`
	Profile *Profile `json:"profile,omitempty" db:"-"`
}

// TableName returns the database table name for User.
func (User) TableName() string { return "users" }
