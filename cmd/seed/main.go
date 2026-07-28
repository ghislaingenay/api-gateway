// Command seed inserts a tenant and a small set of test users with known
// dev passwords, for exercising the auth endpoints locally. It refuses to
// run unless APP_ENV=development, since it writes known, weak credentials.
package main

import (
	"context"
	"os"

	"api-gateway/config"
	"api-gateway/internal/auth"
	"api-gateway/internal/database"
	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

const seedPassword = "password123"

type seedUser struct {
	email string
	role  string
}

type seedPermission struct {
	name        string
	resource    string
	action      string
	description string
}

var seedPermissions = []seedPermission{
	// User management
	{"users:create", "users", "create", "Create new users within tenant"},
	{"users:read", "users", "read", "View user information"},
	{"users:update", "users", "update", "Update user details and roles"},
	{"users:delete", "users", "delete", "Delete users from tenant"},

	// Tenant management
	{"tenants:create", "tenants", "create", "Create new tenants (super admin only)"},
	{"tenants:read", "tenants", "read", "View tenant information"},
	{"tenants:update", "tenants", "update", "Update tenant settings and configuration"},
	{"tenants:delete", "tenants", "delete", "Delete tenant (super admin only)"},

	// Billing
	{"billing:read", "billing", "read", "View billing information and invoices"},
	{"billing:update", "billing", "update", "Update payment methods and subscription"},
	{"billing:delete", "billing", "delete", "Cancel subscription and delete payment methods"},

	// Settings
	{"settings:read", "settings", "read", "View application settings"},
	{"settings:update", "settings", "update", "Update application settings and configuration"},

	// Roles
	{"roles:read", "roles", "read", "View available roles and permissions"},
	{"roles:assign", "roles", "assign", "Assign roles to users"},

	// Permissions
	{"permissions:read", "permissions", "read", "View available permissions"},

	// Audit logs
	{"audit_logs:read", "audit_logs", "read", "View audit logs and activity history"},

	// API Keys
	{"api_keys:create", "api_keys", "create", "Generate new API keys"},
	{"api_keys:read", "api_keys", "read", "View API keys"},
	{"api_keys:revoke", "api_keys", "revoke", "Revoke API keys"},

	// Orders
	{"orders:read", "orders", "read", "View orders"},
	{"orders:create", "orders", "create", "Create new orders"},
}

var seedTenant = struct {
	name string
	slug string
}{name: "Seed Tenant", slug: "seed-tenant"}

var seedUsers = []seedUser{
	{email: "admin@seed.test", role: "admin"},
	{email: "viewer@seed.test", role: "viewer"},
}

func main() {
	if !config.LoadAppConfig().IsDevelopmentMode() {
		logger.Default().Error("seed: refusing to run outside APP_ENV=development")
		os.Exit(1)
	}

	dbService := database.New(config.LoadDatabaseConfig())
	defer dbService.Close()
	db := dbService.GetDB()
	ctx := context.Background()

	passwordHash, err := auth.HashPassword(seedPassword)
	if err != nil {
		logger.Default().Error("seed: hash password", "error", err.Error())
		os.Exit(1)
	}

	for _, p := range seedPermissions {
		_, err := db.ExecContext(ctx, `
			INSERT INTO permissions (name, resource, action, description)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (name) DO UPDATE SET resource = EXCLUDED.resource, action = EXCLUDED.action, description = EXCLUDED.description
		`, p.name, p.resource, p.action, p.description)
		if err != nil {
			logger.Default().Error("seed: upsert permission", "name", p.name, "error", err.Error())
			os.Exit(1)
		}
	}
	logger.Default().Info("seed: permissions ready", "count", len(seedPermissions))

	var tenantID uuid.UUID
	err = db.QueryRowContext(ctx, `
		INSERT INTO tenants (name, slug, tier)
		VALUES ($1, $2, 'free')
		ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
		RETURNING id
	`, seedTenant.name, seedTenant.slug).Scan(&tenantID)
	if err != nil {
		logger.Default().Error("seed: upsert tenant", "error", err.Error())
		os.Exit(1)
	}

	for _, su := range seedUsers {
		var roleID uuid.UUID
		if err := db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, su.role).Scan(&roleID); err != nil {
			logger.Default().Error("seed: lookup role", "role", su.role, "error", err.Error())
			os.Exit(1)
		}

		_, err := db.ExecContext(ctx, `
			INSERT INTO users (tenant_id, role_id, email, password_hash, is_active, email_verified)
			VALUES ($1, $2, $3, $4, true, true)
			ON CONFLICT (tenant_id, email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		`, tenantID, roleID, su.email, passwordHash)
		if err != nil {
			logger.Default().Error("seed: upsert user", "email", su.email, "error", err.Error())
			os.Exit(1)
		}
		logger.Default().Info("seed: user ready", "email", su.email, "role", su.role, "password", seedPassword, "tenant_slug", seedTenant.slug)
	}

	logger.Default().Info("seed: done")
}
