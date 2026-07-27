package config

import (
	"os"
	"strings"
	"time"

	"api-gateway/internal/logger"
)

// defaultJWKSRefreshInterval is used when JWT_JWKS_REFRESH_INTERVAL is unset
// or invalid.
const defaultJWKSRefreshInterval = time.Hour

// JWTConfig holds the JWT validation settings used by the auth middleware.
type JWTConfig struct {
	// AllowedAlgorithms is the explicit signing-algorithm allowlist (e.g. RS256).
	AllowedAlgorithms []string
	// JWKSURL is the endpoint the gateway fetches signing public keys from,
	// keyed by kid. Required — there is no static-key fallback.
	JWKSURL string
	// JWKSRefreshInterval is how often the JWKS key set is refreshed in the
	// background. Defaults to defaultJWKSRefreshInterval when unset/invalid.
	JWKSRefreshInterval time.Duration
	// SigningKID is the kid used to sign newly issued tokens. Its public
	// half must be present in the JWKS document served at JWKSURL.
	SigningKID string
	// SigningPrivateKey is a base64-encoded PEM-encoded RSA private key used
	// to sign newly issued tokens.
	SigningPrivateKey string
}

// LoadJWTConfig reads JWT settings from the environment.
//
// JWT_ALLOWED_ALGORITHMS is a comma-separated list (defaults to "RS256").
// JWT_JWKS_URL is the JWKS endpoint used to fetch/verify signing keys.
// JWT_JWKS_REFRESH_INTERVAL configures the background refresh cadence
// (parsed with time.ParseDuration; defaults to 1h when unset or invalid).
// JWT_SIGNING_KID and JWT_SIGNING_PRIVATE_KEY configure the key used to
// sign newly issued tokens (login/refresh).
func LoadJWTConfig() *JWTConfig {
	algos := os.Getenv("JWT_ALLOWED_ALGORITHMS")
	if algos == "" {
		algos = "RS256"
	}

	allowed := make([]string, 0)
	for _, a := range strings.Split(algos, ",") {
		if a = strings.TrimSpace(a); a != "" {
			allowed = append(allowed, a)
		}
	}

	signingKID := strings.TrimSpace(os.Getenv("JWT_SIGNING_KID"))
	signingPrivateKey := strings.TrimSpace(os.Getenv("JWT_SIGNING_PRIVATE_KEY"))
	if signingKID == "" || signingPrivateKey == "" {
		logger.Default().Warn("JWT_SIGNING_KID or JWT_SIGNING_PRIVATE_KEY is empty, signing keys will not be available for issuing new tokens")
	}

	refreshInterval := defaultJWKSRefreshInterval
	if raw := strings.TrimSpace(os.Getenv("JWT_JWKS_REFRESH_INTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			logger.Default().Warn("JWT_JWKS_REFRESH_INTERVAL is invalid, using default", "value", raw, "default", defaultJWKSRefreshInterval)
		} else {
			refreshInterval = parsed
		}
	}

	return &JWTConfig{
		AllowedAlgorithms:   allowed,
		JWKSURL:             strings.TrimSpace(os.Getenv("JWT_JWKS_URL")),
		JWKSRefreshInterval: refreshInterval,
		SigningKID:          signingKID,
		SigningPrivateKey:   signingPrivateKey,
	}
}
