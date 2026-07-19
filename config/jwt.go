package config

import (
	"os"
	"strings"
)

// JWTConfig holds the JWT validation settings used by the auth middleware.
type JWTConfig struct {
	// AllowedAlgorithms is the explicit signing-algorithm allowlist (e.g. RS256).
	AllowedAlgorithms []string
	// SigningKeys maps a key ID (kid) to a base64-encoded PEM-encoded RSA
	// public key, enabling multiple simultaneously active keys for rotation.
	SigningKeys map[string]string
}

// LoadJWTConfig reads JWT settings from the environment.
//
// JWT_ALLOWED_ALGORITHMS is a comma-separated list (defaults to "RS256").
// JWT_SIGNING_KEYS is a comma-separated list of "kid=base64pem" pairs.
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

	keys := make(map[string]string)
	jwtSigningKeys := os.Getenv("JWT_SIGNING_KEYS")
	if strings.TrimSpace(jwtSigningKeys) == "" {
		return &JWTConfig{
			AllowedAlgorithms: allowed,
			SigningKeys:       keys,
		}
	}
	
	for _, pair := range strings.Split(os.Getenv("JWT_SIGNING_KEYS"), ",") {
		kid, key, found := strings.Cut(pair, "=")
		kid, key = strings.TrimSpace(kid), strings.TrimSpace(key)
		if !found || kid == "" || key == "" {
			continue
		}
		keys[kid] = key
	}

	return &JWTConfig{
		AllowedAlgorithms: allowed,
		SigningKeys:       keys,
	}
}
