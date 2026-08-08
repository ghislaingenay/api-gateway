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

	for _, table := range []string{"tenants", "roles", "users", "profiles", "tenant_users", "role_permissions"} {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %q exists: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migration", table)
		}
	}

	for _, table := range []string{"refresh_tokens"} {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %q doesn't exist: %v", table, err)
		}
		if exists {
			t.Errorf("expected table %q to have been dropped by migration (FEAT-012)", table)
		}
	}

	for _, index := range []string{
		"idx_tenants_slug", "idx_tenants_is_active",
		"idx_users_email",
		"idx_profiles_user_id",
		"idx_tenant_users_user_id", "idx_tenant_users_role_id",
		"idx_role_permissions_permission_id",
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

	t.Run("duplicate keycloak_sub fails unique constraint", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO users (email, keycloak_sub) VALUES ('dup-sub-a@example.com', 'dup-sub')`)
		if err != nil {
			t.Fatalf("inserting first user: %v", err)
		}

		_, err = db.Exec(`INSERT INTO users (email, keycloak_sub) VALUES ('dup-sub-b@example.com', 'dup-sub')`)
		if err == nil {
			t.Fatal("expected duplicate keycloak_sub insert to fail, got nil error")
		}
	})

	t.Run("same email for different users succeeds (email no longer unique)", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO users (email, keycloak_sub) VALUES ('same@example.com', 'same-email-a')`)
		if err != nil {
			t.Fatalf("inserting first user: %v", err)
		}

		_, err = db.Exec(`INSERT INTO users (email, keycloak_sub) VALUES ('same@example.com', 'same-email-b')`)
		if err != nil {
			t.Fatalf("expected second insert with duplicate email to succeed, got: %v", err)
		}
	})

	t.Run("tenant_users with non-existent role_id fails FK constraint", func(t *testing.T) {
		var userID string
		if err := db.QueryRow(`INSERT INTO users (email, keycloak_sub) VALUES ('nofk@example.com', 'nofk-sub') RETURNING id`).Scan(&userID); err != nil {
			t.Fatalf("inserting user: %v", err)
		}

		_, err := db.Exec(`INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, gen_random_uuid())`, tenantAID, userID)
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})

	t.Run("a user can belong to more than one tenant", func(t *testing.T) {
		var userID string
		if err := db.QueryRow(`INSERT INTO users (email, keycloak_sub) VALUES ('multi-tenant@example.com', 'multi-tenant-sub') RETURNING id`).Scan(&userID); err != nil {
			t.Fatalf("inserting user: %v", err)
		}

		if _, err := db.Exec(`INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, $3)`, tenantAID, userID, roleID); err != nil {
			t.Fatalf("inserting membership in tenant A: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, $3)`, tenantBID, userID, roleID); err != nil {
			t.Fatalf("expected membership in a second tenant to succeed, got: %v", err)
		}
	})

	t.Run("deleting a tenant cascades to delete its tenant_users rows, not the user", func(t *testing.T) {
		var cascadeTenantID string
		if err := db.QueryRow(`INSERT INTO tenants (name, slug) VALUES ('Tenant Cascade', 'tenant-cascade-test') RETURNING id`).Scan(&cascadeTenantID); err != nil {
			t.Fatalf("inserting cascade tenant: %v", err)
		}

		var userID string
		if err := db.QueryRow(`INSERT INTO users (email, keycloak_sub) VALUES ('cascade@example.com', 'cascade-sub') RETURNING id`).Scan(&userID); err != nil {
			t.Fatalf("inserting user for cascade test: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, $3)`, cascadeTenantID, userID, roleID); err != nil {
			t.Fatalf("inserting membership: %v", err)
		}

		if _, err := db.Exec(`DELETE FROM tenants WHERE id = $1`, cascadeTenantID); err != nil {
			t.Fatalf("deleting tenant: %v", err)
		}

		var membershipExists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM tenant_users WHERE tenant_id = $1)`, cascadeTenantID).Scan(&membershipExists); err != nil {
			t.Fatalf("checking membership existence: %v", err)
		}
		if membershipExists {
			t.Error("expected tenant_users row to be cascade-deleted with its tenant")
		}

		var userExists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&userExists); err != nil {
			t.Fatalf("checking user existence: %v", err)
		}
		if !userExists {
			t.Error("expected user to survive its tenant's deletion (users are no longer tenant-scoped)")
		}
	})

	t.Run("deleting a user cascades to delete their profile and tenant_users rows", func(t *testing.T) {
		var userID string
		if err := db.QueryRow(`INSERT INTO users (email, keycloak_sub) VALUES ('profile-cascade@example.com', 'profile-cascade-sub') RETURNING id`).Scan(&userID); err != nil {
			t.Fatalf("inserting user for profile cascade test: %v", err)
		}

		var profileID string
		if err := db.QueryRow(`INSERT INTO profiles (user_id, first_name) VALUES ($1, 'Test') RETURNING id`, userID).Scan(&profileID); err != nil {
			t.Fatalf("inserting profile: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO tenant_users (tenant_id, user_id, role_id) VALUES ($1, $2, $3)`, tenantAID, userID, roleID); err != nil {
			t.Fatalf("inserting membership: %v", err)
		}

		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Fatalf("deleting user: %v", err)
		}

		var profileExists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM profiles WHERE id = $1)`, profileID).Scan(&profileExists); err != nil {
			t.Fatalf("checking profile existence: %v", err)
		}
		if profileExists {
			t.Error("expected profile to be cascade-deleted with its user")
		}

		var membershipExists bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM tenant_users WHERE user_id = $1)`, userID).Scan(&membershipExists); err != nil {
			t.Fatalf("checking membership existence: %v", err)
		}
		if membershipExists {
			t.Error("expected tenant_users row to be cascade-deleted with its user")
		}
	})
}

func TestMigrate_RolePermissions(t *testing.T) {
	db := openTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	var permissionID string
	if err := db.QueryRow(`SELECT id FROM permissions LIMIT 1`).Scan(&permissionID); err != nil {
		t.Fatalf("querying a seeded permission: %v", err)
	}

	var roleID string
	if err := db.QueryRow(`SELECT id FROM roles WHERE name = 'admin'`).Scan(&roleID); err != nil {
		t.Fatalf("querying seeded admin role: %v", err)
	}

	t.Run("role_permissions with non-existent role_id fails FK constraint", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO role_permissions (role_id, permission_id) VALUES (gen_random_uuid(), $1)`, permissionID)
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})

	t.Run("role_permissions with non-existent permission_id fails FK constraint", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, gen_random_uuid())`, roleID)
		if err == nil {
			t.Fatal("expected FK violation error, got nil")
		}
	})

	t.Run("roles.permissions column no longer exists", func(t *testing.T) {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'roles' AND column_name = 'permissions')`).Scan(&exists)
		if err != nil {
			t.Fatalf("checking roles.permissions column: %v", err)
		}
		if exists {
			t.Error("expected roles.permissions to have been dropped (FEAT-012)")
		}
	})
}
