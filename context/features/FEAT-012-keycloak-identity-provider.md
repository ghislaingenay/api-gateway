# FEAT-012: Keycloak as Identity Provider

Status: Doing

Owner: Ghislain Genay
Created: 2026-08-02
Last Updated: 2026-08-04

Technical Design: [TD-012 - Keycloak as Identity Provider](../technical-designs/TD-012-keycloak-identity-provider.md)

---

> Note: "tenant memberships" is just the doc's shorthand for "the set of tenant_users rows belonging to a given user"

# 1. Overview

## Summary

Replace the gateway's self-hosted login/refresh/logout and credential storage
(password hashes, refresh tokens) with Keycloak as an external OIDC identity
provider, run via docker-compose. The gateway keeps verifying tokens — it
already fetches signing keys from a remote JWKS endpoint (FEAT-011) — and
simply repoints that JWKS trust root at Keycloak instead of the local mock
publisher, adding issuer validation. The gateway stops issuing tokens
entirely and becomes the tenant/RBAC authority: tenant context is resolved
server-side, per request, from a client-supplied `X-Tenant-ID` header
verified against a normalized tenant membership table, instead of
being embedded in the JWT. `role`/`permissions` are resolved the same way,
from `tenant_users` → `roles` → `role_permissions`, not from JWT claims.

> Note: `users` contains global details. `tenant_users` are scoped to a tenant and are not the same as global users. A user can be a member of multiple tenants, and each tenant user has its own role/permissions.

## Problem

The gateway re-implements security-sensitive identity machinery — password
hashing, refresh-token issuance/rotation, brute-force login guarding — that a
dedicated identity provider does better and that the gateway shouldn't need
to own. Separately, `role`/`permissions` are baked into every access token
at issuance time, so a role change made by an admin has no effect until the
affected user's token expires and they log in again; and `roles.permissions`
is a denormalized JSONB snapshot with no foreign-key integrity against the
`permissions` table it duplicates.

## Goals

- Keycloak (docker-compose) issues and signs identity tokens (`sub`, `email`)
  for the frontend; the gateway is never involved in login, and never sees a
  password
- Gateway's existing JWKS-backed key store (FEAT-011) is repointed at
  Keycloak's realm JWKS endpoint, with issuer (`iss`) validation added
- `GET /auth/me` is the only gateway endpoint left under `/auth/*`. Should fetch details from profile and email information
- Tenant context is resolved and verified server-side on every request from
  a client-supplied `X-Tenant-ID` header against a `tenant_users` membership
  table, cached to keep it off the database on the hot path
- `role`/`permissions` are resolved the same way — from `tenant_users` →
  `roles` → a new `role_permissions` join table — never from JWT claims, so
  a role change takes effect within one cache TTL instead of requiring token
  re-issuance
- A brand-new authenticated user can create a tenant and become its owner
  (self-service onboarding) without any gateway-to-Keycloak write-back

## Non-Goals

- Login, registration, refresh, or logout endpoints on the gateway —
  Keycloak's own endpoints are called directly by the frontend; the gateway
  is not involved in that flow at all
- Inviting additional members into an already-existing tenant — this
  feature only covers the first user of a tenant (its owner); adding more
  members later is left to a future feature
- Read replicas or any other database read-scaling mechanism — explicitly
  deferred; this feature uses caching (in-process + Redis) instead to keep
  per-request database load near zero
- Any Keycloak Admin API integration (attribute write-back, token exchange,
  impersonation) — not needed, because tenant identity is never stored as a
  Keycloak user attribute or JWT claim in this design
- mTLS or service-to-service client certificate issuance — "Keycloak as
  certificate authority" in this feature means Keycloak's realm signing
  keypair is the JWT trust root (served via JWKS), nothing broader. mTLS for service authentication is out of scope and will be performed separately in the coming feature specs
- Changing how downstream services consume tenant identity — they still
  only ever see the gateway-set trusted `X-Gateway-Tenant-ID` header,
  unchanged from FEAT-004. Apply mTLS for service authentication.

---

# 2. Users

## Primary Users

- Frontend application, which now authenticates directly against Keycloak
  and never talks to the gateway for login/refresh/logout
- Platform operators, who manage users/roles/realm configuration in Keycloak
  instead of the gateway's own login surface
- Downstream microservices, unaffected — they still only trust the
  gateway-set `X-Gateway-Tenant-ID` header (FEAT-004)

## Stakeholders

- Engineering (gateway implementation, Keycloak realm configuration)
- Security (removing local credential storage, tightening tenant
  verification)

---

# 3. User Stories

### Story 1

As a frontend application

I want to authenticate directly against Keycloak

So that the gateway never receives or stores a user's password

### Story 2

As a platform operator

I want a role or permission change to take effect on the user's very next
request

So that I don't have to wait for their token to expire before a revoked
permission is actually revoked

### Story 3

As a new user with no organization yet

I want my first authenticated request to be able to create a tenant and make
me its owner

So that I can start using the product without a separate admin provisioning
step

### Story 4

As a user who belongs to more than one tenant

I want to switch which tenant I'm acting in without being logged out

So that switching context is instant and doesn't interrupt my session

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must verify Keycloak-issued JWTs via Keycloak's realm JWKS
endpoint and reject tokens from any other issuer or signed with a
disallowed algorithm.

#### Acceptance Criteria

- [x] `JWT_JWKS_URL` points at Keycloak's realm certs endpoint; the existing
      JWKS key store (FEAT-011) requires no interface changes
- [x] `JWT_ISSUER` is validated against the token's `iss` claim; a token
      from an unexpected issuer is rejected with 401
- [x] `alg=none` and any algorithm outside `JWT_ALLOWED_ALGORITHMS` are
      rejected with 401 (unchanged from FEAT-001)

---

### FR-2

The gateway must accept `X-Tenant-ID` as tenant-context input but must
never trust it without re-verifying, on every request, that the
authenticated caller belongs to that tenant.

#### Acceptance Criteria

- [x] A request with `X-Tenant-ID` for a tenant the caller has no
      `tenant_users` row for is rejected with 403
- [x] A route that requires tenant context with `X-Tenant-ID` absent is
      rejected with 400
- [x] A verified `(sub, tenant_id)` pair resolves to a `ResolvedIdentity`
      carrying the caller's role and permissions for that tenant

---

### FR-3

The gateway must JIT-provision a local `users` row the first time it sees a
request from a Keycloak `sub` it doesn't already know about, without
granting that user access to any tenant.

#### Acceptance Criteria

- [x] First request from a new `sub` creates a `users` row
      (`keycloak_sub`, `email`) with no `tenant_users` membership
- [x] A subsequent request from the same `sub` reuses the existing row
      rather than creating a duplicate

---

### FR-4

An authenticated user with zero tenant memberships must be able to create a
tenant via `POST /onboarding` and become its owner.

#### Acceptance Criteria

- [x] `POST /onboarding` creates a `tenants` row and a `tenant_users` row
      with the `owner` role for the calling user, in one transaction
- [x] The response includes the new `tenant_id` so the frontend can start
      sending it as `X-Tenant-ID` on subsequent requests
- [x] `POST /onboarding` does not require `X-Tenant-ID` (there's no tenant
      yet)

---

### FR-5

Role and permission resolution must come from `tenant_users` → `roles` →
`role_permissions`, cached to avoid a database round trip on every request,
never from JWT claims.

#### Acceptance Criteria

- [x] `roles.permissions` JSONB column is removed; permissions come from a
      normalized `role_permissions(role_id, permission_id)` join table
- [x] Membership/role resolution is cached (in-process + Redis) with a
      configurable TTL; cache miss falls back to PostgreSQL
- [x] A role change in `tenant_users` is visible to the affected user
      within one cache TTL, without requiring a new token

---

### FR-6

`GET /auth/me` must return the caller's identity, and either their tenant
memberships (no `X-Tenant-ID`) or their tenant-scoped role/profile
(`X-Tenant-ID` present and verified).

#### Acceptance Criteria

- [x] Without `X-Tenant-ID`: response includes `user_id`, `email`, and the
      list of tenants the caller belongs to with their role in each
- [x] With a verified `X-Tenant-ID`: response includes `tenant_id`, `role`,
      `permissions`, and profile fields (display name, timezone)
- [x] With an unverified `X-Tenant-ID` (caller isn't a member): 403

---

### FR-7

`POST /auth/login`, `POST /auth/refresh`, and `POST /auth/logout` must be
removed from the gateway, not merely disabled.

#### Acceptance Criteria

- [x] All three routes return 404 (no matching route), not 501/disabled
- [x] No password hashing, refresh-token storage, or login brute-force
      guarding code remains reachable in the gateway

---

## Business Rules

- Tenant identity is never taken from a JWT claim — it is always a
  client-supplied `X-Tenant-ID` header, re-verified against `tenant_users`
  on every single request. This explicitly supersedes ADR-003's
  claims-only rule; see TD-012 §7 for the rationale (onboarding's
  chicken-and-egg problem and tenant-switch UX), not a silent
  contradiction of it.
- Role/permission checks always resolve through the membership/role cache,
  never through a JWT claim (there is none to trust or distrust).
- Onboarding is the only tenant-creation path in this feature; a user may
  own/belong to more than one tenant, but inviting other users into an
  existing tenant is out of scope (Non-Goals).
- Keycloak is the sole source of credentials and the sole JWT signer; the
  gateway never issues, signs, or refreshes a token.

---

## Permissions

| Action                            | Any Authenticated Caller   | Tenant Member (verified)    |
| --------------------------------- | -------------------------- | --------------------------- |
| `GET /auth/me` (no tenant header) | ✅                         | ✅                          |
| `POST /onboarding`                | ✅ (if zero memberships\*) | N/A                         |
| `/api/*` proxy routes             | ❌ (needs verified tenant) | ✅ (subject to route perms) |

\* Onboarding is not restricted to zero memberships by default (a user may
own more than one org) — see TD-012 Open Questions.

---

## User Flow

1. Frontend redirects to Keycloak; user authenticates directly against
   Keycloak (gateway not involved)
2. Frontend receives a Keycloak-issued JWT (`sub`, `email`) and calls the
   gateway with `Authorization: Bearer <jwt>`
3. Gateway verifies the token's signature (JWKS), issuer, and algorithm
4. Gateway JIT-provisions a local `users` row if this `sub` is new
5. If the frontend has no tenant selected yet, it calls `GET /auth/me`
   (no `X-Tenant-ID`) to list the caller's tenant memberships, or
   `POST /onboarding` if the list is empty
6. Frontend sends `X-Tenant-ID: <tenant_id>` on subsequent requests; gateway
   verifies membership (cached) and resolves role/permissions
7. Gateway sets the trusted `X-Gateway-Tenant-ID` header and proxies to the
   downstream service (unchanged from FEAT-004)
8. To switch tenants, the frontend simply sends a different `X-Tenant-ID`
   on its next request — no new token, no logout

---

# 5. Edge Cases

- `X-Tenant-ID` for a tenant the caller isn't a member of → 403
- `X-Tenant-ID` present on a request whose token's `sub` has no local
  `users` row yet → JIT-create the `users` row first, then evaluate
  membership (will be "not a member" for a brand-new user, 403)
- Route requires tenant context but `X-Tenant-ID` is missing → 400
- Onboarding called by a user who already belongs to a tenant → allowed by
  default (creates a second tenant they also own); flagged as an open
  question in TD-012 in case a stricter one-tenant-per-user rule is wanted
- Keycloak's JWKS/issuer endpoint unreachable at gateway startup → fail
  fast, no fallback (same posture as FEAT-011)
- Cached membership/role goes stale mid-session after an admin changes a
  role → bounded by the cache TTL, the same trade-off the existing
  `rbac.RoleCache` and `tenant.StatusChecker` caches already accept

---

# 6. Dependencies

## Internal

- [[FEAT-001]] JWT Authentication (CBAC) — JWKS/middleware skeleton is
  reused; `CustomClaims` is trimmed, not replaced
- [[FEAT-002]] RBAC Data Model — restructured (`tenant_users` +
  `role_permissions` replace `users.role_id` and `roles.permissions`)
- [[FEAT-003]] Authorization Enforcement — `RequirePermission`/
  `RequireRole` re-pointed at resolved identity instead of JWT claims
- [[FEAT-004]] Multi-Tenant Isolation & Routing — ADR-003's claims-only
  tenant rule is explicitly superseded (see TD-012 §7); the trusted
  `X-Gateway-Tenant-ID` downstream header is unchanged
- [[FEAT-011]] JWKS Key Rotation — key store is reused almost verbatim,
  just repointed at Keycloak

## External

- Keycloak (docker-compose, own PostgreSQL) — no new Go dependency;
  existing `golang-jwt/jwt/v5` and `MicahParks/jwkset`/`keyfunc` already
  cover JWKS-based verification, and the gateway never calls Keycloak's
  token endpoint itself

## Prerequisites

- FEAT-001, FEAT-002, FEAT-003, FEAT-004, FEAT-011 implemented

---

# 7. Success Criteria

## Business Metrics

- Zero passwords or refresh tokens stored in the gateway's database
- A role/permission change is reflected within one cache TTL instead of
  requiring the affected user's token to expire

## Technical Metrics

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/auth/... ./internal/identity/... ./internal/rbac/...
./internal/user/... ./internal/onboarding/...` passes
- Steady-state per-request database load for tenant/role resolution stays
  near zero (served from cache), matching the existing
  `tenant.StatusChecker`/`rbac.RoleCache` cache-hit-rate expectations

---

# 8. Related Documents

- Technical Design: TD-012
- FEAT-001: JWT Authentication (CBAC)
- FEAT-002: RBAC Data Model
- FEAT-003: Authorization Enforcement
- FEAT-004: Multi-Tenant Isolation & Routing
- FEAT-011: JWKS Key Rotation
- ADR-003: Extract Tenant ID from JWT Claims Only (superseded by this
  feature — see TD-012 §7 for rationale)
