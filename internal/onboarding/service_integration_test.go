package onboarding_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/onboarding"

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

func mustInsertUser(t *testing.T, db *sql.DB, sub string) uuid.UUID {
	t.Helper()
	var userID uuid.UUID
	if err := db.QueryRow(`
		INSERT INTO users (keycloak_sub, email) VALUES ($1, $2) RETURNING id
	`, sub, sub+"@example.test").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

func TestService_Onboard_CreatesTenantAndOwnerMembership(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	svc := onboarding.NewService(db)
	userID := mustInsertUser(t, db, "sub-onboard-1")

	tenantID, err := svc.Onboard(context.Background(), userID, "New Org")
	if err != nil {
		t.Fatalf("Onboard() error = %v", err)
	}
	if tenantID == uuid.Nil {
		t.Fatal("Onboard() returned a nil tenant id")
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM tenants WHERE id = $1`, tenantID).Scan(&name); err != nil {
		t.Fatalf("query created tenant: %v", err)
	}
	if name != "New Org" {
		t.Errorf("tenant name = %q, want %q", name, "New Org")
	}

	var roleName string
	if err := db.QueryRow(`
		SELECT r.name FROM tenant_users tu JOIN roles r ON r.id = tu.role_id
		WHERE tu.tenant_id = $1 AND tu.user_id = $2
	`, tenantID, userID).Scan(&roleName); err != nil {
		t.Fatalf("query tenant_users row: %v", err)
	}
	if roleName != "owner" {
		t.Errorf("role = %q, want %q", roleName, "owner")
	}
}

func TestService_Onboard_AllowsMultipleTenantsPerUser(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	svc := onboarding.NewService(db)
	userID := mustInsertUser(t, db, "sub-onboard-2")

	first, err := svc.Onboard(context.Background(), userID, "Org One")
	if err != nil {
		t.Fatalf("first Onboard() error = %v", err)
	}
	second, err := svc.Onboard(context.Background(), userID, "Org Two")
	if err != nil {
		t.Fatalf("second Onboard() error = %v", err)
	}
	if first == second {
		t.Fatal("expected two distinct tenants, got the same id twice")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tenant_users WHERE user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count tenant_users: %v", err)
	}
	if count != 2 {
		t.Errorf("tenant_users count = %d, want 2", count)
	}
}
