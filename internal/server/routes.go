package server

import (
	"encoding/json"
	"net/http"

	"api-gateway/internal/apidocs"
	"api-gateway/internal/auth"
	"api-gateway/internal/cache"
	"api-gateway/internal/gateway"
	"api-gateway/internal/health"
	"api-gateway/internal/identity"
	"api-gateway/internal/logger"
	"api-gateway/internal/onboarding"
	"api-gateway/internal/ratelimit"
	"api-gateway/internal/rbac"
	"api-gateway/internal/validation"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// /health and /ready are never subject to authentication, rate
	// limiting, or caching middleware (FEAT-009 Business Rules).
	mux.Handle("GET /health", health.HealthHandler())
	mux.Handle("GET /ready", health.ReadyHandler(s.healthChecker))

	// /docs is unauthenticated and excluded from rate limiting, consistent
	// with /health and /ready — it's a documentation surface with no data
	// exposure (FEAT-010 TD-010 §7).
	mux.Handle("GET /docs", apidocs.SwaggerUIHandler())
	mux.Handle("GET /docs/openapi.yaml", apidocs.OpenAPISpecHandler())

	mux.Handle("GET /roles", s.requirePermission("roles:read",
		ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits, s.rateLimitDefs)(rbac.RolesHandler(s.roleCache))))
	mux.Handle("GET /permissions", s.requirePermission("permissions:read",
		ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits, s.rateLimitDefs)(rbac.PermissionsHandler(s.roleCache))))

	// GET /auth/me and POST /onboarding require an authenticated caller but
	// no verified tenant — identity.ResolveMiddleware attaches identity
	// with TenantID=nil when X-Tenant-ID is absent, rather than rejecting
	// (FEAT-012 FR-4/FR-6).
	mux.Handle("GET /auth/me", s.requireIdentity(
		ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits, s.rateLimitDefs)(identity.MeHandler(s.userRepo, s.profileRepo))))
	mux.Handle("POST /onboarding", s.requireIdentity(
		ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits, s.rateLimitDefs)(onboarding.Handler(s.onboardingService))))

	mux.Handle("/api/", auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms, s.jwtIssuer)(
		identity.ResolveMiddleware(s.identityResolver, s.tenantUserCache, s.roleCache)(
			identity.RequireTenant(
				validation.ValidationMiddleware(s.routeTable, s.validationMaxBodyBytes)(
					ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits, s.rateLimitDefs)(
						cache.CacheMiddleware(s.responseCache, s.routeTable, s.tenantStatus, s.cacheDefaultTTL)(
							gateway.NewHandler(s.routeTable, s.tenantStatus, s.proxy),
						),
					),
				),
			),
		),
	))

	// CorrelationIDMiddleware runs first in the chain (FEAT-009 TD-009 §2)
	// so every other middleware can log through logger.FromContext with the
	// correlation ID already attached, and every response — including
	// /health and /ready — carries the X-Correlation-ID header.
	return logger.CorrelationIDMiddleware(s.corsMiddleware(withRouteFallback(mux)))
}

// withRouteFallback wraps mux so that requests matching no registered
// pattern get a JSON 404, and requests matching a pattern's path but not its
// method get a JSON 405 with an Allow header — instead of ServeMux's default
// plain-text bodies. mux.Handler reports which case applies via its returned
// pattern: a non-empty pattern (including for redirects to a canonical path)
// means a real route handles the request, so it's dispatched directly;
// an empty pattern means ServeMux fell back to its internal 404 or 405
// handler, whose outcome we capture (without writing it to the client) so we
// can re-render it as JSON in the response format the rest of the API uses.
func withRouteFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			h.ServeHTTP(w, r)
			return
		}

		capture := newStatusCapture()
		h.ServeHTTP(capture, r)

		status := capture.status
		if status == 0 {
			status = http.StatusNotFound
		}
		if allow := capture.header.Get("Allow"); allow != "" {
			w.Header().Set("Allow", allow)
		}
		writeRouteError(w, r, status)
	})
}

// statusCapture is a minimal http.ResponseWriter that records the status and
// headers ServeMux's internal 404/405 handlers would have written, without
// letting their plain-text bodies reach the client.
type statusCapture struct {
	header http.Header
	status int
}

func newStatusCapture() *statusCapture {
	return &statusCapture{header: make(http.Header)}
}

func (s *statusCapture) Header() http.Header         { return s.header }
func (s *statusCapture) Write(b []byte) (int, error) { return len(b), nil }
func (s *statusCapture) WriteHeader(status int)      { s.status = status }

// requirePermission wraps a handler with JWT authentication, identity
// resolution, and a permission check, so only callers with a valid token
// carrying the given permission can reach it. These are non-tenant-scoped
// permissions (roles:read, permissions:read), so identity.ResolveMiddleware
// is wired without identity.RequireTenant after it.
func (s *Server) requirePermission(permission string, next http.Handler) http.Handler {
	return auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms, s.jwtIssuer)(
		identity.ResolveMiddleware(s.identityResolver, s.tenantUserCache, s.roleCache)(
			identity.RequirePermission(permission)(next),
		),
	)
}

// requireIdentity wraps a handler with JWT authentication and identity
// resolution only, for endpoints that need an authenticated caller but no
// specific permission or tenant context (GET /auth/me, POST /onboarding).
func (s *Server) requireIdentity(next http.Handler) http.Handler {
	return auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms, s.jwtIssuer)(
		identity.ResolveMiddleware(s.identityResolver, s.tenantUserCache, s.roleCache)(next),
	)
}

// corsMiddleware sets CORS headers. Requests from an origin in
// s.corsConfig's allowlist get that exact origin echoed back with
// credentials enabled; any other origin (or an empty allowlist, the
// default) falls back to the previous wildcard, non-credentialed behavior,
// so existing bearer-token-only callers are unaffected.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.corsConfig.IsAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Credentials", "false")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Tenant-ID")

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Proceed with the next handler
		next.ServeHTTP(w, r)
	})
}

func writeRouteError(w http.ResponseWriter, r *http.Request, status int) {
	code, message := "not_found", "no matching route"
	if status == http.StatusMethodNotAllowed {
		code, message = "method_not_allowed", "method not allowed for this route"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	}); err != nil {
		logger.FromContext(r.Context()).Error("server: failed to write route error response", "error", err.Error())
	}
}
