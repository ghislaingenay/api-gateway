# FEAT-000: Core Identity Data Model (Users, Profiles & Tenants)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-15
Last Updated: 2026-07-17

Technical Design: [TD-000 - Core Identity Data Model (Users, Profiles & Tenants)](../technical-designs/TD-000-core-identity-data-model.md)

---

# 1. Overview

## Summary

The foundational persistent data model for identity in the gateway: `tenants`, `users`, and `profiles` tables in PostgreSQL. This feature owns the schema, migrations, and Go data models for tenant organizations, the users that belong to them, and each user's profile details. Every other identity-adjacent feature — RBAC ([[FEAT-002]]), authorization enforcement ([[FEAT-003]]), and multi-tenant routing ([[FEAT-004]]) — depends on these tables existing first.

## Problem

There is currently no canonical schema for the core entities the rest of the gateway assumes: a tenant an account belongs to, a user within that tenant, and the profile information tied to that user. Without this foundational model, JWT issuance, RBAC role assignment, and tenant-scoped routing have nothing to reference.

## Goals

- Store tenants as organizations with rate limit quotas, tier, and feature flags
- Store users scoped to a tenant, referencing their assigned role
- Store profile details (name, avatar, timezone) as a one-to-one extension of a user
- Enforce referential integrity between tenants, users, roles, and profiles
- Provide Go data models (`Tenant`, `User`, `Profile`) matching the schema for use by downstream features

## Non-Goals

- Role/permission schema itself (see [[FEAT-002]])
- Authorization enforcement of permissions (see [[FEAT-003]])
- JWT issuance/authentication flows (see [[FEAT-001]])
- User-facing CRUD endpoints for managing users/profiles/tenants (post-MVP, beyond seed/migration-level creation)
- Billing/subscription management beyond the `tier` field

---

# 2. Users

## Primary Users

- Gateway authentication and authorization middleware (reads user/tenant data)
- Other backend features that need to resolve a user's tenant, role, or profile

## Stakeholders

- Engineering
- Security/Compliance (tenant isolation correctness)

---

# 3. User Stories

### Story 1

As a system

I want tenants, users, and profiles defined in structured, related tables

So that authentication, authorization, and routing features have a consistent identity model to build on

### Story 2

As the JWT authentication feature

I want to look up a user by email within a tenant and load their role and profile

So that I can issue tokens with accurate claims

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The system must persist a `tenants` table storing organization-level configuration (name, slug, tier, rate limits, feature flags, soft-delete support).

#### Acceptance Criteria

- [x] Migration creates `tenants` table with defined indexes (`idx_tenants_slug`, `idx_tenants_is_active`)
- [x] `slug` is unique and used for tenant resolution
- [x] Soft delete via `deleted_at` is supported

---

### FR-2

The system must persist a `users` table scoped to a tenant, referencing a role, with unique email per tenant.

#### Acceptance Criteria

- [x] Migration creates `users` table with `tenant_id` and `role_id` foreign keys
- [x] `(tenant_id, email)` is unique via `unique_email_per_tenant`
- [x] Deleting a tenant cascades to delete its users
- [x] Migration creates indexes: `idx_users_tenant_id`, `idx_users_role_id`, `idx_users_email`, `idx_users_is_active`

---

### FR-3

The system must persist a `profiles` table as a one-to-one extension of `users`, storing name, avatar, and timezone.

#### Acceptance Criteria

- [x] Migration creates `profiles` table with a unique `user_id` foreign key
- [x] Deleting a user cascades to delete their profile
- [x] Migration creates `idx_profiles_user_id`

---

### FR-4

The system must expose Go data models (`Tenant`, `User`, `Profile`) with JSON/db/validation tags matching the schema, including relationship fields populated via JOIN.

#### Acceptance Criteria

- [x] `User.PasswordHash` is never serialized to JSON
- [x] `User` exposes `Tenant`, `Role`, `Profile` as optional populated relationships
- [x] Validation tags reject malformed emails, empty names, and out-of-range tiers

---

## Business Rules

- A user always belongs to exactly one tenant; cross-tenant user references are disallowed
- Email uniqueness is enforced per tenant, not globally (two tenants may each have a user with the same email)
- A user has at most one profile
- Tenants, users, and profiles all support soft delete except profiles, which are hard-deleted via cascade with their user

---

## User Flow

1. Migration runs at deploy time, creating `tenants`, `users`, and `profiles` tables in dependency order
2. A tenant is provisioned (via seed/admin process), then users are created within it referencing a seeded role ([[FEAT-002]])
3. Downstream features ([[FEAT-001]], [[FEAT-003]], [[FEAT-004]]) query these tables to authenticate, authorize, and route requests

---

# 5. Edge Cases

- Migration re-run should be idempotent and not fail on existing tables
- Creating a user with a `role_id` that doesn't exist (must fail via FK constraint)
- Creating two users with the same email in different tenants (must succeed)
- Creating two users with the same email in the same tenant (must fail)
- Deleting a tenant with active users (cascade behavior must be intentional, not accidental data loss in production — confirm soft-delete vs hard cascade semantics before enforcing in the application layer)

---

# 6. Dependencies

## Internal

- [[FEAT-002]] RBAC Data Model (users reference `roles.id`)

## External

- PostgreSQL 15+
- Goose (migrations)

## Prerequisites

- PostgreSQL database provisioned

---

# 7. Success Criteria

## Business Metrics

- Schema supports every downstream identity-dependent feature without later migration rework

## Technical Metrics

- Migration runs cleanly on an empty database
- All foreign key and uniqueness constraints verified by tests

---

# 8. Related Documents

- Technical Design: TD-000
- Project Overview: [Database Schema](../project-overview.md)
- [[FEAT-001]] JWT Authentication
- [[FEAT-002]] RBAC Data Model
- [[FEAT-003]] Authorization Enforcement
- [[FEAT-004]] Multi-Tenant Isolation & Routing
