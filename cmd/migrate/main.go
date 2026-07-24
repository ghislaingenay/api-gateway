// Command migrate applies pending database migrations and exits. It is the
// one-shot migration job run ahead of the gateway server (coding-standards.md
// "Database Migrations"): the server itself never migrates on production
// startup.
package main

import (
	"os"

	"api-gateway/config"
	"api-gateway/internal/database"
	"api-gateway/internal/logger"
)

func main() {
	dbService := database.New(config.LoadDatabaseConfig())
	defer dbService.Close()

	if err := database.Migrate(dbService.GetDB()); err != nil {
		logger.Default().Error("migrate: could not run database migrations", "error", err.Error())
		os.Exit(1)
	}

	logger.Default().Info("migrate: database migrations applied successfully")
}
