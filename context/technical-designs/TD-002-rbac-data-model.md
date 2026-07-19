# TD-002: RBAC Data Model (Roles & Permissions)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-19

Feature Spec: [FEAT-002 - RBAC Data Model (Roles & Permissions)](../features/FEAT-002-rbac-data-model.md)

---

# 1. Overview

## Summary

PostgreSQL schema and seed data for `permissions`, managed via Goose migrations, plus read-only HTTP endpoints and an in-memory role-permission cache loaded at gateway startup.

> **Note (2026-07-17):** the `roles` table (schema + admin/manager/viewer seed data) was created by [[FEAT-000]] (`internal/database/migrations/00002_create_roles.sql`), since `users.role_id` needed a `roles` table to reference a FK to and FEAT-000 landed first. TD-002 no longer creates `roles` — it only creates `permissions` and reads the existing `roles` table for the `RoleStore`/endpoints below. Do not re-add a `CREATE TABLE roles` migration here.

## Goals

- Migration-managed, seeded `permissions` table matching the documented matrix (the `roles` table and its seed data already exist, created by [[FEAT-000]])
- In-memory cache for O(1) permission lookups at runtime

## Non-Goals

- Custom/tenant-defined roles
- Runtime role mutation via API

---

# 2. Architecture

## High-Level Design

```
Goose Migration (permissions only; roles table already exists from FEAT-000)
│
▼
PostgreSQL (roles [existing], permissions [new] tables)
│
▼ (loaded at startup)
RoleCache (interface; in-memory map[string]Role, constructor-injected — not a global)
│
▼ (injected into)
GET /roles, GET /permissions handlers
```

---

# 3. Components

## New Components

- Goose migration file: `NNN_create_permissions.sql` (permissions table + seed data only — `roles` already exists from FEAT-000)
- `models.Permission` (per project overview Go data models; `models.Role` already exists from FEAT-000)
- `auth.RoleCache` — interface exposing `GetRole(name string) (*Role, bool)` and `All() []Role`; unexported concrete impl loads roles/permissions from PostgreSQL into memory once, via `NewRoleCache(db database.Service) (RoleCache, error)` which returns the interface (same shape as `auth.KeyStore`/`NewKeyStore`, TD-001). No package-level/global instance — `main.go` constructs it once and passes it in.
- `handlers.RolesHandler(roles auth.RoleCache) http.HandlerFunc`, `handlers.PermissionsHandler(roles auth.RoleCache) http.HandlerFunc` — dependency injected via constructor parameter, not resolved from a global.

## Modified Components

- None. Depends on the `roles` table and `models.Role` created by [[FEAT-000]]; does not modify or re-create them.

---

# 4. Data Model

## Existing Tables (created by FEAT-000, not modified here)

### roles

| Column          | Type      |
| --------------- | --------- |
| id              | uuid (PK) |
| name            | varchar(50), unique |
| display_name    | varchar(100) |
| description     | text      |
| permissions     | jsonb     |
| is_system_role  | boolean   |
| created_at      | timestamptz |
| updated_at      | timestamptz |

Already created and seeded (admin/manager/viewer) by `internal/database/migrations/00002_create_roles.sql` in FEAT-000. Listed here for reference only.

## New Tables

### permissions

| Column       | Type      |
| ------------ | --------- |
| id           | uuid (PK) |
| name         | varchar(100), unique |
| resource     | varchar(50) |
| action       | varchar(50) |
| description  | text      |
| created_at   | timestamptz |

Indexes: `idx_roles_name`, `idx_permissions_resource`, `idx_permissions_name`.

Full DDL and seed data as specified in `project-overview.md` §11.

## Schema Changes

None (initial creation).

---

# 5. API Design

## New Endpoints

### GET /roles

Purpose: list all roles with permissions.

Request: `Authorization: Bearer <jwt>`, requires `roles:read` permission.

Response: `200 [{id, name, display_name, description, permissions[], is_system_role}]`

---

### GET /permissions

Purpose: list all available permissions, optionally filterable by resource.

Request: `Authorization: Bearer <jwt>`, requires `roles:read` permission. Query param `?resource=` (TODO: confirm filter support — not specified in overview).

Response: `200 [{id, name, resource, action, description}]`

---

## Endpoint Changes

None.

---

# 6. Sequence Flow

```
Startup
│
main.go builds RoleCache via NewRoleCache(db) → injects into handlers
│
Request → GET /roles
│
JWT Validation (TD-001)
│
Permission Check: roles:read (TD-003)
│
RolesHandler's injected RoleCache.All() → JSON response
```

---

# 7. Security

## Authentication

Handled upstream by TD-001.

## Authorization

`roles:read` permission required for both endpoints (enforced by TD-003 middleware).

## Data Protection

Roles/permissions are non-sensitive reference data; no special protection beyond standard authz.

## Rate Limiting

Standard per-tenant rate limiting applies (TD-005).

---

# 8. Performance

## Expected Load

Low-frequency reads (roles/permissions rarely change); heavy reuse via in-memory cache.

## Database Impact

Single query at startup; no per-request DB load for lookups (served from memory).

## Caching Strategy

Full in-memory cache of roles/permissions, refreshed on startup. TODO: define refresh strategy if roles change without a restart (not specified in overview — likely out of MVP scope since roles are system-defined and immutable).

---

# 9. Monitoring

## Metrics

- `role_cache_load_duration_seconds`
- `roles_endpoint_requests_total`

## Logging

- Startup log confirming roles/permissions loaded (count of each)

## Alerts

- Startup failure to load roles/permissions (should fail gateway startup — fail closed)

---

# 10. Risks

## Risk 1

Migration seed data drifting from the permission matrix documented in the project overview.

Mitigation: single source of truth in the migration file, referenced directly by both docs and tests.

---

## Risk 2

In-memory cache becomes stale if roles are ever modified without a restart.

Mitigation: MVP treats roles as immutable/system-defined, restart required for changes; documented as a known limitation.

---

# 11. Rollout Plan

## Deployment

1. Run Goose migration to create and seed the `permissions` table (`roles` already exists from FEAT-000)
2. Deploy gateway; `main.go` constructs `RoleCache` via `NewRoleCache(db)` at startup and injects it into the handlers
3. Verify `GET /roles` returns expected seed data

## Rollback

1. Roll back migration (Goose down)
2. Roll back gateway deployment

---

# 12. Open Questions

- Should `GET /permissions` support resource filtering in MVP?

---

# 13. ADR References

- ADR-001: CBAC with RBAC for Authentication and Authorization
