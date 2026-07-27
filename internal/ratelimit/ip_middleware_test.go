package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeKeyLimiter struct {
	decision Decision
	err      error
	calls    []string
}

func (f *fakeKeyLimiter) AllowKey(ctx context.Context, key string, window Window, limit int) (Decision, error) {
	f.calls = append(f.calls, key)
	if f.err != nil {
		return Decision{}, f.err
	}
	return f.decision, nil
}

func nextCalled() (http.Handler, *bool) {
	called := false
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), &called
}

func TestIPRateLimitMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("allows and sets headers when within limit", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeKeyLimiter{decision: Decision{Allowed: true, Limit: 20, Remaining: 19}}
		next, called := nextCalled()

		handler := IPRateLimitMiddleware(limiter, 20)(next)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !*called {
			t.Fatal("expected next handler to be called")
		}
		if got := w.Header().Get("X-RateLimit-Remaining"); got != "19" {
			t.Errorf("X-RateLimit-Remaining = %q, want 19", got)
		}
		if len(limiter.calls) != 1 || limiter.calls[0] != "ip:203.0.113.7" {
			t.Errorf("calls = %v, want key ip:203.0.113.7", limiter.calls)
		}
	})

	t.Run("prefers X-Forwarded-For over RemoteAddr", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeKeyLimiter{decision: Decision{Allowed: true, Limit: 20, Remaining: 19}}
		next, _ := nextCalled()

		handler := IPRateLimitMiddleware(limiter, 20)(next)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		req.Header.Set("X-Forwarded-For", "198.51.100.9, 203.0.113.7")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if len(limiter.calls) != 1 || limiter.calls[0] != "ip:198.51.100.9" {
			t.Errorf("calls = %v, want key ip:198.51.100.9", limiter.calls)
		}
	})

	t.Run("denies with 429 and Retry-After when limit exceeded", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeKeyLimiter{decision: Decision{Allowed: false, Limit: 20, Remaining: 0, RetryAfter: 15 * time.Second}}
		next, called := nextCalled()

		handler := IPRateLimitMiddleware(limiter, 20)(next)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if *called {
			t.Fatal("expected next handler NOT to be called")
		}
		if w.Result().StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", w.Result().StatusCode)
		}
		if got := w.Header().Get("Retry-After"); got != "15" {
			t.Errorf("Retry-After = %q, want 15", got)
		}
	})

	t.Run("fails open when redis is unavailable", func(t *testing.T) {
		t.Parallel()

		limiter := &fakeKeyLimiter{err: errors.New("connection refused")}
		next, called := nextCalled()

		handler := IPRateLimitMiddleware(limiter, 20)(next)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !*called {
			t.Fatal("expected next handler to be called (fail open)")
		}
	})
}
