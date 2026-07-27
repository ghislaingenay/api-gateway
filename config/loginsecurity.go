package config

import "time"

// Defaults for the progressive login-failure delay (brute-force slowdown).
const (
	DefaultLoginFailureWindowSeconds = 900  // 15 minutes
	DefaultLoginFailureDelayBaseMS   = 250
	DefaultLoginFailureDelayMaxMS    = 4000
)

// LoginSecurityConfig holds the failure-window and backoff parameters used
// to progressively delay responses to repeated failed logins.
type LoginSecurityConfig struct {
	FailureWindow    time.Duration
	DelayBaseBackoff time.Duration
	DelayMax         time.Duration
}

// LoadLoginSecurityConfig reads the login-failure-delay defaults from the
// environment.
func LoadLoginSecurityConfig() *LoginSecurityConfig {
	return &LoginSecurityConfig{
		FailureWindow:    time.Duration(positiveIntEnv("LOGIN_FAILURE_WINDOW_SECONDS", DefaultLoginFailureWindowSeconds)) * time.Second,
		DelayBaseBackoff: time.Duration(positiveIntEnv("LOGIN_FAILURE_DELAY_BASE_MS", DefaultLoginFailureDelayBaseMS)) * time.Millisecond,
		DelayMax:         time.Duration(positiveIntEnv("LOGIN_FAILURE_DELAY_MAX_MS", DefaultLoginFailureDelayMaxMS)) * time.Millisecond,
	}
}
