package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/config"
	"api-gateway/internal/cache"
	"api-gateway/internal/database"
	"api-gateway/internal/logger"
	"api-gateway/internal/server"

	"github.com/redis/go-redis/v9"
)

// App is a central struct that holds your dependencies.
// This prevents you from needing global variables.
type App struct {
	Config *config.Config
	Redis  *redis.Client
	Db     database.Service
}

func main() {
	// 1. Load Configuration
	cfg := config.Load()

	// 2. Initialize Redis
	redisClient, err := cache.NewRedisClient(cfg.RedisURL)
	if err != nil {
		logger.Default().Error("api: could not connect to redis", "error", err.Error())
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Default().Info("api: redis connected successfully")

	// 3. Initialize Database
	dbService := database.New()
	defer dbService.Close()
	logger.Default().Info("api: database connected successfully")

	featureFlags := config.LoadFeatureFlags()
	allowAutoDBMigration := featureFlags.AllowAutoDBMigration

	if allowAutoDBMigration {
		if err := database.Migrate(dbService.GetDB()); err != nil {
			logger.Default().Error("api: could not run database migrations", "error", err.Error())
			os.Exit(1)
		}
		logger.Default().Info("api: database migrations applied successfully")
	} else {
		logger.Default().Info("api: auto database migration is disabled, skipping migrations")
	}

	// 4. Inject dependencies into your application struct
	app := &App{
		Config: cfg,
		Redis:  redisClient,
		Db:     dbService,
	}

	// 5. Pass the app instance to the server
	server := server.NewServer(app.Db.GetDB(), app.Redis)

	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine
	go gracefulShutdown(server, done)

	logger.Default().Info("api: server is ready to handle requests", "addr", ":8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Default().Error("api: could not listen", "addr", ":8080", "error", err.Error())
		os.Exit(1)
	}

	// Wait for the graceful shutdown to complete
	<-done
	logger.Default().Info("api: graceful shutdown complete")
}

func gracefulShutdown(apiServer *http.Server, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	logger.Default().Info("api: shutting down gracefully, press Ctrl+C again to force")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		logger.Default().Error("api: server forced to shutdown", "error", err.Error())
	}

	logger.Default().Info("api: server exiting")

	// Notify the main goroutine that the shutdown is complete
	done <- true
}
