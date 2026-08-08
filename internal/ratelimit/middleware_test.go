package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/identity"
	"api-gateway/internal/tenant"

	"github.com/google/uuid"
)

type fakeLimiter struct {
	decisions map[Window]Decision
	err       error
	calls     int
}

func (f *fakeLimiter) AllowBoth(ctx context.Context, tenantID, userID uuid.UUID, minuteLimit, hourLimit int) (Decision, Decision, error) {
	f.calls++
	if f.err != nil {
		return Decision{}, Decision{}, f.err
	}
	return f.decisions[WindowMinute], f.decisions[WindowHour], nil
}

type fakeSingleWindowLimiter struct {
	decision Decision
	err      error
	calls    int
	gotKey   string
}

func (f *fakeSingleWindowLimiter) Allow(ctx context.Context, key string, window Window, limit int) (Decision, error) {
	f.calls++
	f.gotKey = key
	if f.err != nil {
		return Decision{}, f.err
	}
	return f.decision, nil
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
	tenantID := uuid.New()
	ident := &identity.ResolvedIdentity{UserID: uuid.New(), TenantID: &tenantID}
	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	return req.WithContext(identity.WithIdentity(req.Context(), ident))
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

	t.Run("returns 401 when identity is missing", func(t *testing.T) {
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

	t.Run("keys under uuid.Nil when identity has no tenant context", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeLimiter{decisions: map[Window]Decision{
			WindowMinute: {Allowed: true, Limit: 60, Remaining: 59},
			WindowHour:   {Allowed: true, Limit: 1000, Remaining: 999},
		}}
		limits := &fakeLimitsProvider{limits: tenant.RateLimits{PerMinute: 60, PerHour: 1000}}
		next, called := nextCalled()

		handler := RateLimitMiddleware(limiter, limits, Defaults{PerMinute: 60, PerHour: 1000})(next)
		w := httptest.NewRecorder()

		ident := &identity.ResolvedIdentity{UserID: uuid.New()}
		req := httptest.NewRequest(http.MethodGet, "/roles", nil)
		req = req.WithContext(identity.WithIdentity(req.Context(), ident))
		handler.ServeHTTP(w, req)

		if !*called {
			t.Fatal("expected next handler to be called")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
	})
}

func TestOnboardingRateLimitMiddleware(t *testing.T) {
	t.Parallel()

	nextCalled := func() (http.Handler, *bool) {
		called := false
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}), &called
	}

	newOnboardingRequest := func(userID uuid.UUID) *http.Request {
		ident := &identity.ResolvedIdentity{UserID: userID}
		req := httptest.NewRequest(http.MethodPost, "/onboarding", nil)
		return req.WithContext(identity.WithIdentity(req.Context(), ident))
	}

	t.Run("allows and sets headers when within the window", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeSingleWindowLimiter{decision: Decision{Allowed: true, Limit: 1, Remaining: 0}}
		next, called := nextCalled()

		handler := OnboardingRateLimitMiddleware(limiter, WindowDay, 1)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newOnboardingRequest(uuid.New()))

		if !*called {
			t.Fatal("expected next handler to be called")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
		if got := w.Header().Get("X-RateLimit-Limit"); got != "1" {
			t.Errorf("X-RateLimit-Limit = %q, want 1", got)
		}
	})

	t.Run("denies with 429 and Retry-After when the daily limit is exceeded", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeSingleWindowLimiter{decision: Decision{Allowed: false, Limit: 1, Remaining: 0, RetryAfter: time.Hour}}
		next, called := nextCalled()

		handler := OnboardingRateLimitMiddleware(limiter, WindowDay, 1)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newOnboardingRequest(uuid.New()))

		if *called {
			t.Fatal("expected next handler NOT to be called")
		}
		if w.Result().StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", w.Result().StatusCode)
		}
		if got := w.Header().Get("Retry-After"); got != "3600" {
			t.Errorf("Retry-After = %q, want 3600", got)
		}
	})

	t.Run("fails open when the limiter backend is unavailable", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeSingleWindowLimiter{err: errors.New("connection refused")}
		next, called := nextCalled()

		handler := OnboardingRateLimitMiddleware(limiter, WindowDay, 1)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newOnboardingRequest(uuid.New()))

		if !*called {
			t.Fatal("expected next handler to be called (fail open)")
		}
	})

	t.Run("returns 401 when identity is missing", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeSingleWindowLimiter{}
		next, called := nextCalled()

		handler := OnboardingRateLimitMiddleware(limiter, WindowDay, 1)(next)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/onboarding", nil)
		handler.ServeHTTP(w, req)

		if *called {
			t.Fatal("expected next handler NOT to be called")
		}
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Result().StatusCode)
		}
	})

	t.Run("keys by user id, not tenant", func(t *testing.T) {
		t.Parallel()

		userID := uuid.New()
		limiter := &fakeSingleWindowLimiter{decision: Decision{Allowed: true, Limit: 1, Remaining: 0}}
		next, _ := nextCalled()

		handler := OnboardingRateLimitMiddleware(limiter, WindowDay, 1)(next)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, newOnboardingRequest(userID))

		want := "onboarding:" + userID.String()
		if limiter.gotKey != want {
			t.Errorf("limiter key = %q, want %q", limiter.gotKey, want)
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
