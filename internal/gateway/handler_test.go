package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/identity"

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

	t.Run("missing identity returns 401", func(t *testing.T) {
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
			t.Error("proxy should not be called without identity")
		}
	})

	t.Run("identity with no tenant returns 401", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{active: true}, proxy)

		ident := &identity.ResolvedIdentity{UserID: uuid.New()}
		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		req = req.WithContext(identity.WithIdentity(req.Context(), ident))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
		if proxy.called {
			t.Error("proxy should not be called without a resolved tenant")
		}
	})

	t.Run("status check error returns 500", func(t *testing.T) {
		t.Parallel()
		proxy := &fakeProxier{}
		handler := NewHandler(routes, fakeStatusChecker{err: errors.New("redis down")}, proxy)

		req := withIdentity(httptest.NewRequest(http.MethodGet, "/api/orders/1", nil), tenantID)
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

		req := withIdentity(httptest.NewRequest(http.MethodGet, "/api/orders/1", nil), tenantID)
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

		req := withIdentity(httptest.NewRequest(http.MethodGet, "/unknown", nil), tenantID)
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

		req := withIdentity(httptest.NewRequest(http.MethodGet, "/api/orders/1", nil), tenantID)
		req.Header.Set("X-Tenant-ID", tenantID.String())  // legitimate client input, left alone
		req.Header.Set(TenantHeader, uuid.New().String()) // client-supplied, must be overwritten
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
			t.Errorf("forwarded tenant header = %q, want %q (resolved identity tenant_id)", proxy.header, tenantID.String())
		}
	})
}

func withIdentity(r *http.Request, tenantID uuid.UUID) *http.Request {
	ident := &identity.ResolvedIdentity{UserID: uuid.New(), TenantID: &tenantID}
	return r.WithContext(identity.WithIdentity(r.Context(), ident))
}
