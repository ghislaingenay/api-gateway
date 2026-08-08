package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims are the identity claims the gateway trusts once a JWT's
// signature and standard registered claims (exp, nbf, iat, iss) have been
// validated. Keycloak is the sole token issuer (FEAT-012): the gateway
// never issues tokens itself and no longer trusts tenant/role/permissions
// claims — those are resolved server-side per request by
// identity.ResolveMiddleware instead. Subject (jwt.RegisteredClaims.Subject)
// carries the Keycloak sub, used as the caller's stable identity key.
type CustomClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// Validate implements jwt.ClaimsValidator so the parser rejects tokens
// missing the claims downstream middleware requires to identify the
// caller.
func (c CustomClaims) Validate() error {
	if c.Subject == "" {
		return fmt.Errorf("%w: sub", ErrMissingClaims)
	}
	if c.Email == "" {
		return fmt.Errorf("%w: email", ErrMissingClaims)
	}
	return nil
}
