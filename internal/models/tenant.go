package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a multi-tenant organization.
type Tenant struct {
	ID                 uuid.UUID              `json:"id" db:"id"`
	Name               string                 `json:"name" db:"name" validate:"required,min=2,max=255"`
	Slug               string                 `json:"slug" db:"slug" validate:"required,min=2,max=100,slug"`
	Tier               string                 `json:"tier" db:"tier" validate:"required,oneof=free professional enterprise"`
	RateLimitPerMinute int                    `json:"rate_limit_per_minute" db:"rate_limit_per_minute" validate:"required,min=1"`
	RateLimitPerHour   int                    `json:"rate_limit_per_hour" db:"rate_limit_per_hour" validate:"required,min=1"`
	MaxUsers           int                    `json:"max_users" db:"max_users" validate:"required,min=1"`
	Features           map[string]interface{} `json:"features" db:"features"`
	IsActive           bool                   `json:"is_active" db:"is_active"`
	CreatedAt          time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at" db:"updated_at"`
	DeletedAt          *time.Time             `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name for Tenant.
func (Tenant) TableName() string { return "tenants" }
