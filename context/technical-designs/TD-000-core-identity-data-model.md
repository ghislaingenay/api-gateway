# TD-000: Core Identity Data Model (Users, Profiles & Tenants)

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-15
Last Updated: 2026-07-15

Feature Spec: [FEAT-000 - Core Identity Data Model (Users, Profiles & Tenants)](../features/FEAT-000-core-identity-data-model.md)

---

# 1. Overview

## Summary

PostgreSQL schema for `tenants`, `users`, and `profiles`, managed via Goose migrations, plus the corresponding Go data models (`Tenant`, `User`, `Profile`) that other features build on top of. No HTTP endpoints are introduced by this feature; it is a schema/model foundation consumed by [[FEAT-001]], [[FEAT-002]], [[FEAT-003]], and [[FEAT-004]].

## Goals

- Migration-managed schema for `tenants`, `users`, `profiles` with correct FK/uniqueness constraints
- Go models matching the schema, safe for JSON serialization (no password hash leakage)

## Non-Goals

- CRUD HTTP endpoints for tenants/users/profiles
- Role/permission schema (see TD-002)
- Enforcement or authentication logic

---

# 2. Architecture

## High-Level Design

```
Goose Migration (ordered: tenants → roles/permissions dependency → users → profiles)
│
▼
PostgreSQL (tenants, users, profiles tables)
│
▼
Go models: models.Tenant, models.User, models.Profile
│
▼
Consumed by: TD-001 (JWT issuance), TD-003 (authz), TD-004 (routing)
```

---

# 3. Components

## New Components

- Goose migration files: `NNN_create_tenants.sql`, `NNN_create_users.sql`, `NNN_create_profiles.sql`
- `models.Tenant`, `models.User`, `models.Profile` (per project overview Go data models)

## Modified Components

- None (new feature; note `users.role_id` has a foreign-key dependency on the `roles` table from TD-002, so migration ordering must place TD-002's migration before this feature's `users` migration, or vice versa depending on final ordering chosen at implementation time)

---

# 4. Data Model

## New Tables

### tenants

| Column                 | Type      |
| ---------------------- | --------- |
| id                     | uuid (PK) |
| name                   | varchar(255) |
| slug                   | varchar(100), unique |
| tier                   | varchar(50) (free, professional, enterprise) |
| rate_limit_per_minute  | integer   |
| rate_limit_per_hour    | integer   |
| max_users              | integer   |
| features               | jsonb     |
| is_active              | boolean   |
| created_at             | timestamptz |
| updated_at             | timestamptz |
| deleted_at             | timestamptz (nullable) |

Indexes: `idx_tenants_slug`, `idx_tenants_is_active` (partial, `WHERE deleted_at IS NULL`).

### users

| Column         | Type      |
| -------------- | --------- |
| id             | uuid (PK) |
| tenant_id      | uuid (FK → tenants.id, ON DELETE CASCADE) |
| role_id        | uuid (FK → roles.id) |
| email          | varchar(255) |
| password_hash  | varchar(255) |
| is_active      | boolean   |
| email_verified | boolean   |
| last_login_at  | timestamptz (nullable) |
| created_at     | timestamptz |
| updated_at     | timestamptz |
| deleted_at     | timestamptz (nullable) |

Constraint: `unique_email_per_tenant UNIQUE (tenant_id, email)`.

Indexes: `idx_users_tenant_id`, `idx_users_role_id`, `idx_users_email`, `idx_users_is_active` (partial, `WHERE deleted_at IS NULL`).

### profiles

| Column      | Type      |
| ----------- | --------- |
| id          | uuid (PK) |
| user_id     | uuid (FK → users.id, ON DELETE CASCADE), unique |
| first_name  | varchar(100) |
| last_name   | varchar(100) |
| avatar_url  | text      |
| timezone    | varchar(50), default 'UTC' |
| metadata    | jsonb     |
| created_at  | timestamptz |
| updated_at  | timestamptz |

Indexes: `idx_profiles_user_id`.

Full DDL and seed data as specified in `project-overview.md` §11.

## Schema Changes

None (initial creation).

---

# 5. API Design

## New Endpoints

None. This feature is schema/model-only; endpoints for identity management are out of scope (see Non-Goals).

## Endpoint Changes

None.

---

# 6. Sequence Flow

```
Deploy
│
Goose migration: create tenants
│
Goose migration: create roles/permissions (TD-002, dependency)
│
Goose migration: create users (FK → tenants, roles)
│
Goose migration: create profiles (FK → users)
│
Downstream feature queries (TD-001 JWT issuance, TD-003 authz, TD-004 routing)
```

---

# 7. Security

## Authentication

Not applicable — no endpoints introduced. `password_hash` storage uses bcrypt/argon2 per TD-001's hashing choice; this feature only defines the column.

## Authorization

Not applicable — no endpoints introduced.

## Data Protection

- `password_hash` is tagged `json:"-"` on the Go model to prevent accidental serialization
- `email` and profile fields are tenant-scoped; no cross-tenant query should be possible without an explicit `tenant_id` filter

## Rate Limiting

Not applicable (no endpoints).

---

# 8. Performance

## Expected Load

Read-heavy from downstream features (user lookup by email+tenant on every authentication attempt); writes are low-frequency (user provisioning, profile updates).

## Database Impact

- `idx_users_email` and the `(tenant_id, email)` unique constraint keep authentication lookups indexed
- Soft-delete partial indexes (`WHERE deleted_at IS NULL`) keep active-record queries fast as tables grow

## Caching Strategy

None at this layer; caching of user/session data (if any) belongs to TD-001.

---

# 9. Monitoring

## Metrics

- `identity_migration_duration_seconds`

## Logging

- Startup/deploy log confirming migrations applied cleanly

## Alerts

- Migration failure during deploy (should block rollout)

---

# 10. Risks

## Risk 1

Migration ordering: `users.role_id` depends on TD-002's `roles` table existing first, creating a cross-feature migration dependency.

Mitigation: sequence migrations explicitly (roles/permissions before users) and document the required order in both TD-000 and TD-002.

---

## Risk 2

Cascading deletes (`ON DELETE CASCADE` from tenants → users → profiles) could cause unintended data loss if a tenant is hard-deleted.

Mitigation: application layer should default to soft delete (`deleted_at`) for tenants and users; hard delete/cascade is a last-resort admin operation, not a routine path.

---

# 11. Rollout Plan

## Deployment

1. Run Goose migrations in order: tenants → roles/permissions (TD-002) → users → profiles
2. Deploy gateway; downstream features (TD-001, TD-003, TD-004) can now resolve tenant/user/profile data
3. Verify referential integrity with a smoke-test insert across all three tables

## Rollback

1. Roll back migrations in reverse order (Goose down): profiles → users → roles/permissions → tenants
2. Roll back gateway deployment

---

# 12. Open Questions

- Should tenant/user hard-delete cascades be restricted at the DB level (e.g., `ON DELETE RESTRICT` with an explicit soft-delete-only application policy) instead of `ON DELETE CASCADE`? Not specified in the project overview — flagged for confirmation before implementation.
- Do we need a seed/bootstrap tenant + admin user for local development, and if so, does that belong here or in TD-010 (dev environment)?

---

# 13. ADR References

- ADR-001: CBAC with RBAC for Authentication and Authorization
