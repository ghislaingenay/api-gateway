package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestReverseProxier_Proxy_DeadlineExceeded(t *testing.T) {
	t.Parallel()

	// Upstream sleeps past the request's deadline so the round trip fails
	// with context.DeadlineExceeded (FEAT-008 FR-2).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	proxy := NewReverseProxier()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	proxy.Proxy(rec, req, upstream.URL)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}
