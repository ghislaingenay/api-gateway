package server

import (
	"api-gateway/config"
	"api-gateway/internal/auth"
	"api-gateway/internal/database"
	"api-gateway/internal/gateway"
	"api-gateway/internal/rbac"
	"api-gateway/internal/tenant"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port          int
	db            database.Service
	roleCache     rbac.RoleCache
	keyStore      auth.KeyStore
	jwtAlgorithms []string
	routeTable    *gateway.RouteTable
	tenantStatus  gateway.TenantStatusChecker
	proxy         gateway.Proxier
}

func NewServer(db *sql.DB, redisClient *redis.Client) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	dbService := database.New()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	roleCache, err := rbac.NewRoleCache(ctx, dbService)
	if err != nil {
		log.Fatalf("failed to load role cache: %v", err)
	}

	jwtConfig := config.LoadJWTConfig()
	keyStore, err := auth.NewKeyStore(jwtConfig)
	if err != nil {
		log.Fatalf("failed to load JWT key store: %v", err)
	}

	routeEntries, err := config.LoadRoutesConfig()
	if err != nil {
		log.Fatalf("failed to load routes config: %v", err)
	}
	routeTable := gateway.NewRouteTable(toGatewayRoutes(routeEntries))

	tenantRepo := tenant.NewRepository(dbService.GetDB())
	tenantStatus := tenant.NewStatusCache(tenantRepo, redisClient, tenant.StatusCacheTTL)

	NewServer := &Server{
		port:          port,
		db:            dbService,
		roleCache:     roleCache,
		keyStore:      keyStore,
		jwtAlgorithms: jwtConfig.AllowedAlgorithms,
		routeTable:    routeTable,
		tenantStatus:  tenantStatus,
		proxy:         gateway.NewReverseProxier(),
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

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
		}
	}
	return routes
}
