package rbac_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/rbac"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// seededRoleCount is the number of rows inserted by 00002_create_roles.sql
// (admin, manager, viewer) plus 00012_seed_role_permissions.sql (owner,
// FEAT-012).
const seededRoleCount = 4

// seededPermissionCount is the number of rows inserted by
// 00005_create_permissions.sql (kept in sync with that migration).
const seededPermissionCount = 19

// unreachableRedisClient points at a closed local port so Get/Set calls
// fail fast with a connection error, driving rbac.NewRoleCache down its
// PostgreSQL fallback path — exactly what these integration tests intend
// to exercise, without adding a Redis testcontainer dependency.
func unreachableRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
}

// testDBService is a minimal database.Service backed directly by a *sql.DB,
// used to point rbac.NewRoleCache at an ephemeral testcontainers database
// without depending on the database package's env-var-configured singleton.
type testDBService struct {
	db *sql.DB
}

func (s *testDBService) GetDB() *sql.DB               { return s.db }
func (s *testDBService) Health() database.HealthStats { return database.HealthStats{} }
func (s *testDBService) Close() error                 { return s.db.Close() }

func mustStartMigratedPostgres(t *testing.T) database.Service {
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

	return &testDBService{db: db}
}

func TestNewRoleCache_LoadsSeededRolesAndPermissions(t *testing.T) {
	dbService := mustStartMigratedPostgres(t)

	cache, err := rbac.NewRoleCache(context.Background(), dbService, unreachableRedisClient(), rbac.RoleCacheTTL)
	if err != nil {
		t.Fatalf("NewRoleCache() error = %v", err)
	}

	roles := cache.All()
	if len(roles) != seededRoleCount {
		t.Fatalf("len(All()) = %d, want %d (admin, manager, viewer, owner)", len(roles), seededRoleCount)
	}

	for _, name := range []string{"admin", "manager", "viewer", "owner"} {
		role, ok := cache.GetRole(name)
		if !ok {
			t.Errorf("GetRole(%q) not found", name)
			continue
		}
		if !role.IsSystemRole {
			t.Errorf("GetRole(%q).IsSystemRole = false, want true", name)
		}
		if len(role.Permissions) == 0 {
			t.Errorf("GetRole(%q).Permissions is empty, want seeded permissions", name)
		}
	}

	admin, _ := cache.GetRole("admin")
	viewer, _ := cache.GetRole("viewer")
	if len(admin.Permissions) <= len(viewer.Permissions) {
		t.Errorf("admin has %d permissions, viewer has %d; want admin > viewer (permission hierarchy)", len(admin.Permissions), len(viewer.Permissions))
	}

	if _, ok := cache.GetRole("nonexistent-role"); ok {
		t.Errorf("GetRole(nonexistent-role) found a role, want not found")
	}

	permissions := cache.AllPermissions()
	if len(permissions) != seededPermissionCount {
		t.Fatalf("len(AllPermissions()) = %d, want %d (seeded permission matrix)", len(permissions), seededPermissionCount)
	}

	found := false
	for _, p := range permissions {
		if p.Name == "roles:read" {
			found = true
			if p.Resource != "roles" || p.Action != "read" {
				t.Errorf("roles:read permission = %+v, want resource=roles action=read", p)
			}
		}
	}
	if !found {
		t.Error("roles:read permission not found in seeded permissions")
	}
}

func TestNewRoleCache_MigrationIsIdempotent(t *testing.T) {
	dbService := mustStartMigratedPostgres(t)

	// Re-running migrations against an already-migrated database must not
	// duplicate seed rows (FEAT-002 Edge Cases: "Migration re-run should be
	// idempotent").
	if err := database.Migrate(dbService.GetDB()); err != nil {
		t.Fatalf("re-running database.Migrate() error = %v", err)
	}

	cache, err := rbac.NewRoleCache(context.Background(), dbService, unreachableRedisClient(), rbac.RoleCacheTTL)
	if err != nil {
		t.Fatalf("NewRoleCache() error = %v", err)
	}

	if len(cache.All()) != seededRoleCount {
		t.Errorf("len(All()) = %d after re-migration, want %d (no duplicates)", len(cache.All()), seededRoleCount)
	}
	if len(cache.AllPermissions()) != seededPermissionCount {
		t.Errorf("len(AllPermissions()) = %d after re-migration, want %d (no duplicates)", len(cache.AllPermissions()), seededPermissionCount)
	}
}

func TestRoleCache_Refresh_ReloadsFromPostgres(t *testing.T) {
	dbService := mustStartMigratedPostgres(t)

	cache, err := rbac.NewRoleCache(context.Background(), dbService, unreachableRedisClient(), rbac.RoleCacheTTL)
	if err != nil {
		t.Fatalf("NewRoleCache() error = %v", err)
	}

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if len(cache.All()) != seededRoleCount {
		t.Errorf("len(All()) after Refresh() = %d, want %d (admin, manager, viewer)", len(cache.All()), seededRoleCount)
	}
	if len(cache.AllPermissions()) != seededPermissionCount {
		t.Errorf("len(AllPermissions()) after Refresh() = %d, want %d (seeded permission matrix)", len(cache.AllPermissions()), seededPermissionCount)
	}
	if _, ok := cache.GetRole("admin"); !ok {
		t.Error("GetRole(admin) not found after Refresh()")
	}
}
