package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/config"
	"api-gateway/internal/cache"
	"api-gateway/internal/database"
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
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	defer redisClient.Close()
	fmt.Println("Redis connected successfully!")

	// 3. Initialize Database
	dbService := database.New()
	defer dbService.Close()
	fmt.Println("Database connected successfully!")


	featureFlags := config.LoadFeatureFlags()
	allowAutoDBMigration := featureFlags.AllowAutoDBMigration

	if allowAutoDBMigration {
		if err := database.Migrate(dbService.GetDB()); err != nil {
			log.Fatalf("Could not run database migrations: %v", err)
		}
		fmt.Println("Database migrations applied successfully!")
	} else {
		fmt.Println("Auto database migration is disabled. Skipping migrations.")
	}

	// 4. Inject dependencies into your application struct
	app := &App{
		Config: cfg,
		Redis:  redisClient,
		Db:     dbService,
	}

	// 5. Pass the app instance to the server
	server := server.NewServer(app.Db.GetDB())

	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine
	go gracefulShutdown(server, done)

	log.Println("Server is ready to handle requests at :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", ":8080", err)
	}

	// Wait for the graceful shutdown to complete
	<-done
	log.Println("Graceful shutdown complete.")
}

func gracefulShutdown(apiServer *http.Server, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	log.Println("shutting down gracefully, press Ctrl+C again to force")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown with error: %v", err)
	}

	log.Println("Server exiting")

	// Notify the main goroutine that the shutdown is complete
	done <- true
}