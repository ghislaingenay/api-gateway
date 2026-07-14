# TD-007: Request Validation

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Feature Spec: [FEAT-007 - Request Validation](../features/FEAT-007-request-validation.md)

---

# 1. Overview

## Summary

Middleware that validates request bodies, path/query params, and headers against per-route schemas using Go struct validation tags, rejecting malformed requests with a consistent 400 error before they consume rate-limit/cache/downstream resources.

## Goals

- Struct-tag-based body validation consistent with the project's Go data models
- Required path/query parameter validation
- Consistent JSON error format across all validation failures

## Non-Goals

- Business-logic/state-dependent validation (uniqueness, referential integrity)

---

# 2. Architecture

## High-Level Design

```
Request (authenticated + authorized, TD-001/TD-003)
│
▼
ValidationMiddleware
  │ 1. Parse body (if route expects one) → 400 on malformed JSON
  │ 2. Validate struct tags (go-playground/validator) → 400 with field errors
  │ 3. Validate required path/query params → 400 if missing/type-mismatched
▼
RateLimit (TD-005) / Cache (TD-006) / Routing (TD-004)
```

---

# 3. Components

## New Components

- `middleware.ValidationMiddleware(schema RouteSchema) func(http.Handler) http.Handler`
- `validation.RouteSchema` — per-route definition of body type, required params
- Integration with `go-playground/validator` (or equivalent) using existing struct tags from project data models

## Modified Components

- `router.Route` config (TD-004) — extended with optional `BodySchema`, `RequiredParams` fields

---

# 4. Data Model

## New Tables

None.

## Schema Changes

None.

---

# 5. API Design

## New Endpoints

None — middleware only.

## Endpoint Changes

All routes with a defined schema gain a validation failure response:

```json
400 { "error": "validation_failed", "message": "request validation failed", "fields": [{"field": "email", "reason": "required"}] }
```

---

# 6. Sequence Flow

```
Request (authenticated, authorized)
│
Body expected? ──Yes──> Parse JSON ──Malformed──> 400
│ No / Valid JSON
Validate struct tags ──Fail──> 400 + field errors
│ Pass
Validate required path/query params ──Fail──> 400
│ Pass
Proceed to rate limiting / caching / routing
```

---

# 7. Security

## Authentication

Depends on TD-001 (runs after auth per business rule).

## Authorization

Depends on TD-003 (runs after authz, per FEAT-007's business rule to avoid validating unauthorized requests).

## Data Protection

Validation rejects malformed input before it reaches downstream services, reducing injection/malformed-payload attack surface.

## Rate Limiting

Runs before rate limiting (TD-005) so malformed requests don't consume tenant quota — TODO: confirm exact ordering, reasonable inference per FEAT-007.

---

# 8. Performance

## Expected Load

Every request with a defined schema is validated; overhead should be minimal (in-memory struct validation).

## Database Impact

None.

## Caching Strategy

N/A.

---

# 9. Monitoring

## Metrics

- `validation_failures_total{route,field}`

## Logging

- Validation failures logged with correlation ID, route, failed fields (no sensitive field values logged)

## Alerts

- Spike in validation failures for a specific route (possible client bug or attack probing)

---

# 10. Risks

## Risk 1

Oversized request bodies causing memory pressure.

Mitigation: enforce a max body size limit before JSON parsing (TODO: define exact limit, not specified in overview).

---

## Risk 2

Validation schema drifting from actual downstream service expectations.

Mitigation: schemas derived directly from shared Go data model structs, keeping gateway and downstream services in sync by construction where models are shared.

---

# 11. Rollout Plan

## Deployment

1. Deploy validation middleware with schemas for all routes with request bodies
2. Verify with integration tests covering valid/invalid payloads per route

## Rollback

1. Disable validation via feature flag (requests pass through unvalidated — downstream services must handle malformed input themselves temporarily)
2. Roll back deployment

---

# 12. Open Questions

- Exact validation error JSON schema
- Max request body size limit

---

# 13. ADR References

- ADR-004: Use Go for API Gateway Implementation
