package config

import (
	"os"
	"strings"
	"time"
)

// DefaultIdentityCacheTTL is used when IDENTITY_CACHE_TTL is unset or
// invalid. Matches a typical Keycloak access-token lifetime (TD-012 §8).
const DefaultIdentityCacheTTL = 15 * time.Minute

// IdentityConfig holds settings for tenant-membership resolution
// (internal/identity), introduced by FEAT-012.
type IdentityConfig struct {
	// CacheTTL bounds how long a resolved tenant_users membership (positive
	// or negative) may be served before the next request re-verifies it
	// against PostgreSQL.
	CacheTTL time.Duration
}

// LoadIdentityConfig reads identity settings from the environment.
//
// IDENTITY_CACHE_TTL is parsed with time.ParseDuration; defaults to
// DefaultIdentityCacheTTL when unset or invalid.
func LoadIdentityConfig() *IdentityConfig {
	ttl := DefaultIdentityCacheTTL
	if raw := strings.TrimSpace(os.Getenv("IDENTITY_CACHE_TTL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			ttl = parsed
		}
	}
	return &IdentityConfig{CacheTTL: ttl}
}
