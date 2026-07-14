# FEAT-003: Authorization Enforcement (Middleware)

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Technical Design: [TD-003 - Authorization Enforcement (Middleware)](../technical-designs/TD-003-authorization-enforcement.md)

---

# 1. Overview

## Summary

Runtime enforcement of role- and permission-based access control on every gateway request. Composable middleware (`RequirePermission`, `RequireRole`) inspects the validated JWT claims (from [[FEAT-001]]) against the role/permission data model ([[FEAT-002]]) and either allows the request to proceed or returns 403.

## Problem

Having roles and permissions defined in the database is not sufficient — each protected route must actually enforce them consistently. Without composable middleware, permission checks get duplicated or inconsistently applied across route handlers, creating security gaps.

## Goals

- Provide `RequirePermission(permission string)` middleware for granular route protection
- Provide `RequireRole(roles ...string)` middleware for coarse-grained route protection
- Enforce the documented permission matrix (Admin/Manager/Viewer) exactly
- Return consistent 403 JSON error responses on authorization failure
- Log all authorization decisions (allow/deny) with correlation IDs for audit

## Non-Goals

- Defining the roles/permissions themselves (see [[FEAT-002]])
- Attribute-based routing decisions beyond role/permission checks (see [[FEAT-004]])

---

# 2. Users

## Primary Users

- All authenticated API clients (subject to enforcement)
- Route handlers/downstream services (protected by this middleware)

## Stakeholders

- Engineering
- Security/Compliance

---

# 3. User Stories

### Story 1

As a Manager role user

I want to be blocked from billing endpoints

So that billing access remains restricted to Admins only, per policy

### Story 2

As a platform operator

I want every authorization denial logged with a correlation ID

So that I can audit access attempts for compliance

---

# 4. Product Requirements

## Functional Requirements

### FR-1

`RequirePermission` middleware must check `claims.HasPermission(permission)` and return 403 with a structured JSON error if the permission is absent.

#### Acceptance Criteria

- [ ] Request with required permission proceeds to next handler
- [ ] Request without required permission returns 403 with `{"error":"forbidden","message":"insufficient permissions"}`
- [ ] Denial is logged with correlation ID, tenant ID, user ID, and requested permission

---

### FR-2

`RequireRole` middleware must check `claims.Role` against an allowlist of roles and return 403 if no match.

#### Acceptance Criteria

- [ ] Request with matching role proceeds
- [ ] Request without matching role returns 403 with `{"error":"forbidden","message":"insufficient role"}`

---

### FR-3

Authorization middleware must run only after JWT validation ([[FEAT-001]]) has populated claims in the request context; if claims are absent, the request must fail closed (401, not 403).

#### Acceptance Criteria

- [ ] Missing claims in context results in 401, not a panic or 403
- [ ] Middleware order is enforced at router configuration time

---

## Business Rules

- Enforcement mirrors the permission matrix defined in FEAT-002 exactly (Admin full access, Manager no billing/no delete, Viewer read-only)
- Authorization must fail closed: any ambiguity (missing claims, unknown permission) results in denial, never implicit allow

---

## Permissions

| Action                          | Admin | Manager | Viewer |
| -------------------------------- | ----- | ------- | ------ |
| Access `RequirePermission` route | Per permission matrix | Per permission matrix | Per permission matrix |
| Access `RequireRole("admin")` route | ✅ | ❌ | ❌ |

---

## User Flow

1. Request arrives with validated claims already in context (post-FEAT-001)
2. Router applies `RequirePermission`/`RequireRole` middleware configured per route
3. Middleware checks claims against required permission/role
4. On success, request proceeds to routing ([[FEAT-004]]); on failure, 403 returned and denial logged

---

# 5. Edge Cases

- Claims present but `Role` field empty/invalid
- Permission required but claims' `Permissions` array is empty
- Route configured with both `RequireRole` and `RequirePermission` (order of evaluation)
- Concurrent requests with a just-revoked role (see JWT blacklist in FEAT-001)

---

# 6. Dependencies

## Internal

- [[FEAT-001]] JWT Authentication (must run first, populates claims)
- [[FEAT-002]] RBAC Data Model (defines the permissions being checked)
- [[FEAT-004]] Multi-Tenant Isolation & Routing (runs after this middleware)
- [[FEAT-008]] Observability & Health Checks (authorization denials are logged)

## External

- None

## Prerequisites

- FEAT-001 and FEAT-002 implemented

---

# 7. Success Criteria

## Business Metrics

- Zero permission-matrix violations found in security review

## Technical Metrics

- Authorization check latency < 1ms (in-memory claims check, no DB round-trip)
- 100% of denied requests logged with correlation ID

---

# 8. Related Documents

- Technical Design: TD-003
- ADR-001: CBAC with RBAC for Authentication and Authorization
- [RBAC vs ABAC vs PBAC: Access Control Models](../../../knowledge/02-Software%20Engineering/Security/rbac-abac-pbac-access-control.md)
