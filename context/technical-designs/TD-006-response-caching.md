# TD-006: Response Caching

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Feature Spec: [FEAT-006 - Response Caching](../features/FEAT-006-response-caching.md)

---

# 1. Overview

## Summary

Redis-backed response cache for GET requests, keyed by `cache:{tenant_id}:{method}:{path}:{query_hash}` to guarantee tenant isolation by construction. TTL-based expiry, no explicit invalidation in MVP.

## Goals

- Tenant-scoped cache keys, always derived from validated claims
- Configurable per-route TTL
- Fail open to downstream call on Redis unavailability

## Non-Goals

- Write-triggered cache invalidation
- Caching non-GET methods

---

# 2. Architecture

## High-Level Design

```
Request (GET, tenant_id resolved, TD-004; rate limit passed, TD-005)
│
▼
CacheMiddleware
  │ 1. key = cache:{tenant_id}:{method}:{path}:{normalized_query_hash}
  │ 2. Redis GET key
  │ 3. Hit? → return cached response directly
  │ 4. Miss/Redis error → proceed to downstream (TD-004 routing + TD-008 retry/timeout)
  │ 5. On 2xx downstream response → Redis SET key with route TTL
▼
Response to client
```

---

# 3. Components

## New Components

- `cache.ResponseCache` — Redis-backed get/set with tenant-scoped keys
- `middleware.CacheMiddleware`
- `cache.QueryNormalizer` — sorts/normalizes query params before hashing for consistent keys

## Modified Components

- `router.Route` config (TD-004) — extended with optional `CacheTTL` field per route

---

# 4. Data Model

## New Tables

None.

## Schema Changes

None.

## Redis Keys

- `cache:{tenant_id}:{method}:{path}:{query_hash}` — cached response body + status + headers, TTL per route

---

# 5. API Design

## New Endpoints

None — middleware only.

## Endpoint Changes

Cacheable GET routes gain an `X-Cache: HIT|MISS` response header for observability (TODO: confirm this header is desired — reasonable inference, not specified in overview).

---

# 6. Sequence Flow

```
GET Request (tenant_id resolved)
│
Normalize query params, compute key
│
Redis GET key
│
Hit? ──Yes──> Return cached response (X-Cache: HIT)
│ No / Redis error
Proceed downstream (TD-004/TD-008)
│
2xx response? ──Yes──> Redis SET key, TTL (X-Cache: MISS)
│ No (4xx/5xx)
Return response, do not cache
```

---

# 7. Security

## Authentication

N/A (runs after TD-001/TD-003).

## Authorization

N/A — cache hit still requires the request to have passed authz for the resource; cache key includes tenant_id so isolation is structural.

## Data Protection

Tenant isolation is guaranteed by always deriving `tenant_id` from validated claims for the cache key — never from request input.

## Rate Limiting

Cache hits still count against rate limits (TD-005 runs before this middleware) — TODO: confirm ordering, reasonable default to avoid cache from being used to bypass rate limiting is to rate-limit before caching.

---

# 8. Performance

## Expected Load

Cache hits avoid a downstream call entirely — primary latency/load reduction mechanism.

## Database Impact

None.

## Caching Strategy

TTL-based expiry only in MVP; no active invalidation. Default TTL and per-route overrides configurable (TODO: define default TTL value, not specified in overview).

---

# 9. Monitoring

## Metrics

- `cache_requests_total{result="hit|miss|error"}`
- `cache_hit_ratio` (derived)

## Logging

- Cache hit/miss logged with correlation ID, tenant_id, cache key (hashed path only, no sensitive query values in logs)

## Alerts

- Sustained cache_hit_ratio drop (possible cache infrastructure issue)

---

# 10. Risks

## Risk 1

Cache poisoning — a malicious or buggy upstream response cached and served to the same tenant repeatedly.

Mitigation: only 2xx responses are cached; TTL bounds the blast radius of any bad cached entry.

---

## Risk 2

Query parameter ordering inconsistency causing cache misses or, worse, key collisions.

Mitigation: `QueryNormalizer` sorts params before hashing, ensuring a canonical key regardless of client-supplied order.

---

# 11. Rollout Plan

## Deployment

1. Deploy alongside TD-005 (shares Redis infrastructure)
2. Enable caching per route via config flag, starting with low-risk read-only routes
3. Verify tenant isolation via integration tests (two tenants requesting the same path get isolated cache entries)

## Rollback

1. Disable caching via feature flag (all requests pass through to downstream)
2. Roll back deployment

---

# 12. Open Questions

- Default TTL value
- Exact rate-limit-vs-cache middleware ordering

---

# 13. ADR References

- ADR-002: Redis for Distributed Rate Limiting and Caching
