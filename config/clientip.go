package config

// ClientIPConfig controls how the caller's IP is derived for rate limiting
// and login-failure tracking (see internal/clientip).
type ClientIPConfig struct {
	// TrustedProxyHops is the number of trusted reverse proxies in front of
	// this service (e.g. 1 for a single load balancer). 0 means
	// X-Forwarded-For is ignored and only the raw TCP peer is trusted.
	TrustedProxyHops int
}

// LoadClientIPConfig reads the trusted-proxy-hop count from the environment.
// Since the fallback is 0 (disabled), positiveIntEnv's "non-positive falls
// back" behavior is equivalent to "unset or non-positive means disabled".
func LoadClientIPConfig() *ClientIPConfig {
	return &ClientIPConfig{
		TrustedProxyHops: positiveIntEnv("TRUSTED_PROXY_HOPS", 0),
	}
}
