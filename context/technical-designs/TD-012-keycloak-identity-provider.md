# TD-012: Keycloak as Identity Provider

Status: Doing

Owner: Ghislain Genay
Created: 2026-08-02
Last Updated: 2026-08-04

Feature Spec: [FEAT-012 - Keycloak as Identity Provider](../features/FEAT-012-keycloak-identity-provider.md)

---

# 1. Overview

## Summary

Keycloak (docker-compose, own PostgreSQL) becomes the gateway's identity
provider: it owns credentials and issues signed identity tokens carrying
only `sub` and `email`. The gateway's existing JWKS-backed `KeyStore`
(TD-011) is repointed at Keycloak's realm JWKS endpoint with issuer
validation added — no interface changes there. Everything downstream of
authentication changes: tenant identity is no longer a JWT claim. A new
`internal/identity` package resolves, per request, whether the caller
(identified by `sub`) is a verified member of the tenant named in the
client-supplied `X-Tenant-ID` header, and if so, their role and permissions
— sourced from a normalized `tenant_users`/`role_permissions` data model and
cached (in-process + Redis) to keep this off the database on the hot path.
A new `internal/onboarding` package lets a user with no tenant memberships
create one and become its owner. Login/refresh/logout are deleted from the
gateway entirely.

## Goals

- `auth.JWTAuthMiddleware` keeps its current shape; only its configured
  `KeyStore` and an added issuer check change
- `CustomClaims` shrinks to what Keycloak actually issues:
  `jwt.RegisteredClaims` + `Email`
- A new `identity.ResolvedIdentity` (assembled from the token's `sub` +
  verified `X-Tenant-ID` + cached DB lookup) becomes the thing downstream
  middleware reads instead of `CustomClaims.TenantID`/`Role`/`RoleID`/
  `Permissions`
- `tenant_users` (per-tenant role assignment) and `role_permissions`
  (role → permission join) replace `users.tenant_id`/`users.role_id` and
  the `roles.permissions` JSONB column
- Onboarding creates a tenant and its owner in one transaction, entirely
  within the gateway's own database — no Keycloak Admin API call

## Non-Goals

- Any Keycloak Admin API integration (attribute write-back, token
  exchange, impersonation) — ruled out specifically because tenant
  identity never lives in Keycloak or in the JWT in this design
- Login/refresh/logout implementation of any kind in the gateway
- Inviting additional members into an existing tenant
- Read replicas — caching is the mitigation for the added per-request DB
  dependency, not read scaling
- mTLS/service-certificate issuance. We are handling user authentication and tenant membership verification via Keycloak, but service-to-service authentication is out of scope for this feature and will be handled separately in a future feature spec.

---

# 2. Architecture

## High-Level Design

> Frontend is not in scope for this feature. It is assumed to be a separate SPA that can handle Keycloak login and token storage, and it will send the JWT and X-Tenant-ID header to the gateway.

```
Frontend                         Keycloak (docker-compose, own Postgres)
  │ login (direct — gateway never involved)
  ▼
JWT { sub, email, iss, exp, nbf, iat }   (RS256, realm signing key)
  │
  │ Authorization: Bearer <jwt>
  │ X-Tenant-ID: <tenant_id>        (once the frontend has a tenant selected)
  ▼
Gateway
  auth.JWTAuthMiddleware                  (internal/auth/middleware.go — TD-001,
  │  KeyStore → Keycloak realm JWKS         unchanged shape, TD-011)
  │  + jwt.WithIssuer(cfg.Issuer)  (NEW)
  │  attaches trimmed CustomClaims{sub, email} to context
  ▼
identity.ResolveMiddleware  (NEW — internal/identity)
  │  JIT-provision `users` row by keycloak_sub if unseen
  │  X-Tenant-ID absent  → ResolvedIdentity{UserID, Email}  (identity only)
  │  X-Tenant-ID present → membership.Cache.Resolve(sub, tenant_id)
  │      cache hit/miss → tenant_users JOIN roles              (Postgres)
  │      role → rbac.RoleCache.GetRoleByID(role_id)             (existing, TD-002)
  │    → ResolvedIdentity{UserID, TenantID, RoleID, RoleName, Permissions}
  │    not a member → 403
  ▼
RequirePermission / RequireRole / gateway.NewHandler
  │  read identity.FromContext (not auth.ClaimsFromContext for
  │  tenant/role/permissions — Email/sub still come from claims)
  │  gateway.NewHandler sets trusted X-Gateway-Tenant-ID (unchanged, FEAT-004)
  ▼
ratelimit / cache middleware
  keyed by ResolvedIdentity.TenantID / ResolvedIdentity.UserID
  ▼
Downstream services (unchanged — only ever see X-Gateway-Tenant-ID)
```

---

# 3. Components

## New Components

- `internal/identity/model.go` — `ResolvedIdentity{UserID uuid.UUID, Email
string, TenantID *uuid.UUID, RoleID *uuid.UUID, RoleName string,
Permissions []string}`. `TenantID` is nil when no `X-Tenant-ID` was
  supplied/verified (e.g. before onboarding).
- `internal/identity/resolver.go` — `Resolver` interface:
  `EnsureUser(ctx, keycloakSub, email string) (uuid.UUID, error)` (JIT
  provisioning, upsert-by-`keycloak_sub`) and `ResolveMembership(ctx,
userID, tenantID uuid.UUID) (*Membership, error)` reading
  `tenant_users JOIN roles`.
- `internal/identity/cache.go` — Redis + in-process membership cache,
  keyed by `(keycloak_sub, tenant_id)`, following the exact pattern
  already established by `tenant.memoryStatusCache` (singleflight-
  collapsed loads, TTL-bounded, negative-caches non-membership so repeated
  403s don't repeatedly hit Postgres). TTL from `IDENTITY_CACHE_TTL`
  (config, default 15m).
- `internal/identity/middleware.go` — `identity.ResolveMiddleware`: runs
  immediately after `auth.JWTAuthMiddleware`; calls `EnsureUser`, then
  `ResolveMembership` if `X-Tenant-ID` is present, attaches
  `ResolvedIdentity` to context via `WithIdentity`/`FromContext`.
- `internal/identity/authorize.go` — `RequirePermission`/`RequireRole`
  (moved here from `internal/auth`, see Modified Components below for
  why): read `identity.FromContext` instead of `auth.ClaimsFromContext`.
- `internal/identity/handler.go`, `internal/identity/dto.go` —
  `MeHandler` for `GET /auth/me` (moved here from `internal/auth`, see
  Modified Components below for why) and its response DTOs
  (`MeWithoutTenantResponse`, `CurrentUserResponse`, etc).
- `internal/onboarding/handler.go` — `POST /onboarding`: reads the
  caller's `UserID` from `identity.FromContext`, creates a `tenants` row
  and a `tenant_users` row (`role = owner`) in a single DB transaction,
  returns `{tenant_id, role}`.
- `internal/onboarding/dto.go` — `OnboardRequest{OrganizationName string}`,
  `OnboardResponse{TenantID uuid.UUID, Role string}`.
- `cmd/dbflush/main.go` (dev-only) — drops and recreates the local
  database, reruns goose migrations, reruns `cmd/seed`. Exits with an
  error unless `APP_ENV=development`, matching the guard pattern already
  used for `ALLOW_AUTO_DB_MIGRATION`. Wired as `make db-reset`.
- `keycloak/realm-export.json` — realm config imported at container start
  via `--import-realm`: realm `api-gateway`, a public PKCE-enabled client
  named `client-frontend` for the frontend, a confidential client
  (`api-gateway-backend`) for audience validation, and seeded demo users
  whose `email` matches `cmd/seed`'s local demo tenant owners so local dev
  has working matched identities out of the box. No custom protocol
  mappers are needed — default `sub`/`email`/`email_verified` claims are
  sufficient.

## Modified Components

- `internal/auth/claims.go` — `CustomClaims` trimmed to
  `jwt.RegisteredClaims` + `Email`; `Validate()` no longer requires
  `tenant_id`/`user_id` (Keycloak's token doesn't carry them).
- `internal/auth/middleware.go` — add `jwt.WithIssuer(cfg.Issuer)` to the
  `jwt.ParseWithClaims` options alongside the existing
  `jwt.WithValidMethods(allowedAlgorithms)`.
- **Deviation from the original plan, forced by Go's import-cycle rule**:
  `identity.ResolveMiddleware` must read `auth.ClaimsFromContext` (it needs
  the token's `sub`/`email` to JIT-provision/resolve identity), so
  `internal/identity` necessarily imports `internal/auth`. That makes it
  impossible for anything in `internal/auth` to import
  `internal/identity` back — `auth → identity → auth` doesn't compile.
  Concretely this means `MeHandler`, its DTOs, and
  `RequirePermission`/`RequireRole` cannot stay in `internal/auth` once
  they read `identity.FromContext`; they move to `internal/identity`
  instead (`internal/identity/handler.go`, `dto.go`, `authorize.go`).
  `internal/auth` shrinks to just what `identity` depends on: JWT
  verification (`middleware.go`, `keystore.go`, `claims.go`,
  `context.go`, `errors.go`) — a one-way dependency, low-level →
  high-level, same direction the rest of the codebase already uses (e.g.
  `rbac`/`user`/`tenant` never import `gateway`).
- `internal/auth/handler.go`, `internal/auth/dto.go` — deleted (moved to
  `internal/identity/handler.go`/`dto.go`, see above). `LoginHandler`,
  `RefreshHandler`, `LogoutHandler`, `issueTokenPair`, `setRefreshCookie`/
  `clearRefreshCookie`/`readRefreshCookie` are gone entirely, not moved.
  The relocated `MeHandler` reads `identity.FromContext` and branches on
  whether `ResolvedIdentity.TenantID` is set (list tenant access vs.
  tenant-scoped profile).
- `internal/auth/authorize.go` — deleted (moved to
  `internal/identity/authorize.go`, see above).
  `RequirePermission`/`RequireRole` read `identity.FromContext` instead of
  `auth.ClaimsFromContext` for `TenantID`/`UserID`/`Role`/`Permissions`.
- `internal/auth/signer.go`, `internal/auth/password.go`,
  `internal/auth/refreshtoken.go` — deleted.
- `internal/gateway/handler.go` — `clientTenantHeaders` no longer strips
  `X-Tenant-ID` itself (it's now legitimate, server-verified input); it
  still strips/re-sets the internal `X-Gateway-Tenant-ID`. Reads
  `identity.FromContext` instead of `auth.ClaimsFromContext` for
  `TenantID`/`UserID`/`Permissions`.
- **New**: `internal/identity/middleware.go` also exports
  `identity.RequireTenant`, a small middleware that 400s when
  `ResolvedIdentity.TenantID` is nil. `ResolveMiddleware` itself can't
  enforce this — it's shared by `GET /auth/me`/`POST /onboarding`, which
  deliberately don't require a tenant — so the `/api/*` chain wires
  `RequireTenant` in right after it, ahead of rate-limit/cache/the gateway
  handler, giving those downstream layers a guaranteed non-nil
  `TenantID` (FEAT-012 FR-2's 400 acceptance criterion).
- `internal/cache/middleware.go`, `internal/ratelimit/middleware.go` —
  read `identity.FromContext` instead of `auth.ClaimsFromContext`.
  `ratelimit.RateLimitMiddleware` is also wired into non-tenant-scoped
  chains (`GET /roles`, `GET /permissions`), so it keys under `uuid.Nil`
  when `TenantID` is nil rather than rejecting — only routes behind
  `identity.RequireTenant` are guaranteed a real tenant ID.
  `cache.CacheMiddleware` is only ever wired into the tenant-scoped
  `/api/*` chain, so it treats a nil `TenantID` as unauthorized (401),
  matching the untouched-since-FEAT-001 auth-required posture of that
  chain.
- `internal/rbac/repository.go` — `loadRoles` no longer decodes
  `roles.permissions` JSONB; `loadPermissions`-style join against the new
  `role_permissions` table populates `Role.Permissions` instead. `Role`
  struct and `RoleCache` interface shape are unchanged, so every existing
  caller (`GetRoleByID`, `GET /roles`, `GET /permissions`) is unaffected —
  the same "narrow interface absorbs the change" approach TD-011 used for
  `KeyStore`.
- `internal/user/model.go` — `User{ID, KeycloakSub, Email, CreatedAt,
UpdatedAt}` only; `TenantID`, `RoleID`, `PasswordHash`, `IsActive`,
  `EmailVerified`, `LastLoginAt`, `DeletedAt` removed.
- `internal/user/repository.go` — `GetByEmail`/login-era methods removed;
  add `GetByKeycloakSub(ctx, sub string) (*User, error)` and
  `EnsureByKeycloakSub(ctx, sub, email string) (*User, error)` (upsert,
  used by `identity.Resolver.EnsureUser`).
- `internal/server/routes.go` — remove `POST /auth/login`,
  `/auth/refresh`, `/auth/logout` registrations and their
  `loginConcurrency`/`loginIPLimit` wiring; keep `GET /auth/me`; add
  `POST /onboarding`; insert `identity.ResolveMiddleware` into
  `requirePermission` and a new `requireIdentity` helper (replacing
  `requireAuth`, since `GET /auth/me`/`POST /onboarding` need resolved
  identity, not just validated claims), and into the `/api/` chain right
  after `auth.JWTAuthMiddleware`, followed by `identity.RequireTenant`
  (only `/api/*` requires a verified tenant; `requirePermission`/
  `requireIdentity` don't add it, since `GET /roles`/`GET /permissions`/
  `GET /auth/me`/`POST /onboarding` are deliberately tenant-optional).
  `corsMiddleware`'s `Access-Control-Allow-Headers` swaps the
  refresh-cookie-era `X-CSRF-Token` placeholder for `X-Tenant-ID`, now a
  real custom header the frontend sends cross-origin and that needs CORS
  preflight allowance.
- `config/jwt.go` — remove `SigningKID`/`SigningPrivateKey` fields and
  their env parsing; add `Issuer string` (`JWT_ISSUER`, required — no
  fallback, fail startup if unset, matching `JWKSURL`'s existing
  fail-fast posture).
- `config/cookie.go` — deleted (no refresh-token cookie to configure).
- Login-security config (`LoginSecurityConfig`, `LOGIN_*` env vars) —
  deleted along with `internal/loginguard`.
- `internal/refreshtoken/` — deleted package.
- `internal/loginguard/` — deleted package.
- `internal/ratelimit/ip_middleware.go` (`IPRateLimitMiddleware`,
  `KeyLimiter`), `internal/ratelimit/concurrency_middleware.go`
  (`ConcurrencyLimitMiddleware`) — deleted. Both existed solely to protect
  the now-deleted `POST /auth/login`/`POST /auth/refresh` (per-IP limiting
  and the concurrency cap sized around `LoginHandler`'s intentional
  failure delay); `RateLimitMiddleware`/`MultiWindowLimiter.AllowBoth`
  (tenant/user-keyed, used by `/api/*` and `GET /roles`/`GET /permissions`)
  are untouched.
- `internal/clientip/`, `config/clientip.go` (`ClientIPConfig`,
  `TRUSTED_PROXY_HOPS`) — deleted; client-IP resolution existed solely for
  login-failure-guard keying and the deleted IP rate limiter, with no
  other caller.
- `docker-compose.yml` — remove `jwks-service`; add `keycloak` (image
  `quay.io/keycloak/keycloak`, `start-dev --import-realm`, realm mounted
  from `keycloak/realm-export.json`) and a dedicated `keycloak-db`
  Postgres container (isolated from the gateway's own `postgres` service,
  keeping Keycloak's schema lifecycle independent). `gateway`'s
  `JWT_JWKS_URL` becomes
  `http://keycloak:8080/realms/api-gateway/protocol/openid-connect/certs`
  in `docker-compose.yml` (internal docker-compose network — only the
  gateway container ever calls this URL to fetch keys), same override
  pattern the existing `jwks-service`/`JWT_JWKS_URL` entry already uses.
  `.env.example`'s host-perspective default stays
  `http://localhost:8090/.../certs`. Keycloak is exposed on host port
  **8090**, not 8080 — 8080 is already the gateway's own host port, so
  mapping Keycloak there too would conflict. `keycloak`'s ports entry is
  `"8090:8080"`.
  Add `JWT_ISSUER=http://localhost:8090/realms/api-gateway` to
  `.env.example` with **no** docker-compose override — unlike
  `JWT_JWKS_URL`, this value is only ever string-compared against the
  token's `iss` claim, never dialed, and `iss` reflects how the frontend
  (outside this compose file, always via the host-mapped port) reached
  Keycloak, so the same value is correct whether the gateway validating it
  runs in-container or on the host. `keycloak` service sets
  `KC_HOSTNAME=http://localhost:8090` (a full URL, not just a bare
  hostname — required so the port is included) so `iss` is deterministic
  rather than derived from whichever Host header a given request happens
  to carry. Keycloak image pinned to `quay.io/keycloak/keycloak:26.7.0`.
- `cmd/mockjwks/` — deleted (superseded by the real Keycloak container).
- `.env.example` — remove `JWT_SIGNING_KID`/`JWT_SIGNING_PRIVATE_KEY`/
  `COOKIE_*`/`LOGIN_*`; add `JWT_ISSUER`, `IDENTITY_CACHE_TTL`.
- `cmd/seed/main.go` — rewritten to seed a tenant + demo `users` rows whose
  `keycloak_sub` matches `keycloak/realm-export.json`'s seeded demo users'
  fixed `id` fields, plus their `tenant_users` role assignments.
  `role_permissions` is no longer seed's concern — see the `role_permissions`
  entry under Data Model below, it's migration-seeded instead so
  production has a working role catalog without depending on `cmd/seed`.
- `context/technical-designs/TD-001-jwt-authentication.md` — Open
  Questions updated: token issuance moved to Keycloak, superseding the
  gateway-issuance model TD-001 originally assumed.
- `context/technical-designs/TD-004-multi-tenant-routing.md` — Open
  Questions updated: tenant identity is now a server-verified
  `X-Tenant-ID` header, not a JWT claim — see §7 below for why this
  supersedes ADR-003's original claims-only rule.

---

# 4. Data Model

## New Tables

- `role_permissions(role_id UUID REFERENCES roles(id), permission_id UUID
REFERENCES permissions(id), PRIMARY KEY (role_id, permission_id))` —
  replaces `roles.permissions` JSONB. The table-creation migration does
  not derive rows from the JSONB column it drops (no dynamic backfill
  query); a separate migration explicitly re-inserts each system role's
  grants (admin/manager/viewer, matching the JSONB values exactly) plus a
  new `owner` role (see below), so production has a working role catalog
  without depending on `cmd/seed`, which only ever runs in
  `APP_ENV=development`. `cmd/seed` still owns dev-only demo _tenants and
  users_, just not the role catalog itself.
- `owner` role (new system role, `is_system_role=true`, migration-seeded
  alongside `role_permissions`) — granted automatically to whoever creates
  a tenant via `POST /onboarding`; same permission set as `admin`, scoped
  to the tenant they created. `rbac.Role.Name`'s validate tag
  (`oneof=admin manager viewer`) gains `owner`.
- `tenant_users(tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
user_id UUID REFERENCES users(id) ON DELETE CASCADE, role_id UUID
REFERENCES roles(id), created_at, updated_at, PRIMARY KEY (tenant_id,
user_id))` — replaces `users.tenant_id`/`users.role_id`. No migration
  backfill (no pre-existing `tenant_users` data to derive); `cmd/seed`
  creates `tenant_users` rows directly for the demo users it seeds.

## Schema Changes

- `users`: drop `tenant_id`, `role_id`, `password_hash`, `is_active`,
  `email_verified`, `last_login_at`, `deleted_at`; add `keycloak_sub
VARCHAR(255) UNIQUE NOT NULL`. Unique constraint on `(tenant_id, email)`
  is dropped along with `tenant_id` — `email` is no longer
  tenant-scoped-unique (identity now lives in Keycloak; a `users` row is
  one person, independent of any tenant).
- `roles`: drop `permissions` JSONB column (superseded by
  `role_permissions`); `id`, `name`, `display_name`, `description`,
  `is_system_role` unchanged.
- `refresh_tokens`: dropped entirely.
- `profiles`: unchanged — still `user_id` FK, `first_name`, `last_name`,
  `avatar_url`, `timezone`, `metadata`. No duplicated fields added to
  `users`; the split stays exactly as it is today.

## Redis Keys

- `identity:membership:{keycloak_sub}:{tenant_id}` (new) — cached
  `Membership{UserID, RoleID}`, TTL = `IDENTITY_CACHE_TTL` (default 15m,
  matching a typical Keycloak access-token lifetime; confirm exact value
  at implementation time, see Open Questions).
- `rbac:roles` / `rbac:permissions` (existing, unchanged key names and
  TTL — `RoleCacheTTL` — only the Postgres query behind a cache miss
  changes, from JSONB decode to a `role_permissions` join).

---

# 5. API Design

## New Endpoints

### POST /onboarding

Purpose: let an authenticated user with no tenant create one and become
its owner.
Auth: `Authorization: Bearer <jwt>` required; `X-Tenant-ID` not required.
Request: `{"organization_name": "Redicreate"}`
Response: `201 {"tenant_id": "...", "role": "owner"}`
Errors: `400` invalid/missing `organization_name`; `401` invalid/missing
token.

## Endpoint Changes

### GET /auth/me

Without `X-Tenant-ID`: `200 {"user_id": "...", "email": "...", "tenants":
[{"tenant_id": "...", "name": "...", "role": "..."}]}`.
With a verified `X-Tenant-ID`: `200 {"user_id": "...", "email": "...",
"tenant_id": "...", "role": "...", "permissions": [...], "profile":
{"display_name": "...", "timezone": "..."}}` (profile fields sourced from
the unchanged `profiles` table).
With an unverified `X-Tenant-ID` (caller not a member): `403`.

### /api/\* proxy routes

Now require a verified `X-Tenant-ID` for any route needing tenant context
(`400` if absent, `403` if present but the caller isn't a `tenant_users`
member). Permission checks (`route.PermissionsRequired`) now read
`identity.FromContext().Permissions` instead of
`claims.Permissions`.

## Removed Endpoints

`POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout` — return
`404`, matching the codebase's existing route-not-found behavior rather
than a bespoke "endpoint disabled" response.

---

# 6. Sequence Flow

```
Request arrives with Authorization: Bearer <jwt> [+ X-Tenant-ID]
│
auth.JWTAuthMiddleware
│  verify signature via KeyStore.GetKey(kid)  (Keycloak JWKS, TD-011 store reused)
│  verify alg ∈ JWT_ALLOWED_ALGORITHMS
│  verify iss == JWT_ISSUER                                    (NEW)
│  invalid? ──Yes──> 401
│  No
▼
identity.ResolveMiddleware
│  EnsureUser(sub, email)              — JIT-create `users` row if new
│  X-Tenant-ID present?
│    No  ──> ResolvedIdentity{UserID, Email}, TenantID=nil, attach, continue
│    Yes ──> ResolveMembership(user_id, tenant_id)
│              cache hit? ──Yes──> use cached {RoleID}
│              No ──> query tenant_users JOIN roles; populate cache (incl. negative)
│            not a member? ──Yes──> 403
│            No  ──> rbac.RoleCache.GetRoleByID(role_id) → RoleName, Permissions
│                     ResolvedIdentity{UserID, TenantID, RoleID, RoleName,
│                     Permissions}, attach, continue
▼
Route-specific middleware (RequirePermission / RequireRole / route
PermissionsRequired check in gateway.NewHandler) read identity.FromContext
│  insufficient? ──Yes──> 403
│  No
▼
ratelimit.RateLimitMiddleware / cache.CacheMiddleware
│  keyed by ResolvedIdentity.TenantID / UserID  (unchanged mechanics, FEAT-005/006)
▼
gateway.NewHandler
│  tenant.StatusChecker.IsActive(TenantID)  (unchanged, FEAT-004)
│  strip client X-Gateway-Tenant-ID, set trusted one from ResolvedIdentity.TenantID
▼
Proxied to downstream service
```

---

# 7. Security

## Authentication

Keycloak-issued RS256 JWTs verified via JWKS (TD-011's key store,
repointed) plus an explicit issuer check (new) and the existing algorithm
allowlist (TD-001) — this extends, rather than weakens, the
alg-confusion protections already in place.

## Authorization

**This is the deliberate deviation from ADR-003** ("Extract Tenant ID
from JWT Claims Only"). ADR-003 was written when the gateway issued its
own tokens and could embed `tenant_id` at issuance time; Keycloak cannot
do that, because a brand-new user has no tenant yet at first login
(onboarding creates the tenant _after_ authentication), and re-issuing a
token on every tenant switch would require either the gateway becoming a
second token signer or wiring Keycloak token-exchange — both rejected
during design (see FEAT-012's Non-Goals).

Instead: tenant identity is a client-supplied `X-Tenant-ID` header, but it
is **never trusted directly**. Every single request re-verifies, via
`identity.ResolveMiddleware`, that the authenticated `sub` has a
`tenant_users` row for that `tenant_id` before any role/permission is
resolved or any downstream header is set. This is a "verify, don't trust"
pattern, not a return to the pre-ADR-003 "trust the header" vulnerability
it was written to close — the header only ever selects _which_ tenant
context to verify against, it never grants access on its own.
`TD-004-multi-tenant-routing.md`'s Open Questions section is updated to
point here.

## Data Protection

The gateway stores `keycloak_sub` only — no passwords, no refresh tokens.
Keycloak's own database holds credentials; its container should use a
dedicated volume/DB (`keycloak-db`, isolated from the gateway's
`postgres`).

## Rate Limiting

Unchanged (FEAT-005) — `ratelimit.RateLimitMiddleware` keys off
`ResolvedIdentity.TenantID`/`UserID` instead of `claims.TenantID`/
`UserID`; same Redis-backed distributed limiter.

---

# 8. Performance

## Expected Load

Every authenticated request now needs a tenant-membership lookup that
used to be free (embedded in the JWT). The `identity` package's two-tier
cache (in-process + Redis, `IDENTITY_CACHE_TTL`) is designed specifically
to keep this near-zero on the hot path, following the same trade-off
`tenant.memoryStatusCache` (30s TTL) and `rbac.RoleCache` (5m TTL) already
make in this codebase.

## Database Impact

- `EnsureUser` (JIT provisioning): one upsert per genuinely new `sub`,
  effectively zero afterward.
- `ResolveMembership`: one query per cache miss per `(sub, tenant_id)`
  pair, collapsed under concurrent load via singleflight (same pattern as
  `tenant.memoryStatusCache`).
- `role_permissions` join replaces a JSONB decode with a real join, but
  it's only read on `rbac.RoleCache` (re)load (every `RoleCacheTTL`, 5m),
  not per request — no new per-request DB cost from this change alone.

## Caching Strategy

Explicitly caching-over-read-replicas per the user's direction: read
replicas are a Non-Goal for this feature. `IDENTITY_CACHE_TTL` defaults to
15m (tunable via env var) to bound staleness after a role change while
keeping steady-state reads almost entirely in Redis/in-process memory.

---

# 9. Monitoring

## Metrics

- Reuses `jwt_validation_total{result}`/`jwt_validation_duration_seconds`
  (TD-001) — validation path itself is unchanged beyond the issuer check
- TODO: `identity_membership_cache_total{result="hit|miss|negative"}` —
  not specified yet, left to the implementer, mirrors the existing
  `rbac`/`tenant` cache instrumentation gap noted in prior TDs

## Logging

- 403s from a failed membership check log `sub`+`tenant_id` (not full
  claims) for audit, via the existing `audit.LogAuthzDecision` pattern
- JIT user-provisioning events logged at info level (first-sight of a new
  `sub` is a meaningful, low-volume event worth tracing)

## Alerts

- TODO: no specific alerting threshold defined yet for
  Keycloak-unreachable-at-startup or elevated membership-cache miss rate

---

# 10. Risks

## Risk 1

Removing `tenant_id`/`role`/`permissions` from the JWT is a breaking
contract change for any existing client code (including this repo's own
tests/fixtures) built against the old `CustomClaims` shape.

Mitigation: single-shot migration by design, matching the "no fallback"
precedent TD-011 already set when it removed `SigningKeys`; no dual-mode
compatibility shim.

---

## Risk 2

Trusting `X-Tenant-ID` as input at all — even though it's re-verified
every request — is a meaningful architecture reversal from ADR-003 and
could be mis-implemented as "trust the header" if the membership check is
ever accidentally skipped on a new route.

Mitigation: the verification lives in one shared middleware
(`identity.ResolveMiddleware`) that every authenticated route chain must
include, rather than being re-implemented per handler; `gateway.NewHandler`
and `RequirePermission`/`RequireRole` all read the already-verified
`ResolvedIdentity`, never the raw header, so there is exactly one place
the check can be missed, not many.

---

## Risk 3

`role_permissions` has no data of its own until something populates it;
if `cmd/seed`'s role→permission grants silently drift from what
`roles.permissions` used to encode, a role could end up with fewer
permissions than intended and nothing would surface that until an
authorization check unexpectedly fails.

Mitigation: `role_permissions` grants are inserted by an explicit,
reviewable migration (not derived dynamically from the JSONB it
replaces, and not left to `cmd/seed`, which never runs outside
`APP_ENV=development`), verified against `go test ./internal/rbac/...`
fixtures asserting each system role's resolved permission set matches
what `roles.permissions` used to define.

---

# 11. Rollout Plan

## Deployment

1. Add `keycloak`/`keycloak-db` services to `docker-compose.yml`; author
   `keycloak/realm-export.json`
2. Land the data-model migrations (`role_permissions` + explicit grant
   seed incl. new `owner` role, `tenant_users`, `users` column changes,
   drop `refresh_tokens`) — grants are explicit migration-time inserts,
   not a dynamic JSONB backfill
3. Implement `internal/identity` (resolver, cache, middleware) and
   `internal/onboarding`
4. Update `internal/auth`, `internal/rbac`, `internal/user`,
   `internal/gateway`, `internal/server/routes.go`, `config/jwt.go` per
   §3 Modified Components
5. Delete `internal/refreshtoken`, `internal/loginguard`,
   `internal/auth/{signer,password}.go`, `config/cookie.go`,
   `cmd/mockjwks`
6. Rewrite `cmd/seed` to match the new schema and Keycloak's seeded demo
   users; add `cmd/dbflush`
7. Verify `go build ./...`, `go vet ./...`, and the full test suite are
   green; run the gateway against `docker-compose up` end-to-end
   (Keycloak login → onboarding → `X-Tenant-ID`-scoped `/api/*` call)

## Rollback

Revert the commit(s). Because this removes the gateway's own token
issuance and credential storage entirely (not a parallel/fallback path),
rollback means reverting to the pre-FEAT-012 gateway image and
`docker-compose.yml` together — there is no partial/mixed-mode rollback,
consistent with the "no fallback" precedent from TD-011 and Risk 1 above.

---

# 12. Open Questions

- Can a user own/belong to more than one tenant? The data model
  (`tenant_users` as a proper many-to-many) supports it, and the default
  in this design is to allow it, but this should be explicitly confirmed
  before implementation — it affects whether `POST /onboarding` should
  reject a caller who already has ≥1 membership.
- Exact Keycloak image tag/version to pin in `docker-compose.yml`, and the
  realm name (`api-gateway` assumed here).
- `IDENTITY_CACHE_TTL` default (15m proposed, matching a typical Keycloak
  access-token lifetime) — confirm the actual value and env var name
  before implementation.
- Confirm invite-additional-member-into-existing-tenant is genuinely
  deferred to a future feature and not something needed for this
  feature's launch.

---

# 13. ADR References

- ADR-001: CBAC with RBAC for Authentication and Authorization
- ADR-003: Extract Tenant ID from JWT Claims Only — **superseded by this
  feature**; see §7 Security → Authorization above for the rationale
- ADR-004: Use Go for API Gateway Implementation
