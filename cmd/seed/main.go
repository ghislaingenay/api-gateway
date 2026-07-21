// Command seed inserts a tenant and a small set of test users with known
// dev passwords, for exercising the auth endpoints locally. It refuses to
// run unless APP_ENV=development, since it writes known, weak credentials.
package main

import (
	"context"
	"log"

	"api-gateway/config"
	"api-gateway/internal/auth"
	"api-gateway/internal/database"

	"github.com/google/uuid"
)

const seedPassword = "password123"

type seedUser struct {
	email string
	role  string
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
	if !config.IsDevelopmentMode() {
		log.Fatal("seed: refusing to run outside APP_ENV=development")
	}

	dbService := database.New()
	defer dbService.Close()
	db := dbService.GetDB()
	ctx := context.Background()

	passwordHash, err := auth.HashPassword(seedPassword)
	if err != nil {
		log.Fatalf("seed: hash password: %v", err)
	}

	var tenantID uuid.UUID
	err = db.QueryRowContext(ctx, `
		INSERT INTO tenants (name, slug, tier)
		VALUES ($1, $2, 'free')
		ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
		RETURNING id
	`, seedTenant.name, seedTenant.slug).Scan(&tenantID)
	if err != nil {
		log.Fatalf("seed: upsert tenant: %v", err)
	}

	for _, su := range seedUsers {
		var roleID uuid.UUID
		if err := db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, su.role).Scan(&roleID); err != nil {
			log.Fatalf("seed: lookup role %q: %v", su.role, err)
		}

		_, err := db.ExecContext(ctx, `
			INSERT INTO users (tenant_id, role_id, email, password_hash, is_active, email_verified)
			VALUES ($1, $2, $3, $4, true, true)
			ON CONFLICT (tenant_id, email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		`, tenantID, roleID, su.email, passwordHash)
		if err != nil {
			log.Fatalf("seed: upsert user %q: %v", su.email, err)
		}
		log.Printf("seed: user ready email=%s role=%s password=%s tenant_slug=%s", su.email, su.role, seedPassword, seedTenant.slug)
	}

	log.Println("seed: done")
}
