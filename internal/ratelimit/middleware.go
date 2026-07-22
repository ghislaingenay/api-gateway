package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"api-gateway/internal/auth"
	"api-gateway/internal/logger"
	"api-gateway/internal/tenant"

	"github.com/google/uuid"
)

// LimitsProvider resolves a tenant's configured per-minute/per-hour limits.
// Declared here (the consumer) per the DI convention; *tenant.redisStatusCache
// satisfies it structurally.
type LimitsProvider interface {
	RateLimits(ctx context.Context, tenantID uuid.UUID) (tenant.RateLimits, error)
}

// Defaults are the environment-configured fallback limits applied when a
// tenant's configured limit is missing or non-positive (FEAT-005 Edge Cases).
type Defaults struct {
	PerMinute int
	PerHour   int
}

// RateLimitMiddleware enforces per-tenant, per-user, per-minute and per-hour
// request limits and sets the standard rate-limit response headers. It must run
// after auth.JWTAuthMiddleware, since it reads tenant/user identity from
// validated claims rather than parsing the token itself. On any failure to
// reach Redis or resolve tenant limits it fails open (allows the request)
// and logs the failure, per FEAT-005 FR-3.
func RateLimitMiddleware(limiter Limiter, limits LimitsProvider, defaults Defaults) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing authenticated identity")
				return
			}

			tenantLimits, err := limits.RateLimits(r.Context(), claims.TenantID)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: failed to resolve tenant limits, failing open",
					"event_type", "rate_limit_fail_open",
					"tenant_id", claims.TenantID.String(),
					"reason", err.Error(),
				)
				next.ServeHTTP(w, r)
				return
			}

			perMinute := resolveLimit(tenantLimits.PerMinute, defaults.PerMinute)
			perHour := resolveLimit(tenantLimits.PerHour, defaults.PerHour)

			minuteDecision, err := limiter.Allow(r.Context(), claims.TenantID, claims.UserID, WindowMinute, perMinute)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: redis unavailable, failing open",
					"event_type", "rate_limit_fail_open",
					"tenant_id", claims.TenantID.String(),
					"reason", err.Error(),
				)
				next.ServeHTTP(w, r)
				return
			}

			hourDecision, err := limiter.Allow(r.Context(), claims.TenantID, claims.UserID, WindowHour, perHour)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: redis unavailable, failing open",
					"event_type", "rate_limit_fail_open",
					"tenant_id", claims.TenantID.String(),
					"reason", err.Error(),
				)
				next.ServeHTTP(w, r)
				return
			}

			binding := minuteDecision
			if hourDecision.Remaining < binding.Remaining {
				binding = hourDecision
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(binding.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(binding.Remaining))

			if !minuteDecision.Allowed || !hourDecision.Allowed {
				retryAfter := minuteDecision.RetryAfter
				if hourDecision.RetryAfter > retryAfter {
					retryAfter = hourDecision.RetryAfter
				}
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				writeError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func resolveLimit(configured, fallback int) int {
	if configured <= 0 {
		return fallback
	}
	return configured
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	}); err != nil {
		logger.FromContext(r.Context()).Error("ratelimit: failed to write error response", "error", err.Error())
	}
}
