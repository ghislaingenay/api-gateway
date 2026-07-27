package config

import "time"

// Defaults for the progressive login-failure delay (brute-force slowdown)
// and the concurrency cap on in-flight login/refresh attempts.
const (
	DefaultLoginFailureWindowSeconds = 900  // 15 minutes
	DefaultLoginFailureDelayBaseMS   = 250
	DefaultLoginFailureDelayMaxMS    = 4000
	// DefaultLoginMaxConcurrent bounds how many /auth/login and
	// /auth/refresh requests this instance handles at once, so the
	// intentional per-failure delay can't be used to exhaust goroutines/
	// connections via many attackers held open at the same time.
	DefaultLoginMaxConcurrent = 50
)

// LoginSecurityConfig holds the failure-window and backoff parameters used
// to progressively delay responses to repeated failed logins, plus the
// concurrency cap applied alongside it.
type LoginSecurityConfig struct {
	FailureWindow    time.Duration
	DelayBaseBackoff time.Duration
	DelayMax         time.Duration
	MaxConcurrent    int
}

// LoadLoginSecurityConfig reads the login-failure-delay defaults from the
// environment.
func LoadLoginSecurityConfig() *LoginSecurityConfig {
	return &LoginSecurityConfig{
		FailureWindow:    time.Duration(positiveIntEnv("LOGIN_FAILURE_WINDOW_SECONDS", DefaultLoginFailureWindowSeconds)) * time.Second,
		DelayBaseBackoff: time.Duration(positiveIntEnv("LOGIN_FAILURE_DELAY_BASE_MS", DefaultLoginFailureDelayBaseMS)) * time.Millisecond,
		DelayMax:         time.Duration(positiveIntEnv("LOGIN_FAILURE_DELAY_MAX_MS", DefaultLoginFailureDelayMaxMS)) * time.Millisecond,
		MaxConcurrent:    positiveIntEnv("LOGIN_MAX_CONCURRENT_ATTEMPTS", DefaultLoginMaxConcurrent),
	}
}
