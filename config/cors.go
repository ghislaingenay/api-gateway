package config

import (
	"os"
	"strings"
)

// CORSConfig holds the allowlist of origins permitted to make credentialed
// (cookie-carrying) cross-origin requests. An empty allowlist preserves the
// pre-existing wildcard, non-credentialed CORS behavior.
type CORSConfig struct {
	AllowedOrigins []string
}

// LoadCORSConfig reads CORS_ALLOWED_ORIGINS (comma-separated) from the
// environment.
func LoadCORSConfig() *CORSConfig {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		return &CORSConfig{}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = normalizeOrigin(p); p != "" {
			origins = append(origins, p)
		}
	}

	return &CORSConfig{AllowedOrigins: origins}
}

// IsAllowedOrigin reports whether origin is in the configured allowlist.
// Comparison is case-insensitive (scheme/host are case-insensitive per RFC
// 6454). origin is also run through normalizeOrigin so a stray trailing
// slash never causes a false negative: browsers never send one on the
// Origin header, but a misconfigured CORS_ALLOWED_ORIGINS entry easily
// could, and a silent mismatch there means every credentialed cross-origin
// request from that origin gets rejected by the browser.
func (c *CORSConfig) IsAllowedOrigin(origin string) bool {
	origin = normalizeOrigin(origin)
	if origin == "" {
		return false
	}
	for _, allowed := range c.AllowedOrigins {
		if strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

// normalizeOrigin strips surrounding whitespace and a trailing slash. The
// slash is never a valid part of an Origin value (it's scheme+host+port
// only), so this is a strict removal, not tolerance of an alternate valid
// form — "http://localhost:5173/" is canonicalized to
// "http://localhost:5173" rather than kept as a distinct accepted value.
func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	return strings.TrimSuffix(origin, "/")
}
