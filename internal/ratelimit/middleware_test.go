package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/tenant"

	"github.com/google/uuid"
)

type fakeLimiter struct {
	decisions map[Window]Decision
	err       error
	calls     []Window
}

func (f *fakeLimiter) Allow(ctx context.Context, tenantID, userID uuid.UUID, window Window, limit int) (Decision, error) {
	f.calls = append(f.calls, window)
	if f.err != nil {
		return Decision{}, f.err
	}
	return f.decisions[window], nil
}

type fakeLimitsProvider struct {
	limits tenant.RateLimits
	err    error
}

func (f *fakeLimitsProvider) RateLimits(ctx context.Context, tenantID uuid.UUID) (tenant.RateLimits, error) {
	return f.limits, f.err
}

func newTestRequest(t *testing.T) *http.Request {
	t.Helper()
	claims := &auth.CustomClaims{TenantID: uuid.New(), UserID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	return req.WithContext(auth.WithClaims(req.Context(), claims))
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Parallel()

	nextCalled := func() (http.Handler, *bool) {
		called := false
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}), &called
	}

	t.Run("allows and sets headers when within both windows", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{decisions: map[Window]Decision{
			WindowMinute: {Allowed: true, Limit: 60, Remaining: 59},
			WindowHour:   {Allowed: true, Limit: 1000, Remaining: 999},
		}}
		limits := &fakeLimitsProvider{limits: tenant.RateLimits{PerMinute: 60, PerHour: 1000}}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(t))

		if !*called {
			t.Fatal("expected next handler to be called")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
		if got := w.Header().Get("X-RateLimit-Limit"); got != "60" {
			t.Errorf("X-RateLimit-Limit = %q, want 60", got)
		}
		if got := w.Header().Get("X-RateLimit-Remaining"); got != "59" {
			t.Errorf("X-RateLimit-Remaining = %q, want 59", got)
		}
	})

	t.Run("denies with 429 and Retry-After when minute window exceeded", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{decisions: map[Window]Decision{
			WindowMinute: {Allowed: false, Limit: 60, Remaining: 0, RetryAfter: 30 * time.Second},
			WindowHour:   {Allowed: true, Limit: 1000, Remaining: 500},
		}}
		limits := &fakeLimitsProvider{limits: tenant.RateLimits{PerMinute: 60, PerHour: 1000}}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(t))

		if *called {
			t.Fatal("expected next handler NOT to be called")
		}
		if w.Result().StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", w.Result().StatusCode)
		}
		if got := w.Header().Get("Retry-After"); got != "30" {
			t.Errorf("Retry-After = %q, want 30", got)
		}
	})

	t.Run("fails open when redis is unavailable", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{err: errors.New("connection refused")}
		limits := &fakeLimitsProvider{limits: tenant.RateLimits{PerMinute: 60, PerHour: 1000}}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(t))

		if !*called {
			t.Fatal("expected next handler to be called (fail open)")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
	})

	t.Run("fails open when tenant limits cannot be resolved", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{decisions: map[Window]Decision{}}
		limits := &fakeLimitsProvider{err: errors.New("db unavailable")}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(t))

		if !*called {
			t.Fatal("expected next handler to be called (fail open)")
		}
	})

	t.Run("falls back to defaults when tenant limits are non-positive", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{decisions: map[Window]Decision{
			WindowMinute: {Allowed: true, Limit: 60, Remaining: 10},
			WindowHour:   {Allowed: true, Limit: 1000, Remaining: 10},
		}}
		limits := &fakeLimitsProvider{limits: tenant.RateLimits{PerMinute: 0, PerHour: -1}}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newTestRequest(t))

		if !*called {
			t.Fatal("expected next handler to be called")
		}
	})

	t.Run("returns 401 when claims are missing", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{}
		limits := &fakeLimitsProvider{}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
		handler.ServeHTTP(w, req)

		if *called {
			t.Fatal("expected next handler NOT to be called")
		}
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Result().StatusCode)
		}
	})
}

func TestResolveLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured int
		fallback   int
		want       int
	}{
		{"positive configured value wins", 30, 60, 30},
		{"zero falls back", 0, 60, 60},
		{"negative falls back", -5, 60, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveLimit(tt.configured, tt.fallback); got != tt.want {
				t.Errorf("resolveLimit(%d, %d) = %d, want %d", tt.configured, tt.fallback, got, tt.want)
			}
		})
	}
}
