# FEAT-002: RBAC Data Model (Roles & Permissions)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-19

Technical Design: [TD-002 - RBAC Data Model (Roles & Permissions)](../technical-designs/TD-002-rbac-data-model.md)

---

# 1. Overview

## Summary

The persistent data model backing role-based access control: `roles` and `permissions` tables in PostgreSQL, seeded with the three system roles (Admin, Manager, Viewer) and their granular `resource:action` permission sets. This feature owns the read APIs for roles/permissions — enforcement itself is handled by [[FEAT-003]].

> **Note (2026-07-17):** the `roles` table and its schema/seed data were created by [[FEAT-000]] (`internal/database/migrations/00002_create_roles.sql`), since `users.role_id` needed a `roles` table to reference and FEAT-000 landed first. This feature's migration now only needs to create and seed `permissions`; do not re-create `roles`.

## Problem

Authorization decisions need a canonical, queryable source of truth for what each role can do. Hardcoding permission checks throughout the codebase is error-prone and hard to audit; a structured, seeded schema keeps role definitions consistent and auditable.

## Goals

- Store roles as immutable, system-defined entities with a `permissions` JSONB array
- Store granular permissions in `resource:action` format with resource/action metadata
- Seed Admin, Manager, and Viewer roles per the defined permission matrix at migration time
- Expose read endpoints for roles and permissions (`roles:read`, `roles:assign`)

## Non-Goals

- Custom/tenant-defined roles beyond the three system roles (post-MVP)
- Runtime permission editing UI (excluded — admin dashboard is post-MVP)
- Enforcement of permissions on requests (see [[FEAT-003]])

---

# 2. Users

## Primary Users

- Gateway authorization middleware (reads role/permission data)
- Tenant admins assigning roles to users

## Stakeholders

- Engineering
- Security/Compliance (permission matrix accuracy)

---

# 3. User Stories

### Story 1

As a system

I want roles and their permissions defined in a structured, seeded table

So that authorization checks have a single, auditable source of truth

### Story 2

As a tenant admin

I want to view the list of available roles and their permissions

So that I can correctly assign roles to my team members

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The system must persist a `permissions` table per the schema in the project overview. The `roles` table (schema + admin/manager/viewer seed data, `is_system_role = true`) already exists, created by [[FEAT-000]].

#### Acceptance Criteria

- [x] Migration creates `permissions` table with defined indexes (`roles` already exists from FEAT-000 — do not re-create it)
- [x] Migration seeds all permissions listed in the permission matrix
- [ ] Attempting to delete a system role is rejected (deferred — no role-mutation surface exists yet; revisit once a delete/mutation endpoint is built)

---

### FR-2

The system must expose a `GET /roles` endpoint returning all roles with their permissions, gated by `roles:read` permission.

#### Acceptance Criteria

- [x] Returns 200 with role list for authorized callers
- [x] Returns 403 for callers lacking `roles:read`

---

### FR-3

The system must expose a `GET /permissions` endpoint returning all available permissions, gated by `roles:read` permission.

#### Acceptance Criteria

- [x] Returns 200 with permission list for authorized callers
- [ ] Permissions grouped or filterable by `resource` (TODO: confirm exact filtering/query params — not specified in overview)

---

## Business Rules

- Permission naming is always `resource:action` (e.g., `users:create`, `billing:read`)
- Permission hierarchy: Viewer permissions ⊂ Manager permissions ⊂ Admin permissions
- System roles (`is_system_role = true`) cannot be deleted or have their permission set mutated via API

---

## Permissions

| Action              | Admin | Manager | Viewer |
| -------------------- | ----- | ------- | ------ |
| `roles:read`        | ✅    | ✅      | ✅     |
| `roles:assign`      | ✅    | ✅      | ❌     |
| Create custom role  | ❌ (post-MVP) | ❌ | ❌ |

---

## User Flow

1. Migration runs at deploy time, creating and seeding `roles`/`permissions` tables
2. Gateway loads role-permission mappings into memory at startup for fast lookup
3. Admin/Manager queries `GET /roles` to view available roles before assigning one to a user

---

# 5. Edge Cases

- Migration re-run should be idempotent (seed data should not duplicate)
- Query for a non-existent role ID
- Permission list requested by a role with only `roles:read` but no other access

---

# 6. Dependencies

## Internal

- [[FEAT-000]] Core Identity Data Model (owns the `roles` table and `models.Role`; this feature only adds `permissions`)
- [[FEAT-001]] JWT Authentication (permissions are embedded in JWT claims at issuance)
- [[FEAT-003]] Authorization Enforcement (consumes this data at runtime)

## External

- PostgreSQL 15+
- Goose (migrations)

## Prerequisites

- PostgreSQL database provisioned

---

# 7. Success Criteria

## Business Metrics

- Permission matrix matches documented spec with zero discrepancies

## Technical Metrics

- Role/permission lookup from in-memory cache < 1ms
- Migration runs cleanly on empty and existing databases

---

# 8. Related Documents

- Technical Design: TD-002
- ADR-001: CBAC with RBAC for Authentication and Authorization
- [RBAC vs ABAC vs PBAC: Access Control Models](../../../knowledge/02-Software%20Engineering/Security/rbac-abac-pbac-access-control.md)
