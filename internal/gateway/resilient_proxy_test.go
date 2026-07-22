package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api-gateway/internal/resilience"
)

// scriptedProxier returns statuses from a fixed script, one per call, and
// records how many times it was invoked and whether the context it was
// called with was ever expired.
type scriptedProxier struct {
	statuses []int
	calls    int
	ctxDone  []bool
}

func (p *scriptedProxier) Proxy(w http.ResponseWriter, r *http.Request, upstream string) {
	p.ctxDone = append(p.ctxDone, r.Context().Err() != nil)
	status := http.StatusOK
	if p.calls < len(p.statuses) {
		status = p.statuses[p.calls]
	}
	p.calls++
	w.WriteHeader(status)
}

func TestResilientProxier_Proxy(t *testing.T) {
	t.Parallel()

	t.Run("GET succeeds on first attempt without retry", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusOK}}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if inner.calls != 1 {
			t.Fatalf("calls = %d, want 1", inner.calls)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("GET retries on transient failure then succeeds", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusBadGateway, http.StatusOK}}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if inner.calls != 2 {
			t.Fatalf("calls = %d, want 2", inner.calls)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("GET stops retrying once max attempts reached and returns last status", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusServiceUnavailable}}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if inner.calls != 3 {
			t.Fatalf("calls = %d, want 3", inner.calls)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("non-retryable status is not retried", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusNotFound}}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if inner.calls != 1 {
			t.Fatalf("calls = %d, want 1", inner.calls)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("POST is never retried even on transient failure", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusServiceUnavailable, http.StatusOK}}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if inner.calls != 1 {
			t.Fatalf("calls = %d, want 1 (non-idempotent methods must not be retried)", inner.calls)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("deadline exceeded during retry wait returns 504", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway}}
		// Backoff is deliberately longer than the deadline so the deadline
		// always fires while waiting between attempt 1 and attempt 2.
		p := NewResilientProxier(inner, 5*time.Millisecond, resilience.RetryPolicy{MaxAttempts: 5, BaseBackoff: time.Second})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if rec.Code != http.StatusGatewayTimeout {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
		}
		if inner.calls != 1 {
			t.Errorf("calls = %d, want 1 (should abort before a second attempt)", inner.calls)
		}
	})

	t.Run("per-route Deadline and RetryMaxAttempts override defaults", func(t *testing.T) {
		t.Parallel()
		inner := &scriptedProxier{statuses: []int{http.StatusBadGateway, http.StatusBadGateway}}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 5, BaseBackoff: time.Millisecond})

		route := &Route{RetryMaxAttempts: 1, Deadline: time.Second}
		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", nil)
		req = req.WithContext(WithRoute(req.Context(), route))
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if inner.calls != 1 {
			t.Fatalf("calls = %d, want 1 (route override caps attempts at 1)", inner.calls)
		}
		if rec.Code != http.StatusBadGateway {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
		}
	})

	t.Run("request body is preserved across retries", func(t *testing.T) {
		t.Parallel()
		inner := &bodyReadingProxier{}
		p := NewResilientProxier(inner, time.Second, resilience.RetryPolicy{MaxAttempts: 2, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodGet, "/api/orders/1", strings.NewReader("hello"))
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if len(inner.bodies) != 2 {
			t.Fatalf("bodies read = %d, want 2", len(inner.bodies))
		}
		for i, b := range inner.bodies {
			if b != "hello" {
				t.Errorf("attempt %d body = %q, want %q", i+1, b, "hello")
			}
		}
	})

	t.Run("non-GET request is not buffered and context deadline still applies", func(t *testing.T) {
		t.Parallel()
		ctxChecker := &ctxAwareProxier{}
		p := NewResilientProxier(ctxChecker, 50*time.Millisecond, resilience.RetryPolicy{MaxAttempts: 1, BaseBackoff: time.Millisecond})

		req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
		rec := httptest.NewRecorder()
		p.Proxy(rec, req, "http://orders-service")

		if ctxChecker.deadline.IsZero() {
			t.Fatal("expected proxy to receive a request with a context deadline set")
		}
	})
}

// bodyReadingProxier reads and records the full request body on each call.
type bodyReadingProxier struct {
	bodies []string
}

func (p *bodyReadingProxier) Proxy(w http.ResponseWriter, r *http.Request, upstream string) {
	b, _ := io.ReadAll(r.Body)
	p.bodies = append(p.bodies, string(b))
	status := http.StatusBadGateway
	if len(p.bodies) >= 2 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
}

// ctxAwareProxier records the deadline present on the request context it
// receives.
type ctxAwareProxier struct {
	deadline time.Time
}

func (p *ctxAwareProxier) Proxy(w http.ResponseWriter, r *http.Request, upstream string) {
	if d, ok := r.Context().Deadline(); ok {
		p.deadline = d
	}
	w.WriteHeader(http.StatusOK)
}
