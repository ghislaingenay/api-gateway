package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/auth"

	"github.com/google/uuid"
)

type fakeStatusChecker struct {
	active bool
	err    error
}

func (f fakeStatusChecker) IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	return f.active, f.err
}

type fakeProxier struct {
	called   bool
	upstream string
	header   string
}

func (f *fakeProxier) Proxy(w http.ResponseWriter, r *http.Request, upstream string) {
	f.called = true
	f.upstream = upstream
	f.header = r.Header.Get(TenantHeader)
	w.WriteHeader(http.StatusOK)
}

func TestNewHandler(t *testing.T) {
	routes := NewRouteTable([]Route{
		{Path: "/api/orders/*", Method: "GET", Upstream: "http://orders-service"},
	})
	tenantID := uuid.New()

	t.Run("missing claims returns 401", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{active: true}, proxy)

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
		if proxy.called {
			t.Error("proxy should not be called without claims")
		}
	})

	t.Run("status check error returns 500", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{err: errors.New("redis down")}, proxy)

		req := withClaims(httptest.NewRequest(http.MethodGet, "/api/orders/1", nil), tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("inactive tenant returns 403", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{active: false}, proxy)

		req := withClaims(httptest.NewRequest(http.MethodGet, "/api/orders/1", nil), tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if proxy.called {
			t.Error("proxy should not be called for an inactive tenant")
		}
	})

	t.Run("unmatched path returns 404", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{active: true}, proxy)

		req := withClaims(httptest.NewRequest(http.MethodGet, "/unknown", nil), tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("active tenant and matched route proxies with trusted header", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{active: true}, proxy)

		req := withClaims(httptest.NewRequest(http.MethodGet, "/api/orders/1", nil), tenantID)
		req.Header.Set("X-Tenant-ID", uuid.New().String()) // spoofed, must be stripped
		req.Header.Set(TenantHeader, uuid.New().String())  // client-supplied, must be overwritten
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !proxy.called {
			t.Fatal("expected proxy to be called")
		}
		if proxy.upstream != "http://orders-service" {
			t.Errorf("upstream = %q, want %q", proxy.upstream, "http://orders-service")
		}
		if proxy.header != tenantID.String() {
			t.Errorf("forwarded tenant header = %q, want %q (claims tenant_id)", proxy.header, tenantID.String())
		}
		if req.Header.Get("X-Tenant-ID") != "" {
			t.Error("spoofed X-Tenant-ID header was not stripped")
		}
	})
}

func withClaims(r *http.Request, tenantID uuid.UUID) *http.Request {
	claims := &auth.CustomClaims{TenantID: tenantID, UserID: uuid.New()}
	return r.WithContext(auth.WithClaims(r.Context(), claims))
}
