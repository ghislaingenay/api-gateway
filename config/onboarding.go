package config

// DefaultMaxTenantsPerUser caps how many tenants a single user may hold
// membership in via POST /onboarding. Creating a tenant grants unrestricted
// owner access to it, so unlike the generic API rate limit this bounds
// total tenant creation, not just its rate — the default is "once, ever".
const DefaultMaxTenantsPerUser = 1

// DefaultOnboardingPerDayLimit bounds how many onboarding attempts (not
// just successes) a single user may make per rolling day, via a dedicated
// limiter stricter than the generic per-tenant/user API rate limit —
// tenant creation is a privileged, expensive operation, not a normal API
// call.
const DefaultOnboardingPerDayLimit = 1

// OnboardingConfig holds settings for POST /onboarding's abuse-prevention
// checks.
type OnboardingConfig struct {
	MaxTenantsPerUser int
	PerDayLimit       int
}

// LoadOnboardingConfig reads onboarding settings from the environment.
func LoadOnboardingConfig() *OnboardingConfig {
	return &OnboardingConfig{
		MaxTenantsPerUser: positiveIntEnv("MAX_TENANTS_PER_USER", DefaultMaxTenantsPerUser),
		PerDayLimit:       positiveIntEnv("ONBOARDING_PER_DAY_LIMIT", DefaultOnboardingPerDayLimit),
	}
}
