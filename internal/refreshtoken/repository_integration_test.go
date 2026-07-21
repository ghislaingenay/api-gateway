package refreshtoken_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/refreshtoken"

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

// mustInsertUser inserts a minimal tenant/role/user row set directly via SQL
// so this package's tests don't depend on internal/user's repository.
func mustInsertUser(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()

	var tenantID uuid.UUID
	if err := db.QueryRow(`
		INSERT INTO tenants (name, slug, tier) VALUES ('Test Tenant', 'test-tenant', 'free')
		RETURNING id
	`).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	var roleID uuid.UUID
	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'viewer'`).Scan(&roleID); err != nil {
		t.Fatalf("select seeded viewer role: %v", err)
	}

	var userID uuid.UUID
	if err := db.QueryRow(`
		INSERT INTO users (tenant_id, role_id, email, password_hash)
		VALUES ($1, $2, 'user@test.local', 'hash')
		RETURNING id
	`, tenantID, roleID).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	return userID
}

func TestRepository_CreateGetByHashRevoke(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	userID := mustInsertUser(t, db)
	repo := refreshtoken.NewRepository(db)
	ctx := context.Background()

	token := refreshtoken.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: "abc123hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByHash(ctx, "abc123hash")
	if err != nil {
		t.Fatalf("GetByHash() error = %v", err)
	}
	if got.ID != token.ID || got.UserID != userID {
		t.Errorf("GetByHash() = %+v, want ID=%v UserID=%v", got, token.ID, userID)
	}
	if got.RevokedAt != nil {
		t.Errorf("GetByHash() RevokedAt = %v, want nil for a fresh token", got.RevokedAt)
	}
	if !got.Valid(time.Now()) {
		t.Error("fresh token Valid() = false, want true")
	}

	if err := repo.Revoke(ctx, token.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	revoked, err := repo.GetByHash(ctx, "abc123hash")
	if err != nil {
		t.Fatalf("GetByHash() after revoke error = %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Error("GetByHash() after revoke RevokedAt = nil, want non-nil")
	}
	if revoked.Valid(time.Now()) {
		t.Error("revoked token Valid() = true, want false")
	}
}

func TestRepository_GetByHash_NotFound(t *testing.T) {
	db := mustStartMigratedPostgres(t)
	repo := refreshtoken.NewRepository(db)

	_, err := repo.GetByHash(context.Background(), "does-not-exist")
	if err != refreshtoken.ErrNotFound {
		t.Errorf("GetByHash() error = %v, want ErrNotFound", err)
	}
}
