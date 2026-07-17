# Current Feature

FEAT-000: Core Identity Data Model (Users, Profiles & Tenants)

## File

@/context/features/FEAT-000-core-identity-data-model.md

## Goals

- [x] FR-1: Persist a `tenants` table storing organization-level configuration
- [x] FR-2: Persist a `users` table scoped to a tenant, referencing a role, with unique email per tenant
- [x] FR-3: Persist a `profiles` table as a one-to-one extension of `users`
- [x] FR-4: Expose Go data models (`Tenant`, `User`, `Profile`) with JSON/db/validation tags matching the schema

## Notes

Foundational schema for tenants, users, and profiles via Goose migrations plus matching Go
models. `users.role_id` requires a `roles` table (owned by FEAT-002, still Draft) — per user
decision, a minimal `roles` table (schema + seed data from project-overview.md §11) is being
created as part of this feature so the FK constraint can be satisfied. No HTTP endpoints,
no RBAC/authz logic — schema/model foundation only.
