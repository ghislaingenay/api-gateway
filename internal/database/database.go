package database

import (
	"api-gateway/config"
	"api-gateway/internal/logger"
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
)

// Service represents a service that interacts with a database.
type Service interface {
	// Health returns a map of health status information.
	// The keys and values in the map are service-specific.
	Health() HealthStats

	// Close terminates the database connection.
	// It returns an error if the connection cannot be closed.
	Close() error
	GetDB() *sql.DB
}

type service struct {
	db       *sql.DB
	database string
}

func (s *service) GetDB() *sql.DB {
	return s.db
}

var (
	dbInstance *service
	once       sync.Once
)

func New(cfg *config.DatabaseConfig) Service {
	// Reuse Connection
	once.Do(func() {
		db, err := sql.Open("pgx", cfg.ConnectionString())
		if err != nil {
			logger.Default().Error("database: failed to open connection", "error", err.Error())
			os.Exit(1)
		}

		dbInstance = &service{
			db:       db,
			database: cfg.DBDatabase,
		}
	})
	return dbInstance
}

type HealthStats struct {
	Status            string   `json:"status"`
	Message           string   `json:"message,omitempty"`
	Warnings          []string `json:"warnings,omitempty"` // Slice to hold multiple warnings
	Error             string   `json:"error,omitempty"`
	OpenConnections   int      `json:"open_connections,omitempty"`
	InUse             int      `json:"in_use,omitempty"`
	Idle              int      `json:"idle,omitempty"`
	WaitCount         int64    `json:"wait_count,omitempty"`
	WaitDuration      string   `json:"wait_duration,omitempty"`
	MaxIdleClosed     int64    `json:"max_idle_closed,omitempty"`
	MaxLifetimeClosed int64    `json:"max_lifetime_closed,omitempty"`
}

func (s *service) Health() HealthStats {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		logger.Default().Error("database: down", "error", err.Error())
		return HealthStats{
			Status: "down",
			Error:  fmt.Sprintf("db down: %v", err),
		}
	}

	dbStats := s.db.Stats()
	
	stats := HealthStats{
		Status:            "up",
		Message:           "It's healthy",
		OpenConnections:   dbStats.OpenConnections,
		InUse:             dbStats.InUse,
		Idle:              dbStats.Idle,
		WaitCount:         dbStats.WaitCount,
		WaitDuration:      dbStats.WaitDuration.String(),
		MaxIdleClosed:     dbStats.MaxIdleClosed,
		MaxLifetimeClosed: dbStats.MaxLifetimeClosed,
	}

	// Use consecutive if statements and append to a slice
	// This way, if 3 things are wrong, you see all 3 warnings in your monitoring tools.
	if dbStats.OpenConnections > 40 { 
		stats.Warnings = append(stats.Warnings, "The database is experiencing heavy load.")
	} 
	
	if dbStats.WaitCount > 1000 {
		stats.Warnings = append(stats.Warnings, "The database has a high number of wait events, indicating potential bottlenecks.")
	} 
	
	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 {
		stats.Warnings = append(stats.Warnings, "Many idle connections are being closed, consider revising the connection pool settings.")
	} 
	
	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats.Warnings = append(stats.Warnings, "Many connections are being closed due to max lifetime, consider increasing max lifetime or revising the connection usage pattern.")
	}

	// If we have warnings, update the main message to reflect degraded performance
	if len(stats.Warnings) > 0 {
		stats.Message = "The database is up but experiencing issues."
	}

	return stats
}

// Close closes the database connection.
// It logs a message indicating the disconnection from the specific database.
// If the connection is successfully closed, it returns nil.
// If an error occurs while closing the connection, it returns the error.
func (s *service) Close() error {
	logger.Default().Info("database: disconnected", "database", s.database)
	return s.db.Close()
}
