package user_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/user"

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

func mustInsertTenantAndRole(t *testing.T, db *sql.DB) (tenantID, roleID uuid.UUID) {
	t.Helper()

	if err := db.QueryRow(`
		INSERT INTO tenants (name, slug, tier) VALUES ('Test Tenant', 'test-tenant', 'free')
		RETURNING id
	`).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'viewer'`).Scan(&roleID); err != nil {
		t.Fatalf("select seeded viewer role: %v", err)
	}

	return tenantID, roleID
}

func TestRepository_GetByEmail_GetByID_UpdateLastLoginAt(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	tenantID, roleID := mustInsertTenantAndRole(t, db)
	repo := user.NewRepository(db)
	ctx := context.Background()

	var userID uuid.UUID
	if err := db.QueryRow(`
		INSERT INTO users (tenant_id, role_id, email, password_hash, is_active)
		VALUES ($1, $2, 'user@test.local', 'hash', true)
		RETURNING id
	`, tenantID, roleID).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	byEmail, err := repo.GetByEmail(ctx, tenantID, "user@test.local")
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if byEmail.ID != userID {
		t.Errorf("GetByEmail().ID = %v, want %v", byEmail.ID, userID)
	}

	byID, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if byID.Email != "user@test.local" {
		t.Errorf("GetByID().Email = %q, want %q", byID.Email, "user@test.local")
	}
	if byID.LastLoginAt != nil {
		t.Errorf("GetByID().LastLoginAt = %v, want nil before any login", byID.LastLoginAt)
	}

	now := time.Now().Truncate(time.Second)
	if err := repo.UpdateLastLoginAt(ctx, userID, now); err != nil {
		t.Fatalf("UpdateLastLoginAt() error = %v", err)
	}

	updated, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}
	if updated.LastLoginAt == nil || !updated.LastLoginAt.Equal(now) {
		t.Errorf("GetByID().LastLoginAt = %v, want %v", updated.LastLoginAt, now)
	}
}

func TestRepository_GetByEmail_NotFound(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	tenantID, _ := mustInsertTenantAndRole(t, db)
	repo := user.NewRepository(db)

	_, err := repo.GetByEmail(context.Background(), tenantID, "nobody@test.local")
	if err != user.ErrUserNotFound {
		t.Errorf("GetByEmail() error = %v, want ErrUserNotFound", err)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := user.NewRepository(db)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != user.ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want ErrUserNotFound", err)
	}
}
