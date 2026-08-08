// Command dbflush drops and recreates the local database, reruns goose
// migrations, then reruns cmd/seed — a one-shot "start over" for local
// development. It refuses to run unless APP_ENV=development, matching the
// guard pattern config.LoadFeatureFlags().AllowAutoDBMigration already uses
// for auto-migration-on-boot. Wired as `make db-reset`.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"

	"api-gateway/config"
	"api-gateway/internal/database"
	"api-gateway/internal/logger"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if !config.LoadAppConfig().IsDevelopmentMode() {
		logger.Default().Error("dbflush: refusing to run outside APP_ENV=development")
		os.Exit(1)
	}

	cfg := config.LoadDatabaseConfig()

	if err := dropAndRecreate(cfg); err != nil {
		logger.Default().Error("dbflush: drop/recreate database", "error", err.Error())
		os.Exit(1)
	}
	logger.Default().Info("dbflush: database recreated", "database", cfg.DBDatabase)

	dbService := database.New(cfg)
	defer dbService.Close()
	if err := database.Migrate(dbService.GetDB()); err != nil {
		logger.Default().Error("dbflush: run migrations", "error", err.Error())
		os.Exit(1)
	}
	logger.Default().Info("dbflush: migrations applied")

	if err := runSeed(); err != nil {
		logger.Default().Error("dbflush: run seed", "error", err.Error())
		os.Exit(1)
	}

	logger.Default().Info("dbflush: done")
}

// runSeed invokes cmd/seed. Inside the Dockerfile.dev image, the seed
// binary is already built and on PATH; when dbflush is run directly on
// the host (`make db-reset`), there's no such binary, so it falls back to
// `go run ./cmd/seed`.
func runSeed() error {
	var cmd *exec.Cmd
	if path, err := exec.LookPath("seed"); err == nil {
		cmd = exec.Command(path)
	} else {
		cmd = exec.Command("go", "run", "./cmd/seed")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

// dropAndRecreate connects to Postgres' maintenance "postgres" database
// (rather than cfg.DBDatabase itself, which can't be dropped while the
// current session is connected to it) and drops/recreates cfg.DBDatabase.
func dropAndRecreate(cfg *config.DatabaseConfig) error {
	maintenanceCfg := *cfg
	maintenanceCfg.DBDatabase = "postgres"
	maintenanceCfg.DBSchema = ""

	db, err := sql.Open("pgx", maintenanceCfg.ConnectionString())
	if err != nil {
		return fmt.Errorf("open maintenance connection: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, quoteIdentifier(cfg.DBDatabase))); err != nil {
		return fmt.Errorf("drop database: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`CREATE DATABASE %s`, quoteIdentifier(cfg.DBDatabase))); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return nil
}

// quoteIdentifier double-quotes a Postgres identifier, escaping embedded
// double quotes, so a database name can't break out of the DROP/CREATE
// DATABASE statement it's interpolated into.
func quoteIdentifier(name string) string {
	escaped := ""
	for _, r := range name {
		if r == '"' {
			escaped += `""`
		} else {
			escaped += string(r)
		}
	}
	return `"` + escaped + `"`
}
