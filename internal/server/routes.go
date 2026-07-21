package server

import (
	"encoding/json"
	"log"
	"net/http"

	"api-gateway/internal/auth"
	"api-gateway/internal/authhandler"
	"api-gateway/internal/cache"
	"api-gateway/internal/gateway"
	"api-gateway/internal/ratelimit"
	"api-gateway/internal/rbac"
	"api-gateway/internal/validation"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/", s.HelloWorldHandler)

	mux.HandleFunc("/health", s.healthHandler)

	mux.Handle("GET /roles", s.requirePermission("roles:read", rbac.RolesHandler(s.roleCache)))
	mux.Handle("GET /permissions", s.requirePermission("roles:read", rbac.PermissionsHandler(s.roleCache)))

	mux.Handle("POST /auth/login", authhandler.LoginHandler(s.userRepo, s.tenantRepo, s.refreshTokens, s.roleCache, s.signer))
	mux.Handle("POST /auth/refresh", authhandler.RefreshHandler(s.refreshTokens, s.userRepo, s.roleCache, s.signer))
	mux.Handle("POST /auth/logout", s.requireAuth(authhandler.LogoutHandler(s.refreshTokens)))
	mux.Handle("GET /auth/me", s.requireAuth(authhasndler.MeHandler(s.userRepo, s.roleCache)))

	mux.Handle("/api/", auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms)(
		validation.ValidationMiddleware(s.routeTable, s.validationMaxBodyBytes)(
			ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits, s.rateLimitDefs)(
				cache.CacheMiddleware(s.responseCache, s.routeTable, s.tenantStatus, s.cacheDefaultTTL)(
					gateway.NewHandler(s.routeTable, s.tenantStatus, s.proxy),
				),
			),
		),
	))

	// Wrap the mux with CORS middleware
	return s.corsMiddleware(mux)
}

// requirePermission wraps a handler with JWT authentication and a permission
// check, so only callers with a valid token carrying the given permission
// can reach it.
func (s *Server) requirePermission(permission string, next http.HandlerFunc) http.Handler {
	return auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms)(auth.RequirePermission(permission)(next))
}

// requireAuth wraps a handler with JWT authentication only, for endpoints
// that need an authenticated caller but no specific permission.
func (s *Server) requireAuth(next http.HandlerFunc) http.Handler {
	return auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms)(next)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Replace "*" with specific origins if needed
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "false") // Set to "true" if credentials are required

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Proceed with the next handler
		next.ServeHTTP(w, r)
	})
}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"message": "Hello World"}
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResp); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := json.Marshal(s.db.Health())
	if err != nil {
		http.Error(w, "Failed to marshal health check response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
