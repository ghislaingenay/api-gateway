package auth

import "errors"

var (
	// ErrMissingToken means the Authorization header was absent or empty.
	ErrMissingToken = errors.New("missing authorization token")
	// ErrMalformedToken means the Authorization header was not a well-formed "Bearer <token>" value.
	ErrMalformedToken = errors.New("malformed authorization header")
	// ErrUnknownKey means the token's kid did not match any active signing key.
	ErrUnknownKey = errors.New("unknown signing key")
	// ErrMissingClaims means a required custom claim (tenant_id, user_id) was absent or zero-valued.
	ErrMissingClaims = errors.New("missing required claim")
)
