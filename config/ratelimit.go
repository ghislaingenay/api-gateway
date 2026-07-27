package config

import (
	"os"
	"strconv"
)

// Default per-tenant request limits
const (
	DefaultRateLimitPerMinute = 60
	DefaultRateLimitPerHour   = 1000
	// DefaultLoginRateLimitPerMinute caps unauthenticated attempts on
	// /auth/login and /auth/refresh per client IP, per minute.
	DefaultLoginRateLimitPerMinute = 20
)

// RateLimitConfig holds the default per-tenant rate limits applied when a
// tenant's configured limit is missing or invalid (see FEAT-005 Edge Cases),
// plus the IP-keyed limit applied to pre-authentication endpoints.
type RateLimitConfig struct {
	DefaultPerMinute int
	DefaultPerHour   int
	LoginPerMinute   int
}

// LoadRateLimitConfig reads rate limit defaults from the environment.
func LoadRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		DefaultPerMinute: positiveIntEnv("RATE_LIMIT_PER_MINUTE_DEFAULT", DefaultRateLimitPerMinute),
		DefaultPerHour:   positiveIntEnv("RATE_LIMIT_PER_HOUR_DEFAULT", DefaultRateLimitPerHour),
		LoginPerMinute:   positiveIntEnv("RATE_LIMIT_LOGIN_PER_MINUTE_DEFAULT", DefaultLoginRateLimitPerMinute),
	}
}

func positiveIntEnv(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
