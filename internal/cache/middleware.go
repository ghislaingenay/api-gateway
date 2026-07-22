package cache

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/gateway"
	"api-gateway/internal/logger"
)

// maxCacheableBodyBytes bounds how large a downstream response body may be
// before it is skipped for caching (FEAT-006 Edge Cases: large response
// bodies), so a single oversized response can't bloat Redis.
const maxCacheableBodyBytes = 1 << 20 // 1 MiB

// RouteResolver resolves the static route for a method+path, giving access
// to its per-route CacheTTL override. Declared here (the consumer);
// *gateway.RouteTable satisfies it structurally.
type RouteResolver interface {
	Resolve(method, path string) (*gateway.Route, bool)
}

// CacheMiddleware serves cached responses for GET requests from Redis, and
// stores successful downstream responses for future hits. It must run after
// auth.JWTAuthMiddleware (reads tenant identity from validated claims) and
// after any rate-limit middleware, so a cache hit still counts against the
// tenant's rate limit rather than bypassing it. Non-GET requests bypass the
// cache entirely (FEAT-006 FR-3). On a Redis error it fails open to the
// downstream call, consistent with FEAT-005's fail-open philosophy. It
// re-checks tenant status on a cache hit — gateway.NewHandler is the only
// other place that check runs, and a hit never reaches it — so a
// deactivated tenant is blocked from a cache hit exactly as it would be
// from a live downstream call.
func CacheMiddleware(store ResponseCache, routes RouteResolver, tenantStatus gateway.TenantStatusChecker, defaultTTL time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing authenticated identity")
				return
			}

			route, ok := routes.Resolve(r.Method, r.URL.Path)
			if !ok {
				// No matching route: let the downstream handler produce the
				// 404, rather than duplicating route-resolution logic here.
				next.ServeHTTP(w, r)
				return
			}
			// Share the resolved route with gateway.NewHandler via context
			// so it doesn't re-run RouteTable.Resolve for the same request.
			r = r.WithContext(gateway.WithRoute(r.Context(), route))

			ttl := defaultTTL
			if route.CacheTTL > 0 {
				ttl = route.CacheTTL
			}

			queryHash := NormalizeQueryHash(r.URL.RawQuery)
			key := BuildKey(claims.TenantID, r.Method, r.URL.Path, queryHash)

			cached, hit, err := store.Get(r.Context(), key)
			if err != nil {
				logger.FromContext(r.Context()).Warn("cache: redis unavailable, failing open",
					"key", key,
					"error", err.Error(),
				)
			} else if hit {
				active, err := tenantStatus.IsActive(r.Context(), claims.TenantID)
				if err != nil {
					logger.FromContext(r.Context()).Warn("cache: tenant status check failed, failing open to downstream",
						"tenant_id", claims.TenantID.String(),
						"error", err.Error(),
					)
				} else if !active {
					writeError(w, r, http.StatusForbidden, "tenant_inactive", "tenant is not active")
					return
				} else {
					logger.FromContext(r.Context()).Info("cache hit",
						"event_type", "cache_hit",
						"tenant_id", claims.TenantID.String(),
						"path", r.URL.Path,
					)
					writeCached(w, r, cached)
					return
				}
			}

			logger.FromContext(r.Context()).Info("cache miss",
				"event_type", "cache_miss",
				"tenant_id", claims.TenantID.String(),
				"path", r.URL.Path,
			)
			w.Header().Set("X-Cache", "MISS")
			rec := newResponseRecorder(w)
			next.ServeHTTP(rec, r)

			if rec.cacheable && !rec.bodyTooLarge {
				resp := &CachedResponse{
					StatusCode: rec.statusCode,
					Header:     rec.snapshotHeader,
					Body:       rec.body.Bytes(),
				}
				if err := store.Set(r.Context(), key, resp, ttl); err != nil {
					logger.FromContext(r.Context()).Error("cache: failed to store response", "key", key, "error", err.Error())
				}
			}
		})
	}
}

// writeCached replays a cached response verbatim, including its stored
// headers, and marks it with X-Cache: HIT for observability.
func writeCached(w http.ResponseWriter, r *http.Request, cached *CachedResponse) {
	dst := w.Header()
	for k, v := range cached.Header {
		dst[k] = v
	}
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(cached.StatusCode)
	if _, err := w.Write(cached.Body); err != nil {
		logger.FromContext(r.Context()).Error("cache: failed to write cached response", "error", err.Error())
	}
}

// responseRecorder wraps a real http.ResponseWriter to tee a downstream
// response into an in-memory buffer for potential caching, while streaming
// every byte through to the client immediately (unlike a fully-buffering
// recorder, this preserves incremental delivery for chunked/SSE responses
// and never delays time-to-first-byte). It implements http.Flusher so
// httputil.ReverseProxy's streaming flush still works through it.
//
// Cacheability is decided once, at WriteHeader time: only 2xx responses
// with no non-identity Content-Encoding are buffered (buffering a
// non-cacheable response would waste memory for nothing, and replaying a
// stored Content-Encoding without also varying the cache key by
// Accept-Encoding can hand one client an undecodable body cached for
// another). Once buffered bytes exceed maxCacheableBodyBytes, buffering
// stops and the response is treated as too large to cache — the client
// keeps receiving bytes unaffected.
type responseRecorder struct {
	underlying     http.ResponseWriter
	statusCode     int
	wroteHeader    bool
	cacheable      bool
	bodyTooLarge   bool
	body           bytes.Buffer
	snapshotHeader http.Header
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{underlying: w, statusCode: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header {
	return r.underlying.Header()
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.statusCode = status

	contentEncoding := r.underlying.Header().Get("Content-Encoding")
	r.cacheable = status >= 200 && status < 300 &&
		(contentEncoding == "" || strings.EqualFold(contentEncoding, "identity"))
	if r.cacheable {
		r.snapshotHeader = r.underlying.Header().Clone()
	}

	r.underlying.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.cacheable && !r.bodyTooLarge {
		if r.body.Len()+len(b) > maxCacheableBodyBytes {
			r.bodyTooLarge = true
			r.body.Reset()
		} else {
			r.body.Write(b)
		}
	}
	return r.underlying.Write(b)
}

// Flush implements http.Flusher, delegating to the underlying writer so
// httputil.ReverseProxy can stream chunked/SSE responses through this
// recorder instead of buffering the whole thing before any bytes are sent.
func (r *responseRecorder) Flush() {
	if f, ok := r.underlying.(http.Flusher); ok {
		f.Flush()
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	}); err != nil {
		logger.FromContext(r.Context()).Error("cache: failed to write error response", "error", err.Error())
	}
}
