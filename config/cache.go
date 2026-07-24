package config

import "time"

// DefaultCacheTTLSeconds is the fallback TTL for cacheable GET routes
const DefaultCacheTTLSeconds = 60

// CacheConfig holds the default response-cache TTL applied when a route has
// no per-route override.
type CacheConfig struct {
	DefaultTTL time.Duration
}

func LoadCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL: time.Duration(positiveIntEnv("CACHE_DEFAULT_TTL_SECONDS", DefaultCacheTTLSeconds)) * time.Second,
	}
}
