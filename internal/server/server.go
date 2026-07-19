package server

import (
	"api-gateway/config"
	"api-gateway/internal/auth"
	"api-gateway/internal/database"
	"api-gateway/internal/rbac"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port          int
	db            database.Service
	roleCache     rbac.RoleCache
	keyStore      auth.KeyStore
	jwtAlgorithms []string
}

func NewServer(db *sql.DB) *http.Server {
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

	NewServer := &Server{
		port:          port,
		db:            dbService,
		roleCache:     roleCache,
		keyStore:      keyStore,
		jwtAlgorithms: jwtConfig.AllowedAlgorithms,
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
