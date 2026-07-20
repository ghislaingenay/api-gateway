# TD-004: Multi-Tenant Isolation & Routing

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-20

Feature Spec: [FEAT-004 - Multi-Tenant Isolation & Routing](../features/FEAT-004-multi-tenant-routing.md)

---

# 1. Overview

## Summary

Router that resolves the downstream service for a request using a static path-to-service configuration, extracts and validates `tenant_id` exclusively from JWT claims, checks tenant active status (cached), strips any client-supplied tenant headers, and forwards a trusted internal tenant header downstream.

## Goals

- Deterministic, static route resolution
- Tenant identity sourced only from validated claims
- Cached tenant active-status check to avoid a DB hit per request

## Non-Goals

- Dynamic service discovery
- Per-tenant custom routing logic beyond tier/role

---

# 2. Architecture

## High-Level Design

```
Request (claims validated, authz passed)
│
▼
RoutingMiddleware
  │ 1. tenant_id = claims.TenantID
  │ 2. tenant active? (cache → fallback DB) No → 403
  │ 3. Resolve upstream from static route table by path+method
  │ 4. Strip inbound X-Tenant-ID (or similar) header
  │ 5. Set X-Gateway-Tenant-ID: {tenant_id} (trusted, internal)
  │ 6. Proxy request to upstream (with retry/timeout, TD-008)
▼
Downstream Service
```

---

# 3. Components

## New Components

- `router.RouteTable` — static config: `[]Route{Path, Method, Upstream, AuthRequired, PermissionsRequired}`
- `middleware.TenantIsolationMiddleware` — extracts/validates tenant, strips spoofed headers
- `store.TenantStatusCache` — Redis or in-memory TTL cache of `tenant_id → is_active`
- `proxy.ReverseProxy` wrapper around Go's `httputil.ReverseProxy`

## Modified Components

- None (new feature)

---

# 4. Data Model

## New Tables

### tenants

Per `project-overview.md` §11 — `id, name, slug, tier, rate_limit_per_minute, rate_limit_per_hour, max_users, features, is_active, created_at, updated_at, deleted_at`.

## Schema Changes

None (initial creation; shared with TD-005/TD-006 rate limit/cache config columns).

## Redis Keys

- `tenant:status:{tenant_id}` — cached active-status flag, short TTL (TODO: define TTL, not specified in overview)

---

# 5. API Design

## New Endpoints

None — this is the core proxying/routing layer, not a standalone API.

## Endpoint Changes

All proxied routes now require a resolvable `tenant_id` and active tenant status; inactive tenant returns:

```json
403 { "error": "tenant_inactive", "message": "tenant is not active" }
```

Unmatched route returns `404`.

---

# 6. Sequence Flow

```
Request (claims validated, authz passed)
│
Extract tenant_id from claims
│
Tenant active? (cache, fallback DB) ──No──> 403
│ Yes
Resolve route from static table ──No match──> 404
│ Match
Strip client tenant headers
│
Set X-Gateway-Tenant-ID
│
Proxy to upstream (TD-008 retry/timeout applied)
```

---

# 7. Security

## Authentication

Depends on TD-001 (upstream, already validated).

## Authorization

Depends on TD-003 (upstream, already validated).

## Data Protection

Tenant ID never sourced from headers — this is the core isolation guarantee (ADR-003). Client-supplied tenant-like headers are always stripped before forwarding.

## Rate Limiting

Rate limiting (TD-005) is applied using the `tenant_id` resolved here.

---

# 8. Performance

## Expected Load

Every request passes through this layer.

## Database Impact

Tenant active-status check hits DB only on cache miss; cache TTL bounds staleness vs. load tradeoff.

## Caching Strategy

`tenant:status:{tenant_id}` cached in Redis (or local in-memory with short TTL) to avoid a DB round-trip per request.

---

# 9. Monitoring

## Metrics

- `routing_requests_total{route,status}`
- `tenant_inactive_rejections_total`

## Logging

- Every routing decision logged with correlation ID, tenant_id, resolved upstream
- Inactive-tenant rejections logged distinctly for audit

## Alerts

- Spike in 404s (possible misconfiguration or route scanning attempt)
- Spike in tenant_inactive rejections for a single tenant (possible post-deactivation retry storm)

---

# 10. Risks

## Risk 1

Cross-tenant data leakage if tenant header spoofing is not fully stripped.

Mitigation: allowlist-based header forwarding (only explicitly allowed headers pass through) rather than blocklist, to avoid missing a spoofing vector.

---

## Risk 2

Tenant deactivated mid-session — active JWT still has old tenant status baked into claims is NOT the case (status checked live, not from claims), but cache staleness could allow a brief window of access post-deactivation.

Mitigation: short cache TTL bounds the staleness window; critical deactivations can bypass cache (force DB read) if needed.

---

# 11. Rollout Plan

## Deployment

1. Deploy tenants table migration (shared with TD-002 migration set)
2. Deploy routing middleware with static route config
3. Verify tenant isolation via integration tests (spoofed header rejected, correct tenant routed)

## Rollback

1. Revert route config
2. Roll back deployment

---

# 12. Open Questions

- ~~Reject vs. silently ignore a conflicting client-supplied tenant header?~~ Resolved 2026-07-20: silently strip, do not reject.
- ~~Tenant status cache TTL value?~~ Resolved 2026-07-20: 30 seconds.

---

## Follow-ups (Deferred)

- **`Route.AuthRequired` / `Route.PermissionsRequired` are declared but unenforced.** The route table schema (`internal/gateway/model.go`) and `config/routes.json` carry these fields per this TD's "New Components" section, but no code path reads them — every request under `/api/` is gated only by the blanket `auth.JWTAuthMiddleware` wrapping the whole prefix, regardless of a route's `AuthRequired` value, and `PermissionsRequired` is never checked against `claims.Permissions`. This wasn't in FEAT-004's acceptance criteria (FEAT-003's authorization middleware already runs upstream of routing), so it was left inert rather than half-built. Before either field is relied upon operationally, a future feature should:
  - Decide whether per-route auth/permission gating belongs in the gateway routing layer at all, or stays owned entirely by FEAT-003's middleware.
  - If it belongs here: enforce `PermissionsRequired` in `gateway.NewHandler` (`internal/gateway/handler.go`) using `claims.Permissions`, and support `AuthRequired: false` routes bypassing `auth.JWTAuthMiddleware` (would need the mux wiring in `internal/server/routes.go` to change from one blanket-authed `/api/` prefix to per-route registration).
  - Otherwise: drop the fields from `Route`/`config.RouteEntry` to avoid the schema implying enforcement that doesn't exist.

---

# 13. ADR References

- ADR-002: Redis for Distributed Rate Limiting and Caching
- ADR-003: Extract Tenant ID from JWT Claims Only
