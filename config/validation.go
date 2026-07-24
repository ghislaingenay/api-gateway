package config

// DefaultMaxBodyBytes is the fallback request body size limit enforced by
// the validation middleware before JSON parsing
const DefaultMaxBodyBytes = 1 << 20 // 1 MiB = 1 * 1024 * 1024

// ValidationConfig holds the request validation middleware's settings.
type ValidationConfig struct {
	MaxBodyBytes int64
}

// LoadValidationConfig reads the validation middleware's max body size from
// the environment.
//
// VALIDATION_MAX_BODY_BYTES defaults to DefaultMaxBodyBytes when unset or
// not a positive integer.
func LoadValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MaxBodyBytes: int64(positiveIntEnv("VALIDATION_MAX_BODY_BYTES", DefaultMaxBodyBytes)),
	}
}
