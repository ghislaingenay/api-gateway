package profile

import (
	"time"

	"github.com/google/uuid"
)

// Profile represents additional profile information for a user.
type Profile struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	UserID    uuid.UUID              `json:"user_id" db:"user_id" validate:"required"`
	FirstName *string                `json:"first_name,omitempty" db:"first_name" validate:"omitempty,min=1,max=100"`
	LastName  *string                `json:"last_name,omitempty" db:"last_name" validate:"omitempty,min=1,max=100"`
	AvatarURL *string                `json:"avatar_url,omitempty" db:"avatar_url" validate:"omitempty,url"`
	Timezone  string                 `json:"timezone" db:"timezone" validate:"required,timezone"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// TableName returns the database table name for Profile.
func (Profile) TableName() string { return "profiles" }
