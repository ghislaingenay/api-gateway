package testfixtures

import (
	"api-gateway/internal/tenant"

	"github.com/google/uuid"
)

func NewValidTenant() tenant.Tenant {
	return tenant.Tenant{
		ID:                 uuid.New(),
		Name:               "Acme Inc",
		Slug:               "acme-inc",
		Tier:               "free",
		RateLimitPerMinute: 60,
		RateLimitPerHour:   1000,
		MaxUsers:           10,
	}
}