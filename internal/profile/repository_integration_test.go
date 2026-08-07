package profile_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/profile"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func mustStartMigratedPostgres(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("database"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("could not start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("could not terminate postgres container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container.Host() error = %v", err)
	}
	mappedPort, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container.MappedPort() error = %v", err)
	}

	connStr := fmt.Sprintf("postgres://user:password@%s:%s/database?sslmode=disable", host, mappedPort.Port())
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	return db
}

func TestRepository_GetByUserID(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := profile.NewRepository(db)
	ctx := context.Background()

	var userID uuid.UUID
	if err := db.QueryRow(`
		INSERT INTO users (keycloak_sub, email) VALUES ('sub-profile', 'profile@example.test') RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO profiles (user_id, first_name, last_name, timezone) VALUES ($1, 'Ada', 'Lovelace', 'Europe/London')
	`, userID); err != nil {
		t.Fatalf("insert profile: %v", err)
	}

	p, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if p.FirstName == nil || *p.FirstName != "Ada" {
		t.Errorf("FirstName = %v, want Ada", p.FirstName)
	}
	if p.Timezone != "Europe/London" {
		t.Errorf("Timezone = %q, want Europe/London", p.Timezone)
	}
}

func TestRepository_GetByUserID_NotFound(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := profile.NewRepository(db)

	_, err := repo.GetByUserID(context.Background(), uuid.New())
	if err != profile.ErrProfileNotFound {
		t.Errorf("GetByUserID() error = %v, want ErrProfileNotFound", err)
	}
}
