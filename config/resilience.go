package config

import "time"

// Default gateway-wide deadline and retry policy applied when a route has
// no per-route override (FEAT-008 Open Questions).
const (
	DefaultDeadlineSeconds    = 5
	DefaultRetryMaxAttempts   = 3
	DefaultRetryBaseBackoffMS = 100
)

// ResilienceConfig holds the default request deadline and retry policy
// applied when a route has no per-route override.
type ResilienceConfig struct {
	DefaultDeadline    time.Duration
	DefaultMaxAttempts int
	DefaultBaseBackoff time.Duration
}

// LoadResilienceConfig reads the resilience defaults from the environment.
//
// GATEWAY_DEFAULT_DEADLINE_SECONDS, GATEWAY_RETRY_MAX_ATTEMPTS, and
// GATEWAY_RETRY_BASE_BACKOFF_MS default to DefaultDeadlineSeconds,
// DefaultRetryMaxAttempts, and DefaultRetryBaseBackoffMS respectively when
// unset or not a positive integer.
func LoadResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		DefaultDeadline:    time.Duration(positiveIntEnv("GATEWAY_DEFAULT_DEADLINE_SECONDS", DefaultDeadlineSeconds)) * time.Second,
		DefaultMaxAttempts: positiveIntEnv("GATEWAY_RETRY_MAX_ATTEMPTS", DefaultRetryMaxAttempts),
		DefaultBaseBackoff: time.Duration(positiveIntEnv("GATEWAY_RETRY_BASE_BACKOFF_MS", DefaultRetryBaseBackoffMS)) * time.Millisecond,
	}
}
