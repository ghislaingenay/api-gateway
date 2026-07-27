package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// CustomClaims are the identity claims the gateway trusts once a JWT's
// signature and standard registered claims (exp, nbf, iat) have been
// validated. Downstream middleware must read identity only from these
// claims, never from request headers.
type CustomClaims struct {
	jwt.RegisteredClaims
	TenantID    uuid.UUID `json:"tenant_id"`
	UserID      uuid.UUID `json:"user_id"`
	Role        string    `json:"role"`
	RoleID      uuid.UUID `json:"role_id"`
	Permissions []string  `json:"permissions"`
	Email       string    `json:"email"`
}

// Validate implements jwt.ClaimsValidator so the parser rejects tokens
// missing the claims downstream middleware requires to identify the
// request's tenant and user.
func (c CustomClaims) Validate() error {
	if c.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id", ErrMissingClaims)
	}
	if c.UserID == uuid.Nil {
		return fmt.Errorf("%w: user_id", ErrMissingClaims)
	}
	return nil
}
