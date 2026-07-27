package ratelimit

import "net/http"

// ConcurrencyLimitMiddleware caps how many requests this instance handles at
// once, rejecting any beyond the cap immediately with 503 rather than
// queuing them — a queued request still holds a goroutine and connection
// open while it waits, which defeats the point of a concurrency cap. This is
// intentionally per-process/in-memory rather than Redis-backed: unlike rate
// limiting (a shared request budget across the fleet), the resource being
// protected here — this instance's goroutines/memory/file descriptors — is
// local to the instance.
//
// It exists alongside IPRateLimitMiddleware and the login-failure delay to
// close a gap those leave open: they bound requests per minute per caller,
// but not how many requests can be in flight at the same instant. A caller
// holding many concurrent connections open (amplified further by a
// handler that intentionally delays its response, as LoginHandler does on
// failure) can exhaust server resources before any per-caller rate limit
// would ever kick in.
func ConcurrencyLimitMiddleware(maxConcurrent int) func(http.Handler) http.Handler {
	sem := make(chan struct{}, maxConcurrent)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Retry-After", "1")
				writeError(w, r, http.StatusServiceUnavailable, "too_many_concurrent_requests", "server is busy, try again shortly")
			}
		})
	}
}
