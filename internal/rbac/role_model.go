package rbac

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a system role with associated permissions.
type Role struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Name         string    `json:"name" db:"name" validate:"required,oneof=admin manager viewer"`
	DisplayName  string    `json:"display_name" db:"display_name" validate:"required,min=2,max=100"`
	Description  string    `json:"description" db:"description" validate:"required"`
	Permissions  []string  `json:"permissions" db:"permissions"`
	IsSystemRole bool      `json:"is_system_role" db:"is_system_role"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name for Role.
func (Role) TableName() string { return "roles" }
