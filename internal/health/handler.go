package health

import (
	"encoding/json"
	"net/http"

	"api-gateway/internal/logger"
)

// HealthHandler returns an http.HandlerFunc for GET /health: a liveness
// probe that reports the process is running without checking any
// dependency (FEAT-009 FR-2).
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, HealthResponse{Status: "ok"})
	}
}

// ReadyHandler returns an http.HandlerFunc for GET /ready: a readiness
// probe that reports 200 only if Redis and PostgreSQL are both reachable,
// 503 with per-dependency detail otherwise (FEAT-009 FR-3).
func ReadyHandler(checker *DependencyChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, ready := checker.Check(r.Context())

		resp := ReadyResponse{Status: "ready", Redis: status.Redis, Postgres: status.Postgres}
		code := http.StatusOK
		if !ready {
			resp.Status = "not_ready"
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, r, code, resp)
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.FromContext(r.Context()).Error("health: failed to write response", "error", err.Error())
	}
}
