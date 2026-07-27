package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestConcurrencyLimitMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("allows requests up to the cap", func(t *testing.T) {
		t.Parallel()

		next, called := nextCalled()
		handler := ConcurrencyLimitMiddleware(2)(next)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", nil))

		if !*called {
			t.Fatal("expected next handler to be called")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Result().StatusCode)
		}
	})

	t.Run("rejects with 503 immediately once the cap is reached, without blocking", func(t *testing.T) {
		t.Parallel()

		release := make(chan struct{})
		inFlight := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release // hold the slot open until the test says otherwise
			w.WriteHeader(http.StatusOK)
		})
		handler := ConcurrencyLimitMiddleware(1)(inFlight)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
		}()

		// Give the first request time to acquire the only slot.
		time.Sleep(20 * time.Millisecond)

		done := make(chan int, 1)
		go func() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
			done <- w.Result().StatusCode
		}()

		select {
		case status := <-done:
			if status != http.StatusServiceUnavailable {
				t.Errorf("second request status = %d, want 503", status)
			}
		case <-time.After(time.Second):
			t.Fatal("second request blocked instead of returning 503 immediately")
		}

		close(release)
		wg.Wait()
	})

	t.Run("frees the slot after the handler returns", func(t *testing.T) {
		t.Parallel()

		next, _ := nextCalled()
		handler := ConcurrencyLimitMiddleware(1)(next)

		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
			if w.Result().StatusCode != http.StatusOK {
				t.Fatalf("request %d: status = %d, want 200 (slot should be free)", i, w.Result().StatusCode)
			}
		}
	})
}
