package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"api-gateway/internal/identity"
	"api-gateway/internal/logger"
	"api-gateway/internal/tenant"

	"github.com/google/uuid"
)

// LimitsProvider resolves a tenant's configured per-minute/per-hour limits.
// Declared here (the consumer) per the DI convention; *tenant.memoryStatusCache
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
// request limits and sets the standard rate-limit response headers. It must
// run after identity.ResolveMiddleware, since it reads tenant/user identity
// from the resolved identity rather than parsing the token itself. Routes
// with no tenant context (e.g. GET /roles) are keyed under uuid.Nil rather
// than rejected — RateLimitMiddleware is also wired into non-tenant-scoped
// chains, unlike identity.RequireTenant. On any failure to reach Redis or
// resolve tenant limits it fails open (allows the request) and logs the
// failure, per FEAT-005 FR-3.
func RateLimitMiddleware(limiter MultiWindowLimiter, limits LimitsProvider, defaults Defaults) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ident, ok := identity.IdentityFromContext(r.Context())
			if !ok || ident == nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing authenticated identity")
				return
			}
			tenantID := uuid.Nil
			if ident.TenantID != nil {
				tenantID = *ident.TenantID
			}

			tenantLimits, err := limits.RateLimits(r.Context(), tenantID)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: failed to resolve tenant limits, failing open",
					"event_type", "rate_limit_fail_open",
					"tenant_id", tenantID.String(),
					"reason", err.Error(),
				)
				next.ServeHTTP(w, r)
				return
			}

			perMinute := resolveLimit(tenantLimits.PerMinute, defaults.PerMinute)
			perHour := resolveLimit(tenantLimits.PerHour, defaults.PerHour)

			minuteDecision, hourDecision, err := limiter.AllowBoth(r.Context(), tenantID, ident.UserID, perMinute, perHour)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: redis unavailable, failing open",
					"event_type", "rate_limit_fail_open",
					"tenant_id", tenantID.String(),
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

// OnboardingRateLimitMiddleware enforces a strict per-user limit (e.g. one
// per day) on POST /onboarding attempts, keyed by caller identity alone
// rather than tenant+user — there's no tenant yet at this point. It's
// deliberately separate from RateLimitMiddleware's generic per-tenant/user
// API limit: tenant creation is a privileged, expensive operation that
// warrants a much stricter bound than ordinary API traffic. Like
// RateLimitMiddleware, it must run after identity.ResolveMiddleware and
// fails open (logging the failure) if the limiter backend is unreachable.
func OnboardingRateLimitMiddleware(limiter SingleWindowLimiter, window Window, limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ident, ok := identity.IdentityFromContext(r.Context())
			if !ok || ident == nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing authenticated identity")
				return
			}

			decision, err := limiter.Allow(r.Context(), "onboarding:"+ident.UserID.String(), window, limit)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: failed to check onboarding limit, failing open",
					"event_type", "rate_limit_fail_open",
					"user_id", ident.UserID.String(),
					"reason", err.Error(),
				)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))

			if !decision.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(decision.RetryAfter.Seconds())))
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
