package database

import (
	"database/sql"
	"testing"
)

// openTestDB opens a dedicated connection to the shared test container,
// independent of the package-level singleton used by database_test.go.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// search_path is hardcoded to "public" here rather than reusing
	// testDBConfig.DBSchema, since that field is only populated when
	// godotenv/autoload finds a repo-root .env file — which depends on the
	// test's working directory and isn't guaranteed across environments.
	cfg := *testDBConfig
	cfg.DBSchema = "public"
	db, err := sql.Open("pgx", cfg.ConnectionString())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

func TestMigrate(t *testing.T) {
	db := openTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	// Re-running the migration must be idempotent.
	if err := Migrate(db); err != nil {
		t.Fatalf("re-running Migrate() returned error: %v", err)
	}

	for _, table := range []string{"tenants", "roles", "users", "profiles"} {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %q exists: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migration", table)
		}
	}

	for _, index := range []string{
		"idx_tenants_slug", "idx_tenants_is_active",
		"idx_users_tenant_id", "idx_users_role_id", "idx_users_email", "idx_users_is_active",
		"idx_profiles_user_id",
	} {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`, index).Scan(&exists)
		if err != nil {
			t.Fatalf("checking index %q exists: %v", index, err)
		}
		if !exists {
			t.Errorf("expected index %q to exist after migration", index)
		}
	}
}

func TestMigrate_TenantSlugUniqueness(t *testing.T) {
	db := openTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO tenants (name, slug) VALUES ('Tenant A', 'dup-slug-test')`); err != nil {
		t.Fatalf("inserting first tenant: %v", err)
	}

	_, err := db.Exec(`INSERT INTO tenants (name, slug) VALUES ('Tenant B', 'dup-slug-test')`)
	if err == nil {
		t.Fatal("expected duplicate slug insert to fail, got nil error")
	}
}

func TestMigrate_ForeignKeyAndUniqueConstraints(t *testing.T) {
	db := openTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	var roleID string
	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'admin'`).Scan(&roleID); err != nil {
		t.Fatalf("querying seeded admin role: %v", err)
	}

	var tenantAID, tenantBID string
	if err := db.QueryRow(`INSERT INTO tenants (name, slug) VALUES ('Tenant A', 'tenant-a-fk-test') RETURNING id`).Scan(&tenantAID); err != nil {
		t.Fatalf("inserting tenant A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO tenants (name, slug) VALUES ('Tenant B', 'tenant-b-fk-test') RETURNING id`).Scan(&tenantBID); err != nil {
		t.Fatalf("inserting tenant B: %v", err)
	}

	t.Run("user with non-existent role_id fails FK constraint", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, gen_random_uuid(), 'nofk@example.com', 'hash')`, tenantAID)
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})

	t.Run("same email in different tenants succeeds", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, $2, 'same@example.com', 'hash')`, tenantAID, roleID)
		if err != nil {
			t.Fatalf("inserting user in tenant A: %v", err)
		}

		_, err = db.Exec(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, $2, 'same@example.com', 'hash')`, tenantBID, roleID)
		if err != nil {
			t.Fatalf("expected insert in different tenant to succeed, got: %v", err)
		}
	})

	t.Run("same email in same tenant fails unique constraint", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, $2, 'dup@example.com', 'hash')`, tenantAID, roleID)
		if err != nil {
			t.Fatalf("inserting first user: %v", err)
		}

		_, err = db.Exec(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, $2, 'dup@example.com', 'hash')`, tenantAID, roleID)
		if err == nil {
			t.Fatal("expected unique_email_per_tenant violation, got nil")
		}
	})

	t.Run("deleting a tenant cascades to delete its users", func(t *testing.T) {
		var cascadeTenantID string
		if err := db.QueryRow(`INSERT INTO tenants (name, slug) VALUES ('Tenant Cascade', 'tenant-cascade-test') RETURNING id`).Scan(&cascadeTenantID); err != nil {
			t.Fatalf("inserting cascade tenant: %v", err)
		}

		var userID string
		if err := db.QueryRow(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, $2, 'cascade@example.com', 'hash') RETURNING id`, cascadeTenantID, roleID).Scan(&userID); err != nil {
			t.Fatalf("inserting user for cascade test: %v", err)
		}

		if _, err := db.Exec(`DELETE FROM tenants WHERE id = $1`, cascadeTenantID); err != nil {
			t.Fatalf("deleting tenant: %v", err)
		}

		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
			t.Fatalf("checking user existence: %v", err)
		}
		if exists {
			t.Error("expected user to be cascade-deleted with its tenant")
		}
	})

	t.Run("deleting a user cascades to delete their profile", func(t *testing.T) {
		var userID string
		if err := db.QueryRow(`INSERT INTO users (tenant_id, role_id, email, password_hash) VALUES ($1, $2, 'profile-cascade@example.com', 'hash') RETURNING id`, tenantAID, roleID).Scan(&userID); err != nil {
			t.Fatalf("inserting user for profile cascade test: %v", err)
		}

		var profileID string
		if err := db.QueryRow(`INSERT INTO profiles (user_id, first_name) VALUES ($1, 'Test') RETURNING id`, userID).Scan(&profileID); err != nil {
			t.Fatalf("inserting profile: %v", err)
		}

		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Fatalf("deleting user: %v", err)
		}

		var exists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM profiles WHERE id = $1)`, profileID).Scan(&exists); err != nil {
			t.Fatalf("checking profile existence: %v", err)
		}
		if exists {
			t.Error("expected profile to be cascade-deleted with its user")
		}
	})
}
