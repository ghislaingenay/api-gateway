// Command seed inserts a tenant and a small set of demo users/tenant
// memberships for exercising the gateway locally against Keycloak. It
// refuses to run unless APP_ENV=development. Unlike the pre-FEAT-012
// version, it no longer creates credentials — Keycloak owns those; this
// seeds gateway-side users rows whose keycloak_sub matches the demo users
// defined in keycloak/realm-export.json, so a login against the local
// Keycloak resolves to a working tenant membership out of the box.
package main

import (
	"context"
	"os"

	"api-gateway/config"
	"api-gateway/internal/database"
	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

type seedUser struct {
	keycloakSub string
	email       string
	role        string
}

var seedTenant = struct {
	name string
	slug string
}{name: "Seed Tenant", slug: "seed-tenant"}

// keycloakSub values here must match the "id" fields of the demo users
// defined in keycloak/realm-export.json — that fixed id is what Keycloak
// issues as the token's sub claim.
var seedUsers = []seedUser{
	{keycloakSub: "11111111-1111-1111-1111-111111111111", email: "admin@seed.test", role: "owner"},
	{keycloakSub: "22222222-2222-2222-2222-222222222222", email: "viewer@seed.test", role: "viewer"},
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

	var tenantID uuid.UUID
	err := db.QueryRowContext(ctx, `
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

		var userID uuid.UUID
		err := db.QueryRowContext(ctx, `
			INSERT INTO users (keycloak_sub, email)
			VALUES ($1, $2)
			ON CONFLICT (keycloak_sub) DO UPDATE SET email = EXCLUDED.email
			RETURNING id
		`, su.keycloakSub, su.email).Scan(&userID)
		if err != nil {
			logger.Default().Error("seed: upsert user", "email", su.email, "error", err.Error())
			os.Exit(1)
		}

		_, err = db.ExecContext(ctx, `
			INSERT INTO tenant_users (tenant_id, user_id, role_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id
		`, tenantID, userID, roleID)
		if err != nil {
			logger.Default().Error("seed: upsert tenant_users", "email", su.email, "error", err.Error())
			os.Exit(1)
		}

		logger.Default().Info("seed: user ready", "email", su.email, "role", su.role, "keycloak_sub", su.keycloakSub, "tenant_slug", seedTenant.slug)
	}

	logger.Default().Info("seed: done")
}
