package config

import "time"

// DefaultCacheTTLSeconds is the fallback TTL for cacheable GET routes that
// have no per-route CacheTTLSeconds override (FEAT-006 Open Questions).
const DefaultCacheTTLSeconds = 60

// CacheConfig holds the default response-cache TTL applied when a route has
// no per-route override.
type CacheConfig struct {
	DefaultTTL time.Duration
}

// LoadCacheConfig reads the response-cache default TTL from the environment.
//
// CACHE_DEFAULT_TTL_SECONDS defaults to DefaultCacheTTLSeconds when unset or
// not a positive integer.
func LoadCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL: time.Duration(positiveIntEnv("CACHE_DEFAULT_TTL_SECONDS", DefaultCacheTTLSeconds)) * time.Second,
	}
}
