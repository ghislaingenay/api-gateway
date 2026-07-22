package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	HealthHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
}

func TestReadyHandler(t *testing.T) {
	t.Parallel()

	t.Run("200 when both dependencies are healthy", func(t *testing.T) {
		t.Parallel()
		checker := NewDependencyChecker(fakeRedisPinger{}, fakePostgresPinger{})

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		ReadyHandler(checker).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var resp ReadyResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "ready" || resp.Redis != "ok" || resp.Postgres != "ok" {
			t.Errorf("resp = %+v, want ready/ok/ok", resp)
		}
	})

	t.Run("503 when a dependency is unreachable", func(t *testing.T) {
		t.Parallel()
		checker := NewDependencyChecker(fakeRedisPinger{err: errors.New("down")}, fakePostgresPinger{})

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		ReadyHandler(checker).ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
		var resp ReadyResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "not_ready" || resp.Redis != "unreachable" {
			t.Errorf("resp = %+v, want not_ready/unreachable", resp)
		}
	})
}
