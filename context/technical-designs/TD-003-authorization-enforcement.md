# TD-003: Authorization Enforcement (Middleware)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-19

Feature Spec: [FEAT-003 - Authorization Enforcement (Middleware)](../features/FEAT-003-authorization-enforcement.md)

---

# 1. Overview

## Summary

Composable `net/http` middleware (`RequirePermission`, `RequireRole`) that reads `*auth.CustomClaims` from request context (populated by TD-001) and enforces the permission matrix defined by TD-002's role data, denying with 403 on mismatch and logging every decision.

## Goals

- Composable, per-route middleware for permission and role checks
- Fail closed on missing/invalid claims (401, distinct from 403 permission denial)
- Structured audit logging of every authorization decision

## Non-Goals

- Defining roles/permissions (TD-002)
- Routing decisions (TD-004)

---

# 2. Architecture

## High-Level Design

```
Request (claims in context, from TD-001)
│
▼
RequirePermission("billing:read") / RequireRole("admin")
  │ 1. Extract claims from context
  │ 2. claims present? No → 401
  │ 3. claims.HasPermission(p) / role match? No → 403 + log
  │ 4. Yes → next.ServeHTTP
▼
Routing (TD-004)
```

---

# 3. Components

## New Components

- `middleware.RequirePermission(permission string) func(http.Handler) http.Handler`
- `middleware.RequireRole(roles ...string) func(http.Handler) http.Handler`
- `audit.LogAuthzDecision(ctx, allowed bool, permission string)` helper

## Modified Components

- Router configuration: each protected route annotated with required permission(s)/role(s)

---

# 4. Data Model

## New Tables

None (reads from TD-002's `roles`/`permissions`, but decisions are made from JWT claims directly, not a DB call).

## Schema Changes

None.

---

# 5. API Design

## New Endpoints

None — middleware only.

## Endpoint Changes

All protected routes gain a 403 response case:

```json
403 { "error": "forbidden", "message": "insufficient permissions" }
```

or

```json
403 { "error": "forbidden", "message": "insufficient role" }
```

---

# 6. Sequence Flow

```
Request (claims present)
│
RequirePermission middleware
│
claims nil? ──Yes──> 401
│ No
HasPermission(required)? ──No──> 403 + audit log (deny)
│ Yes
audit log (allow, optional/sampled)
│
next handler
```

---

# 7. Security

## Authentication

Depends on TD-001 having already populated context; middleware ordering enforced at router setup (fails closed if misconfigured — nil claims → 401).

## Authorization

Core responsibility of this component — enforces exact permission matrix from TD-002.

## Data Protection

N/A.

## Rate Limiting

N/A for this component (runs before or after rate limiting depending on route config — TODO: confirm exact ordering relative to TD-005, not specified in overview; recommended: authz before rate limiting so denied requests don't consume tenant quota).

---

# 8. Performance

## Expected Load

Every protected request passes through this middleware.

## Database Impact

None — pure in-memory claims check, no DB round-trip.

## Caching Strategy

N/A — permissions already flattened into JWT claims at issuance (per ADR-001), avoiding any lookup here.

---

# 9. Monitoring

## Metrics

- `authz_decisions_total{result="allow|deny",permission}`

## Logging

- Every denial logged with correlation ID, tenant_id, user_id, required permission/role, actual role

## Alerts

- Spike in denials for a specific tenant/user (possible privilege escalation attempt)

---

# 10. Risks

## Risk 1

Middleware misconfiguration (route missing a `RequirePermission` call) silently exposes an endpoint.

Mitigation: route configuration table (TD-004) requires explicit permission/role declaration per route, defaulting to deny-all if unspecified.

---

## Risk 2

Stale permissions in a long-lived JWT after a role change (permissions flattened at issuance, not re-checked against DB).

Mitigation: short-lived tokens (5-15 min per TD-001) bound the staleness window; critical role changes can use the JWT blacklist as an emergency revocation path.

---

# 11. Rollout Plan

## Deployment

1. Deploy middleware alongside TD-001 (both foundational, deployed together)
2. Wire route configuration with required permissions per endpoint
3. Verify permission matrix via integration tests covering all three roles

## Rollback

1. Revert route configuration
2. Roll back deployment

---

# 12. Open Questions

- Exact middleware ordering relative to rate limiting (TD-005) — authz-before-rate-limit recommended but not confirmed in overview

---

# 13. ADR References

- ADR-001: CBAC with RBAC for Authentication and Authorization
