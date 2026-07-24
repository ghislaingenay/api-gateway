package config

import "time"

// Default gateway-wide timeout and retry policy applied
const (
	// maximum amount of time allowed for a request to complete before the gateway gives up and cancels it
	DefaultTimeoutSeconds    = 5
	DefaultRetryMaxAttempts   = 3
	DefaultRetryBaseBackoffMS = 100
)

type ResilienceConfig struct {
	DefaultTimeout    time.Duration
	DefaultMaxAttempts int
	DefaultBaseBackoff time.Duration
}

// LoadResilienceConfig reads the resilience defaults from the environment.
func LoadResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		DefaultTimeout:    time.Duration(positiveIntEnv("GATEWAY_DEFAULT_TIMEOUT_SECONDS", DefaultTimeoutSeconds)) * time.Second,
		DefaultMaxAttempts: positiveIntEnv("GATEWAY_RETRY_MAX_ATTEMPTS", DefaultRetryMaxAttempts),
		DefaultBaseBackoff: time.Duration(positiveIntEnv("GATEWAY_RETRY_BASE_BACKOFF_MS", DefaultRetryBaseBackoffMS)) * time.Millisecond,
	}
}
