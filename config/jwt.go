package config

import (
	"api-gateway/internal/logger"
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
	// SigningKID is the kid used to sign newly issued tokens. Its public
	// half must be present in SigningKeys.
	SigningKID string
	// SigningPrivateKey is a base64-encoded PEM-encoded RSA private key used
	// to sign newly issued tokens.
	SigningPrivateKey string
}

// LoadJWTConfig reads JWT settings from the environment.
//
// JWT_ALLOWED_ALGORITHMS is a comma-separated list (defaults to "RS256").
// JWT_SIGNING_KEYS is a comma-separated list of "kid=base64pem" pairs.
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
		return &JWTConfig{
			AllowedAlgorithms: allowed,
			SigningKeys:       nil,
			SigningKID:        signingKID,
			SigningPrivateKey: signingPrivateKey,
		}
	}

	keys := make(map[string]string)
	jwtSigningKeys := os.Getenv("JWT_SIGNING_KEYS")
	if strings.TrimSpace(jwtSigningKeys) == "" {
		logger.Default().Warn("JWT_SIGNING_KEYS is empty, signing keys will not be available for validation")
		return &JWTConfig{
			AllowedAlgorithms: allowed,
			SigningKeys:       keys,
			SigningKID:        signingKID,
			SigningPrivateKey: signingPrivateKey,
		}
	}

	for _, pair := range strings.Split(jwtSigningKeys, ",") {
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
		SigningKID:        signingKID,
		SigningPrivateKey: signingPrivateKey,
	}
}
