package logger

import (
	"net/http"

	"github.com/google/uuid"
)

// CorrelationHeader is the header carrying a request's correlation ID, both
// as a client-supplied input and on every response.
const CorrelationHeader = "X-Correlation-ID"

// CorrelationIDMiddleware assigns a correlation ID to every request: it
// propagates a well-formed client-supplied X-Correlation-ID, or generates a
// new one otherwise (FEAT-009 FR-1). A malformed client-supplied ID is
// replaced rather than trusted (FEAT-009 Edge Cases). It must run first in
// the middleware chain so every other component can log through
// FromContext with the correlation ID already attached.
func CorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(CorrelationHeader)
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.New().String()
		}

		w.Header().Set(CorrelationHeader, id)
		next.ServeHTTP(w, r.WithContext(WithCorrelationID(r.Context(), id)))
	})
}
