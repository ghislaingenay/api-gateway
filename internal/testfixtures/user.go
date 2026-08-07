package testfixtures

import (
	"api-gateway/internal/user"

	"github.com/google/uuid"
)

func NewValidUser() user.User {
	return user.User{
		ID:          uuid.New(),
		KeycloakSub: uuid.New().String(),
		Email:       "user@example.com",
	}
}
