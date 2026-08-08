package server

import (
	"api-gateway/config"
	"api-gateway/internal/auth"
	"api-gateway/internal/cache"
	"api-gateway/internal/database"
	"api-gateway/internal/gateway"
	"api-gateway/internal/health"
	"api-gateway/internal/identity"
	"api-gateway/internal/logger"
	"api-gateway/internal/onboarding"
	"api-gateway/internal/profile"
	"api-gateway/internal/ratelimit"
	"api-gateway/internal/rbac"
	"api-gateway/internal/resilience"
	"api-gateway/internal/tenant"
	"api-gateway/internal/user"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port                   int
	roleCache              rbac.RoleCache
	keyStore               auth.KeyStore
	jwtAlgorithms          []string
	jwtIssuer              string
	routeTable             *gateway.RouteTable
	tenantStatus           gateway.TenantStatusChecker
	proxy                  gateway.Proxier
	rateLimiter            ratelimit.MultiWindowLimiter
	rateLimits             ratelimit.LimitsProvider
	rateLimitDefs          ratelimit.Defaults
	responseCache          cache.ResponseCache
	cacheDefaultTTL        time.Duration
	validationMaxBodyBytes int64
	healthChecker          *health.DependencyChecker
	userRepo               user.Repository
	tenantRepo             tenant.Repository
	profileRepo            profile.Repository
	identityResolver       identity.Resolver
	tenantUserCache        *identity.TenantUserCache
	onboardingService      onboarding.Service
	corsConfig             *config.CORSConfig
}

func NewServer(redisClient *redis.Client) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	dbService := database.New(config.LoadDatabaseConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	roleCache, err := rbac.NewRoleCache(ctx, dbService, redisClient, rbac.RoleCacheTTL)
	if err != nil {
		logger.Default().Error("server: failed to load role cache", "error", err.Error())
		os.Exit(1)
	}

	jwtConfig := config.LoadJWTConfig()
	if jwtConfig.Issuer == "" {
		logger.Default().Error("server: JWT_ISSUER is required and was not set")
		os.Exit(1)
	}
	keyStore, err := auth.NewKeyStore(jwtConfig)
	if err != nil {
		logger.Default().Error("server: failed to load JWT key store", "error", err.Error())
		os.Exit(1)
	}

	routeEntries, err := config.LoadRoutesConfig()
	if err != nil {
		logger.Default().Error("server: failed to load routes config", "error", err.Error())
		os.Exit(1)
	}
	routeTable := gateway.NewRouteTable(toGatewayRoutes(routeEntries))

	tenantRepo := tenant.NewRepository(dbService.GetDB())
	tenantStatus := tenant.NewStatusCache(tenantRepo, tenant.StatusCacheTTL)

	rateLimitConfig := config.LoadRateLimitConfig()
	cacheConfig := config.LoadCacheConfig()
	resilienceConfig := config.LoadResilienceConfig()
	validationConfig := config.LoadValidationConfig()
	identityConfig := config.LoadIdentityConfig()
	corsConfig := config.LoadCORSConfig()

	slidingWindowLimiter := ratelimit.NewSlidingWindowLimiter(redisClient)

	userRepo := user.NewRepository(dbService.GetDB())
	identityResolver := identity.NewResolver(dbService.GetDB(), userRepo)
	tenantUserCache := identity.NewTenantUserCache(identityResolver, redisClient, identityConfig.CacheTTL)

	NewServer := &Server{
		port:          port,
		roleCache:     roleCache,
		keyStore:      keyStore,
		jwtAlgorithms: jwtConfig.AllowedAlgorithms,
		jwtIssuer:     jwtConfig.Issuer,
		routeTable:    routeTable,
		tenantStatus:  tenantStatus,
		proxy: gateway.NewResilientProxier(
			gateway.NewReverseProxier(),
			resilienceConfig.DefaultTimeout,
			resilience.RetryPolicy{
				MaxAttempts: resilienceConfig.DefaultMaxAttempts,
				BaseBackoff: resilienceConfig.DefaultBaseBackoff,
			},
		),
		rateLimiter: slidingWindowLimiter,
		rateLimits:  tenantStatus,
		rateLimitDefs: ratelimit.Defaults{
			PerMinute: rateLimitConfig.DefaultPerMinute,
			PerHour:   rateLimitConfig.DefaultPerHour,
		},
		responseCache:          cache.NewResponseCache(redisClient),
		cacheDefaultTTL:        cacheConfig.DefaultTTL,
		validationMaxBodyBytes: validationConfig.MaxBodyBytes,
		healthChecker:          health.NewDependencyChecker(redisClient, dbService.GetDB()),
		userRepo:               userRepo,
		tenantRepo:             tenantRepo,
		profileRepo:            profile.NewRepository(dbService.GetDB()),
		identityResolver:       identityResolver,
		tenantUserCache:        tenantUserCache,
		onboardingService:      onboarding.NewService(dbService.GetDB()),
		corsConfig:             corsConfig,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	server.RegisterOnShutdown(tenantUserCache.Close)

	return server
}

func toGatewayRoutes(entries []config.RouteEntry) []gateway.Route {
	routes := make([]gateway.Route, len(entries))
	for i, e := range entries {
		routes[i] = gateway.Route{
			Path:                e.Path,
			Method:              e.Method,
			Upstream:            e.Upstream,
			AuthRequired:        e.AuthRequired,
			PermissionsRequired: e.PermissionsRequired,
			CacheTTL:            time.Duration(e.CacheTTLSeconds) * time.Second,
			Timeout:             time.Duration(e.TimeoutSeconds) * time.Second,
			RetryMaxAttempts:    e.RetryMaxAttempts,
			BodySchema:          toBodySchema(e),
			RequiredParams:      toRequiredParams(e.RequiredParams),
		}
	}
	return routes
}

// toBodySchema builds a route's body validation schema from its config
// entry, or returns nil if the route has no body validation configured
// (FEAT-007).
func toBodySchema(e config.RouteEntry) *gateway.BodySchema {
	if !e.BodyRequired && len(e.BodyFields) == 0 {
		return nil
	}
	fields := make([]gateway.FieldRule, len(e.BodyFields))
	for i, f := range e.BodyFields {
		fields[i] = gateway.FieldRule{Field: f.Field, Rule: f.Rule}
	}
	return &gateway.BodySchema{Required: e.BodyRequired, Fields: fields}
}

func toRequiredParams(entries []config.RequiredParamEntry) []gateway.ParamRule {
	if len(entries) == 0 {
		return nil
	}
	params := make([]gateway.ParamRule, len(entries))
	for i, p := range entries {
		params[i] = gateway.ParamRule{Name: p.Name, In: gateway.ParamLocation(p.In), Rule: p.Rule}
	}
	return params
}
