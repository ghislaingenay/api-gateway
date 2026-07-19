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
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testDBService is a minimal database.Service backed directly by a *sql.DB,
// used to point rbac.NewRoleCache at an ephemeral testcontainers database
// without depending on the database package's env-var-configured singleton.
type testDBService struct {
	db *sql.DB
}

func (s *testDBService) GetDB() *sql.DB            { return s.db }
func (s *testDBService) Health() map[string]string { return nil }
func (s *testDBService) Close() error              { return s.db.Close() }

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

	cache, err := rbac.NewRoleCache(context.Background(), dbService)
	if err != nil {
		t.Fatalf("NewRoleCache() error = %v", err)
	}

	roles := cache.All()
	if len(roles) != 3 {
		t.Fatalf("len(All()) = %d, want 3 (admin, manager, viewer)", len(roles))
	}

	for _, name := range []string{"admin", "manager", "viewer"} {
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
	if len(permissions) != 19 {
		t.Fatalf("len(AllPermissions()) = %d, want 19 (seeded permission matrix)", len(permissions))
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

	cache, err := rbac.NewRoleCache(context.Background(), dbService)
	if err != nil {
		t.Fatalf("NewRoleCache() error = %v", err)
	}

	if len(cache.All()) != 3 {
		t.Errorf("len(All()) = %d after re-migration, want 3 (no duplicates)", len(cache.All()))
	}
	if len(cache.AllPermissions()) != 19 {
		t.Errorf("len(AllPermissions()) = %d after re-migration, want 19 (no duplicates)", len(cache.AllPermissions()))
	}
}
