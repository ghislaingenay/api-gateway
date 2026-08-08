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

func TestRepository_GetByKeycloakSub_GetByID(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := user.NewRepository(db)
	ctx := context.Background()

	var userID uuid.UUID
	if err := db.QueryRow(`
		INSERT INTO users (keycloak_sub, email)
		VALUES ('sub-1', 'user@test.local')
		RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	bySub, err := repo.GetByKeycloakSub(ctx, "sub-1")
	if err != nil {
		t.Fatalf("GetByKeycloakSub() error = %v", err)
	}
	if bySub.ID != userID {
		t.Errorf("GetByKeycloakSub().ID = %v, want %v", bySub.ID, userID)
	}

	byID, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if byID.Email != "user@test.local" {
		t.Errorf("GetByID().Email = %q, want %q", byID.Email, "user@test.local")
	}
}

func TestRepository_GetByKeycloakSub_NotFound(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := user.NewRepository(db)

	_, err := repo.GetByKeycloakSub(context.Background(), "nonexistent-sub")
	if err != user.ErrUserNotFound {
		t.Errorf("GetByKeycloakSub() error = %v, want ErrUserNotFound", err)
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

func TestRepository_EnsureByKeycloakSub_JITProvisionsOnce(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := user.NewRepository(db)
	ctx := context.Background()

	first, err := repo.EnsureByKeycloakSub(ctx, "sub-jit", "jit@test.local")
	if err != nil {
		t.Fatalf("EnsureByKeycloakSub() error = %v", err)
	}

	second, err := repo.EnsureByKeycloakSub(ctx, "sub-jit", "jit@test.local")
	if err != nil {
		t.Fatalf("EnsureByKeycloakSub() second call error = %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("EnsureByKeycloakSub() created a duplicate row: first.ID = %v, second.ID = %v", first.ID, second.ID)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE keycloak_sub = 'sub-jit'`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Errorf("users row count for sub-jit = %d, want 1", count)
	}
}

func TestRepository_ListTenantAccess(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	tenantID, roleID := mustInsertTenantAndRole(t, db)
	repo := user.NewRepository(db)
	ctx := context.Background()

	u, err := repo.EnsureByKeycloakSub(ctx, "sub-access", "access@test.local")
	if err != nil {
		t.Fatalf("EnsureByKeycloakSub() error = %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, $3)
	`, tenantID, u.ID, roleID); err != nil {
		t.Fatalf("insert tenant_users: %v", err)
	}

	access, err := repo.ListTenantAccess(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListTenantAccess() error = %v", err)
	}
	if len(access) != 1 {
		t.Fatalf("ListTenantAccess() returned %d entries, want 1", len(access))
	}
	if access[0].TenantID != tenantID {
		t.Errorf("ListTenantAccess()[0].TenantID = %v, want %v", access[0].TenantID, tenantID)
	}
	if access[0].RoleName != "viewer" {
		t.Errorf("ListTenantAccess()[0].RoleName = %q, want %q", access[0].RoleName, "viewer")
	}
}

func TestRepository_ListTenantAccess_Empty(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := user.NewRepository(db)
	ctx := context.Background()

	u, err := repo.EnsureByKeycloakSub(ctx, "sub-no-access", "no-access@test.local")
	if err != nil {
		t.Fatalf("EnsureByKeycloakSub() error = %v", err)
	}

	access, err := repo.ListTenantAccess(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListTenantAccess() error = %v", err)
	}
	if len(access) != 0 {
		t.Errorf("ListTenantAccess() returned %d entries, want 0", len(access))
	}
}
