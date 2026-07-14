# TD-002: RBAC Data Model (Roles & Permissions)

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Feature Spec: [FEAT-002 - RBAC Data Model (Roles & Permissions)](../features/FEAT-002-rbac-data-model.md)

---

# 1. Overview

## Summary

PostgreSQL schema and seed data for `roles` and `permissions`, managed via Goose migrations, plus read-only HTTP endpoints and an in-memory role-permission cache loaded at gateway startup.

## Goals

- Migration-managed, seeded roles/permissions matching the documented matrix
- In-memory cache for O(1) permission lookups at runtime

## Non-Goals

- Custom/tenant-defined roles
- Runtime role mutation via API

---

# 2. Architecture

## High-Level Design

```
Goose Migration
│
▼
PostgreSQL (roles, permissions tables)
│
▼ (loaded at startup)
In-Memory RoleStore (map[string]Role)
│
▼
GET /roles, GET /permissions handlers
```

---

# 3. Components

## New Components

- Goose migration files: `NNN_create_roles_and_permissions.sql`
- `models.Role`, `models.Permission` (per project overview Go data models)
- `store.RoleStore` — loads roles/permissions into memory at startup, exposes `GetRole(name string) (*Role, bool)`
- `handlers.RolesHandler`, `handlers.PermissionsHandler`

## Modified Components

- None (new feature)

---

# 4. Data Model

## New Tables

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
Load roles + permissions from PostgreSQL into RoleStore
│
Request → GET /roles
│
JWT Validation (TD-001)
│
Permission Check: roles:read (TD-003)
│
RoleStore.All() → JSON response
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

- `role_store_load_duration_seconds`
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

1. Run Goose migration to create and seed tables
2. Deploy gateway; RoleStore loads at startup
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
