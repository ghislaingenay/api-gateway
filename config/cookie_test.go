package config

import (
	"net/http"
	"testing"
)

func TestLoadCookieConfig(t *testing.T) {
	t.Run("development mode defaults to insecure, lax, no domain", func(t *testing.T) {
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvDevelopment})
		if got.Secure {
			t.Error("Secure = true, want false in development mode")
		}
		if got.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite = %v, want Lax", got.SameSite)
		}
		if got.Domain != "" {
			t.Errorf("Domain = %q, want empty", got.Domain)
		}
	})

	t.Run("production mode defaults to secure", func(t *testing.T) {
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvProduction})
		if !got.Secure {
			t.Error("Secure = false, want true in production mode")
		}
	})

	t.Run("COOKIE_SECURE overrides the mode default", func(t *testing.T) {
		t.Setenv("COOKIE_SECURE", "true")
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvDevelopment})
		if !got.Secure {
			t.Error("Secure = false, want true from COOKIE_SECURE override")
		}
	})

	t.Run("COOKIE_SECURE invalid value falls back to the mode default", func(t *testing.T) {
		t.Setenv("COOKIE_SECURE", "not-a-bool")
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvProduction})
		if !got.Secure {
			t.Error("Secure = false, want true (fallback) when COOKIE_SECURE is invalid")
		}
	})

	t.Run("COOKIE_SAMESITE strict", func(t *testing.T) {
		t.Setenv("COOKIE_SAMESITE", "strict")
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvDevelopment})
		if got.SameSite != http.SameSiteStrictMode {
			t.Errorf("SameSite = %v, want Strict", got.SameSite)
		}
	})

	t.Run("COOKIE_SAMESITE none", func(t *testing.T) {
		t.Setenv("COOKIE_SAMESITE", "None")
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvDevelopment})
		if got.SameSite != http.SameSiteNoneMode {
			t.Errorf("SameSite = %v, want None", got.SameSite)
		}
	})

	t.Run("COOKIE_DOMAIN is passed through", func(t *testing.T) {
		t.Setenv("COOKIE_DOMAIN", "example.com")
		got := LoadCookieConfig(&AppConfig{AppEnv: EnvDevelopment})
		if got.Domain != "example.com" {
			t.Errorf("Domain = %q, want example.com", got.Domain)
		}
	})
}
