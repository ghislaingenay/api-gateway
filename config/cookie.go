package config

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

// RefreshTokenCookieName is the name of the httpOnly cookie carrying the raw
// refresh token, and RefreshTokenCookiePath scopes it to /auth/* so it isn't
// sent on every /api/* proxied request.
const (
	RefreshTokenCookieName = "refresh_token"
	RefreshTokenCookiePath = "/auth"
)

// CookieConfig controls the attributes of the refresh token cookie.
type CookieConfig struct {
	Secure   bool
	SameSite http.SameSite
	Domain   string
}

// LoadCookieConfig reads cookie attribute overrides from the environment.
// Secure defaults to true in production and false in development (so local
// dev over plain http://localhost still works), overridable via
// COOKIE_SECURE. SameSite defaults to Lax; Domain defaults to empty
// (host-only cookie).
func LoadCookieConfig(appEnv *AppConfig) *CookieConfig {
	secure := !appEnv.IsDevelopmentMode()
	if raw, ok := os.LookupEnv("COOKIE_SECURE"); ok {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			secure = parsed
		}
	}

	return &CookieConfig{
		Secure:   secure,
		SameSite: sameSiteFromEnv("COOKIE_SAMESITE", http.SameSiteLaxMode),
		Domain:   os.Getenv("COOKIE_DOMAIN"),
	}
}

func sameSiteFromEnv(name string, fallback http.SameSite) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return fallback
	}
}
