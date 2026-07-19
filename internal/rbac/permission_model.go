package rbac

import (
	"time"

	"github.com/google/uuid"
)

// Permission represents a single grantable action on a resource, in
// "resource:action" format (e.g. "users:create").
type Permission struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name" validate:"required"`
	Resource    string    `json:"resource" db:"resource" validate:"required"`
	Action      string    `json:"action" db:"action" validate:"required"`
	Description string    `json:"description" db:"description" validate:"required"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// TableName returns the database table name for Permission.
func (Permission) TableName() string { return "permissions" }
