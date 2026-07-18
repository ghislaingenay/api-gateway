package testfixtures

import (
	"api-gateway/internal/user"

	"github.com/google/uuid"
)

func NewValidUser() user.User {
	return user.User {
		ID:           uuid.New(),
		TenantID:     uuid.New(),
		RoleID:       uuid.New(),
		Email:        "user@example.com",
		PasswordHash: "$2a$12$Vw...", // standard mock hash
	}
}