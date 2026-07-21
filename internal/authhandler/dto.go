package authhandler

import (
	"api-gateway/internal/user"
)

// LoginRequest is the body of POST /auth/login. TenantSlug is required
// because users are only unique per-tenant (tenant_id, email).
type LoginRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
	TenantSlug string `json:"tenant_slug" validate:"required"`
}

// LoginResponse is the body returned by a successful login or refresh.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// RefreshResponse has the same shape as LoginResponse.
type RefreshResponse = LoginResponse

// RefreshRequest is the body of POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest is the body of POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// UserResponse is the API representation of a User returned by GET /auth/me.
// PasswordHash is never included.
type UserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
}

func newUserResponse(u user.User, roleName string) UserResponse {
	return UserResponse{
		ID:       u.ID.String(),
		Email:    u.Email,
		TenantID: u.TenantID.String(),
		Role:     roleName,
	}
}
