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
	// Issuer is the expected `iss` claim on every token, validated against
	// Keycloak's realm issuer URL. Required — no fallback, fails startup if
	// unset (FEAT-012 FR-1), matching JWKSURL's existing fail-fast posture.
	Issuer string
}

// LoadJWTConfig reads JWT settings from the environment.
//
// JWT_ALLOWED_ALGORITHMS is a comma-separated list (defaults to "RS256").
// JWT_JWKS_URL is the JWKS endpoint used to fetch/verify signing keys.
// JWT_JWKS_REFRESH_INTERVAL configures the background refresh cadence
// (parsed with time.ParseDuration; defaults to 1h when unset or invalid).
// JWT_ISSUER is the expected `iss` claim (Keycloak's realm issuer URL).
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

	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))

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
		Issuer:              issuer,
	}
}
