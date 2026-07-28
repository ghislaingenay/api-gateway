package config

import "testing"

func TestLoadCORSConfig(t *testing.T) {
	t.Run("unset env yields an empty allowlist", func(t *testing.T) {
		got := LoadCORSConfig()
		if len(got.AllowedOrigins) != 0 {
			t.Errorf("AllowedOrigins = %v, want empty", got.AllowedOrigins)
		}
	})

	t.Run("comma-separated origins are trimmed and normalized", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:5173/ ,https://app.example.com,")
		got := LoadCORSConfig()
		want := []string{"http://localhost:5173", "https://app.example.com"}
		if len(got.AllowedOrigins) != len(want) {
			t.Fatalf("AllowedOrigins = %v, want %v", got.AllowedOrigins, want)
		}
		for i, o := range want {
			if got.AllowedOrigins[i] != o {
				t.Errorf("AllowedOrigins[%d] = %q, want %q", i, got.AllowedOrigins[i], o)
			}
		}
	})
}

func TestCORSConfig_IsAllowedOrigin(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"http://localhost:5173", "https://app.example.com"}}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"exact match", "http://localhost:5173", true},
		{"trailing slash is stripped before comparison", "http://localhost:5173/", true},
		{"case-insensitive match", "HTTP://LOCALHOST:5173", true},
		{"not in allowlist", "http://evil.test", false},
		{"empty origin", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.IsAllowedOrigin(tt.origin); got != tt.want {
				t.Errorf("IsAllowedOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}

	t.Run("empty allowlist allows nothing", func(t *testing.T) {
		empty := &CORSConfig{}
		if empty.IsAllowedOrigin("http://localhost:5173") {
			t.Error("empty allowlist should not allow any origin")
		}
	})
}
