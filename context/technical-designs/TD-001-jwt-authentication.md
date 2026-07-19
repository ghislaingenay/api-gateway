# TD-001: JWT Authentication (CBAC)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-19

Feature Spec: [FEAT-001 - JWT Authentication (CBAC)](../features/FEAT-001-jwt-authentication.md)

---

# 1. Overview

## Summary

An `net/http` middleware that parses, validates, and extracts claims from JWTs using `golang-jwt/jwt/v5`, with an explicit algorithm allowlist and multi-key support for rotation. Populates `*auth.CustomClaims` into the request context for all downstream middleware.

## Goals

- Reject tokens with `alg=none` or any algorithm outside the allowlist
- Support key rotation via `kid` header lookup against an active key set
- Attach `CustomClaims` to `context.Context` for downstream consumption

## Non-Goals

- Token issuance/login endpoints
- Refresh token flow

---

# 2. Architecture

## High-Level Design

```
Client (Authorization: Bearer <jwt>)
│
▼
JWTAuthMiddleware
  │ 1. Parse header, extract kid + alg
  │ 2. Validate alg against allowlist
  │ 3. Look up public/secret key by kid
  │ 4. Verify signature + exp/nbf/iat
  │ 5. Unmarshal CustomClaims
  │ 6. context.WithValue(ctx, claimsKey, claims)
▼
Next middleware (Authorization, Routing, ...)
```

---

# 3. Components

## New Components

- `auth.CustomClaims` struct (jwt.RegisteredClaims + tenant_id, user_id, role, role_id, permissions, email)
- `middleware.JWTAuthMiddleware(keyStore KeyStore) func(http.Handler) http.Handler`
- `auth.KeyStore` interface: `GetKey(kid string) (interface{}, error)` backed by active signing keys config

## Modified Components

- None (foundational feature)

---

# 4. Data Model

## New Tables

None — JWT claims are stateless; no new PostgreSQL tables required.

## Schema Changes

None.

## Redis Keys

- `jwt:blacklist:{jti}` — optional token revocation blacklist, checked after signature validation (TODO: confirm if blacklist check is in MVP scope or post-MVP — mentioned in overview's Redis Keys list but not in Core Features)

---

# 5. API Design

## New Endpoints

None — this is middleware applied to all protected routes, not a standalone endpoint.

## Endpoint Changes

All protected routes gain a dependency on `Authorization: Bearer <jwt>` header; unauthenticated requests receive:

```json
401 { "error": "unauthorized", "message": "invalid or missing token" }
```

---

# 6. Sequence Flow

```
Request
│
Extract Authorization header
│
Parse JWT header (alg, kid)
│
alg in allowlist? ──No──> 401
│ Yes
kid known? ──No──> 401
│ Yes
Signature + exp/nbf/iat valid? ──No──> 401
│ Yes
Unmarshal CustomClaims
│
context.WithValue(claims)
│
Next handler
```

---

# 7. Security

## Authentication

Signature verification via `golang-jwt/jwt/v5` with `jwt.WithValidMethods([]string{"RS256"})` (or configured allowlist) passed explicitly to `jwt.ParseWithClaims` to prevent algorithm confusion attacks.

## Authorization

Not handled here — see TD-003.

## Data Protection

Signing keys stored outside source control (TODO: confirm key storage mechanism — env vars vs. secrets manager, not specified in overview).

## Rate Limiting

N/A for this component.

---

# 8. Performance

## Expected Load

Every request passes through this middleware; must be near-zero overhead.

## Database Impact

None — stateless validation, no DB calls in the hot path.

## Caching Strategy

Public keys/JWKS cached in memory at startup and on rotation events; no per-request cache lookups needed.

---

# 9. Monitoring

## Metrics

- `jwt_validation_total{result="success|failure"}`
- `jwt_validation_duration_seconds`

## Logging

- Structured log on every validation failure: reason (`alg_rejected`, `expired`, `unknown_kid`, `malformed`), correlation ID

## Alerts

- Spike in `alg_rejected` failures (possible attack attempt)

---

# 10. Risks

## Risk 1

Algorithm confusion attack (HMAC/RSA key confusion).

Mitigation: explicit `WithValidMethods` allowlist passed to the JWT library; never trust the `alg` header alone.

---

## Risk 2

Key rotation causing valid tokens to be rejected during rollover.

Mitigation: support multiple simultaneously active keys keyed by `kid`; grace period before retiring old keys.

---

# 11. Rollout Plan

## Deployment

1. Deploy key store configuration (initial signing key + kid)
2. Deploy JWT middleware wired into router before all protected routes
3. Verify with integration tests against valid/invalid/expired tokens

## Rollback

1. Disable middleware via feature flag (falls back to rejecting all requests — fail closed, not a silent bypass)
2. Roll back deployment

---

# 12. Open Questions

- ~~Is JWT blacklist (revocation) in MVP scope, or deferred entirely?~~
  Resolved 2026-07-19: out of scope for this pass. Not listed under FEAT-001's
  Functional Requirements/Acceptance Criteria, only mentioned as a TODO in the
  Redis Keys section; consistent with the Non-Goals excluding refresh tokens
  and full OAuth2.
- ~~What is the key storage mechanism (env var, file, KMS)?~~
  Resolved 2026-07-19: env-var/config-based. `config.LoadJWTConfig` reads
  `JWT_ALLOWED_ALGORITHMS` and `JWT_SIGNING_KEYS` (kid=base64-PEM pairs) from
  the environment; `auth.NewKeyStore` parses them into RSA public keys. No
  KMS/JWKS integration for MVP.

---

# 13. ADR References

- ADR-001: CBAC with RBAC for Authentication and Authorization
- ADR-004: Use Go for API Gateway Implementation
