package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)


func TestCorrelationIDMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("generates an id when none is supplied", func(t *testing.T) {
		t.Parallel()
		var gotCtxID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCtxID, _ = CorrelationIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		rec := httptest.NewRecorder()
		CorrelationIDMiddleware(next).ServeHTTP(rec, req)

		header := rec.Header().Get(CorrelationHeader)
		if header == "" {
			t.Fatal("expected response header to carry a correlation ID")
		}
		if _, err := uuid.Parse(header); err != nil {
			t.Errorf("generated correlation ID %q is not a valid UUID: %v", header, err)
		}
		if gotCtxID != header {
			t.Errorf("context correlation ID = %q, want it to match response header %q", gotCtxID, header)
		}
	})

	t.Run("propagates a well-formed client-supplied id", func(t *testing.T) {
		t.Parallel()
		clientID := uuid.New().String()
		var gotCtxID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCtxID, _ = CorrelationIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		req.Header.Set(CorrelationHeader, clientID)
		rec := httptest.NewRecorder()
		CorrelationIDMiddleware(next).ServeHTTP(rec, req)

		if got := rec.Header().Get(CorrelationHeader); got != clientID {
			t.Errorf("response header = %q, want client-supplied %q", got, clientID)
		}
		if gotCtxID != clientID {
			t.Errorf("context correlation ID = %q, want %q", gotCtxID, clientID)
		}
	})

	t.Run("replaces a malformed client-supplied id", func(t *testing.T) {
		t.Parallel()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		req.Header.Set(CorrelationHeader, "not-a-uuid; drop table logs;")
		rec := httptest.NewRecorder()
		CorrelationIDMiddleware(next).ServeHTTP(rec, req)

		header := rec.Header().Get(CorrelationHeader)
		if _, err := uuid.Parse(header); err != nil {
			t.Errorf("expected a generated valid UUID to replace the malformed id, got %q: %v", header, err)
		}
	})
}
