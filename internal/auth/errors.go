package auth

import "errors"

var (
	ErrMissingToken           = errors.New("missing authorization token")
	ErrMalformedToken         = errors.New("malformed authorization header")
	ErrUnknownKey             = errors.New("unknown signing key")
	ErrMissingClaims          = errors.New("missing required claim")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidRefreshToken    = errors.New("invalid refresh token")
	ErrGeneratingRefreshToken = errors.New("error generating refresh token")
)
