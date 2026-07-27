package clientip

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFromRequest(t *testing.T) {
	t.Parallel()

	t.Run("trustedHops=0 ignores X-Forwarded-For even if present", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		req.Header.Set("X-Forwarded-For", "6.6.6.6")

		if got := FromRequest(req, 0); got != "203.0.113.7" {
			t.Errorf("FromRequest() = %q, want 203.0.113.7", got)
		}
	})

	t.Run("trustedHops=0 falls back to RemoteAddr without a port", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "not-a-host-port"

		if got := FromRequest(req, 0); got != "not-a-host-port" {
			t.Errorf("FromRequest() = %q, want not-a-host-port", got)
		}
	})

	t.Run("trustedHops=1 trusts the last entry, not the first", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		req.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.9")

		if got := FromRequest(req, 1); got != "198.51.100.9" {
			t.Errorf("FromRequest() = %q, want 198.51.100.9 (last hop)", got)
		}
	})

	t.Run("trustedHops=2 trusts the second-from-last entry", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		req.Header.Set("X-Forwarded-For", "6.6.6.6, 198.51.100.9, 192.0.2.1")

		if got := FromRequest(req, 2); got != "198.51.100.9" {
			t.Errorf("FromRequest() = %q, want 198.51.100.9", got)
		}
	})

	t.Run("trustedHops exceeding the chain length falls back to RemoteAddr", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		req.Header.Set("X-Forwarded-For", "198.51.100.9")

		if got := FromRequest(req, 5); got != "203.0.113.7" {
			t.Errorf("FromRequest() = %q, want 203.0.113.7 (fallback)", got)
		}
	})

	t.Run("truncates an oversized value", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		req.Header.Set("X-Forwarded-For", strings.Repeat("a", 500))

		got := FromRequest(req, 1)
		if len(got) != maxLen {
			t.Errorf("len(FromRequest()) = %d, want %d", len(got), maxLen)
		}
	})
}
