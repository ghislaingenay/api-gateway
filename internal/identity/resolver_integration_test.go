package identity_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/identity"
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

func TestResolver_EnsureUser_JITProvisionsWithoutTenantMembership(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	resolver := identity.NewResolver(db, user.NewRepository(db))
	ctx := context.Background()

	userID, err := resolver.EnsureUser(ctx, "sub-new", "new@example.test")
	if err != nil {
		t.Fatalf("EnsureUser() error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tenant_users WHERE user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count tenant_users: %v", err)
	}
	if count != 0 {
		t.Errorf("tenant_users count for newly JIT-provisioned user = %d, want 0", count)
	}

	again, err := resolver.EnsureUser(ctx, "sub-new", "new@example.test")
	if err != nil {
		t.Fatalf("second EnsureUser() error = %v", err)
	}
	if again != userID {
		t.Errorf("second EnsureUser() = %v, want %v (same row reused)", again, userID)
	}
}

func TestResolver_ResolveTenantUser(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	resolver := identity.NewResolver(db, user.NewRepository(db))
	ctx := context.Background()

	var tenantID, roleID uuid.UUID
	if err := db.QueryRow(`INSERT INTO tenants (name, slug) VALUES ('T', 't-resolver') RETURNING id`).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'viewer'`).Scan(&roleID); err != nil {
		t.Fatalf("select viewer role: %v", err)
	}

	userID, err := resolver.EnsureUser(ctx, "sub-member", "member@example.test")
	if err != nil {
		t.Fatalf("EnsureUser() error = %v", err)
	}

	if _, err := resolver.ResolveTenantUser(ctx, userID, tenantID); !errors.Is(err, identity.ErrNotMember) {
		t.Fatalf("ResolveTenantUser() before membership exists: error = %v, want ErrNotMember", err)
	}

	if _, err := db.Exec(`INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, $3)`, tenantID, userID, roleID); err != nil {
		t.Fatalf("insert tenant_users: %v", err)
	}

	tu, err := resolver.ResolveTenantUser(ctx, userID, tenantID)
	if err != nil {
		t.Fatalf("ResolveTenantUser() error = %v", err)
	}
	if tu.RoleID != roleID {
		t.Errorf("RoleID = %v, want %v", tu.RoleID, roleID)
	}
}
