package cache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/gateway"

	"github.com/google/uuid"
)

type fakeResponseCache struct {
	getResp   *CachedResponse
	getHit    bool
	getErr    error
	setErr    error
	setCalls  []string
	getCalled bool
}

func (f *fakeResponseCache) Get(ctx context.Context, key string) (*CachedResponse, bool, error) {
	f.getCalled = true
	return f.getResp, f.getHit, f.getErr
}

func (f *fakeResponseCache) Set(ctx context.Context, key string, resp *CachedResponse, ttl time.Duration) error {
	f.setCalls = append(f.setCalls, key)
	return f.setErr
}

type staticRouteResolver struct {
	route *gateway.Route
	ok    bool
}

func (s *staticRouteResolver) Resolve(method, path string) (*gateway.Route, bool) {
	return s.route, s.ok
}

type fakeTenantStatusChecker struct {
	active bool
	err    error
	calls  int
}

func (f *fakeTenantStatusChecker) IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	f.calls++
	return f.active, f.err
}

func newTestRequest(method, target string) *http.Request {
	claims := &auth.CustomClaims{TenantID: uuid.New(), UserID: uuid.New()}
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(auth.WithClaims(req.Context(), claims))
}

func TestCacheMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("non-GET requests bypass the cache entirely", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusCreated)
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodPost, "/api/orders"))

		if !nextCalled {
			t.Fatal("expected next handler to be called")
		}
		if store.getCalled || len(store.setCalls) != 0 {
			t.Fatal("expected cache to never be consulted for non-GET requests")
		}
	})

	t.Run("cache hit returns stored response without calling next", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{
			getHit: true,
			getResp: &CachedResponse{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       []byte(`{"cached":true}`),
			},
		}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if nextCalled {
			t.Fatal("expected next handler NOT to be called on cache hit")
		}
		if w.Header().Get("X-Cache") != "HIT" {
			t.Fatalf("X-Cache = %q, want HIT", w.Header().Get("X-Cache"))
		}
		if w.Body.String() != `{"cached":true}` {
			t.Fatalf("body = %q, want cached body", w.Body.String())
		}
		if tenantStatus.calls != 1 {
			t.Fatalf("expected tenant status to be checked once on a hit, got %d calls", tenantStatus.calls)
		}
	})

	t.Run("cache hit for a deactivated tenant returns 403 instead of serving the cache", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{
			getHit:  true,
			getResp: &CachedResponse{StatusCode: http.StatusOK, Header: http.Header{}, Body: []byte("stale")},
		}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: false}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if nextCalled {
			t.Fatal("expected next handler NOT to be called for a deactivated tenant")
		}
		if w.Result().StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Result().StatusCode)
		}
		if w.Body.String() == "stale" {
			t.Fatal("expected the stale cached body NOT to be served to a deactivated tenant")
		}
	})

	t.Run("cache hit fails open to downstream when the tenant status check errors", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{
			getHit:  true,
			getResp: &CachedResponse{StatusCode: http.StatusOK, Header: http.Header{}, Body: []byte("cached")},
		}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{err: errors.New("redis unavailable")}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if !nextCalled {
			t.Fatal("expected next handler to be called (fail open)")
		}
	})

	t.Run("cache miss forwards to downstream and stores a 2xx response", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if len(store.setCalls) != 1 {
			t.Fatalf("expected 1 set call, got %d", len(store.setCalls))
		}
		if w.Header().Get("X-Cache") != "MISS" {
			t.Fatalf("X-Cache = %q, want MISS", w.Header().Get("X-Cache"))
		}
		if w.Body.String() != `{"ok":true}` {
			t.Fatalf("body = %q, want forwarded body", w.Body.String())
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
	})

	t.Run("4xx/5xx downstream responses are not cached", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if len(store.setCalls) != 0 {
			t.Fatalf("expected no set calls for a 404, got %d", len(store.setCalls))
		}
		if w.Result().StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Result().StatusCode)
		}
	})

	t.Run("responses with a non-identity Content-Encoding are not cached", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Encoding", "gzip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("compressed-bytes"))
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if len(store.setCalls) != 0 {
			t.Fatalf("expected no set calls for a gzip-encoded response, got %d", len(store.setCalls))
		}
		if w.Body.String() != "compressed-bytes" {
			t.Fatalf("body = %q, want forwarded body regardless of caching decision", w.Body.String())
		}
	})

	t.Run("streaming responses are flushed through immediately, not buffered until completion", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		var sawFlush bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("chunk-1"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
				sawFlush = true
			}
			_, _ = w.Write([]byte("chunk-2"))
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if !sawFlush {
			t.Fatal("expected the response recorder to implement http.Flusher so downstream code can flush mid-stream")
		}
		if w.Body.String() != "chunk-1chunk-2" {
			t.Fatalf("body = %q, want both chunks forwarded", w.Body.String())
		}
	})

	t.Run("fails open to downstream when redis errors on read", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{getErr: errors.New("connection refused")}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if !nextCalled {
			t.Fatal("expected next handler to be called (fail open)")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
	})

	t.Run("no matching route forwards to next without consulting cache", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{ok: false}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNotFound)
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/unknown"))

		if !nextCalled {
			t.Fatal("expected next handler to be called")
		}
		if store.getCalled {
			t.Fatal("expected cache not to be consulted when no route matches")
		}
	})

	t.Run("returns 401 when claims are missing", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		handler.ServeHTTP(w, req)

		if nextCalled {
			t.Fatal("expected next handler NOT to be called")
		}
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Result().StatusCode)
		}
	})

	t.Run("route-specific TTL overrides the default", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		routes := &staticRouteResolver{route: &gateway.Route{CacheTTL: 5 * time.Minute}, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		var capturedTTL time.Duration
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		handler := CacheMiddleware(&ttlCapturingCache{fakeResponseCache: store, captured: &capturedTTL}, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if capturedTTL != 5*time.Minute {
			t.Fatalf("ttl = %v, want route override of 5m", capturedTTL)
		}
	})

	t.Run("shares the resolved route with downstream handlers via context", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseCache{}
		route := &gateway.Route{Path: "/api/orders", Method: http.MethodGet}
		routes := &staticRouteResolver{route: route, ok: true}
		tenantStatus := &fakeTenantStatusChecker{active: true}
		var gotRoute *gateway.Route
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRoute, _ = gateway.RouteFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		handler := CacheMiddleware(store, routes, tenantStatus, time.Minute)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(http.MethodGet, "/api/orders"))

		if gotRoute != route {
			t.Fatalf("expected downstream handler to see the same resolved route via context, got %+v", gotRoute)
		}
	})
}

type ttlCapturingCache struct {
	*fakeResponseCache
	captured *time.Duration
}

func (c *ttlCapturingCache) Set(ctx context.Context, key string, resp *CachedResponse, ttl time.Duration) error {
	*c.captured = ttl
	return c.fakeResponseCache.Set(ctx, key, resp, ttl)
}
