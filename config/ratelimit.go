package config

import (
	"os"
	"strconv"
)

// Default per-tenant request limits used when RATE_LIMIT_PER_MINUTE_DEFAULT /
// RATE_LIMIT_PER_HOUR_DEFAULT are unset or invalid.
const (
	DefaultRateLimitPerMinute = 60
	DefaultRateLimitPerHour   = 1000
)

// RateLimitConfig holds the default per-tenant rate limits applied when a
// tenant's configured limit is missing or invalid (see FEAT-005 Edge Cases).
type RateLimitConfig struct {
	DefaultPerMinute int
	DefaultPerHour   int
}

// LoadRateLimitConfig reads rate limit defaults from the environment.
//
// RATE_LIMIT_PER_MINUTE_DEFAULT and RATE_LIMIT_PER_HOUR_DEFAULT default to
// DefaultRateLimitPerMinute/DefaultRateLimitPerHour when unset or not a
// positive integer.
func LoadRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		DefaultPerMinute: positiveIntEnv("RATE_LIMIT_PER_MINUTE_DEFAULT", DefaultRateLimitPerMinute),
		DefaultPerHour:   positiveIntEnv("RATE_LIMIT_PER_HOUR_DEFAULT", DefaultRateLimitPerHour),
	}
}

func positiveIntEnv(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
