package testfixtures

import (
	"api-gateway/internal/profile"

	"github.com/google/uuid"
)

func NewValidProfile() profile.Profile {
	return profile.Profile{
		ID:       uuid.New(),
		UserID:   uuid.New(),
		Timezone: "UTC",
	}
}
