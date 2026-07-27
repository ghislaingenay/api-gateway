package ratelimit

import (
	"net/http"
	"strconv"

	"api-gateway/internal/clientip"
	"api-gateway/internal/logger"
)

// IPRateLimitMiddleware enforces a per-minute request limit keyed by client
// IP, for endpoints reached before any authenticated identity exists (e.g.
// /auth/login, /auth/refresh). Like RateLimitMiddleware, it fails open on
// Redis errors and logs the failure.
func IPRateLimitMiddleware(limiter KeyLimiter, perMinute int, trustedProxyHops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "ip:" + clientip.FromRequest(r, trustedProxyHops)

			decision, err := limiter.AllowKey(r.Context(), key, WindowMinute, perMinute)
			if err != nil {
				logger.FromContext(r.Context()).Warn("ratelimit: redis unavailable, failing open",
					"event_type", "rate_limit_fail_open",
					"key", key,
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
