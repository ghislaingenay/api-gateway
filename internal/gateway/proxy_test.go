package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReverseProxier_Proxy(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	proxy := NewReverseProxier()

	req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
	rec := httptest.NewRecorder()
	proxy.Proxy(rec, req, upstream.URL)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-Upstream-Path"); got != "/api/orders/1" {
		t.Errorf("upstream received path = %q, want %q", got, "/api/orders/1")
	}
}

func TestReverseProxier_Proxy_InvalidUpstream(t *testing.T) {
	t.Parallel()

	proxy := NewReverseProxier()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
	rec := httptest.NewRecorder()

	proxy.Proxy(rec, req, "://not-a-valid-url")

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}
