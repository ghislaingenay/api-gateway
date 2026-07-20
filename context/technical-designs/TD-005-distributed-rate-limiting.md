# TD-005: Distributed Rate Limiting

Status: Doing

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-20

Feature Spec: [FEAT-005 - Distributed Rate Limiting](../features/FEAT-005-distributed-rate-limiting.md)

---

# 1. Overview

## Summary

Redis-backed sliding-window rate limiter, keyed per tenant (and optionally per user), enforcing both per-minute and per-hour limits sourced from tenant configuration with environment-variable defaults. Fails open with alerting if Redis is unavailable.

## Goals

- Sliding-window algorithm with ~0.003% error tolerance (Cloudflare-style two-bucket approximation)
- Per-tenant limit overrides from the `tenants` table
- Fail-open behavior on Redis failure, with alerting

## Non-Goals

- Hot tenant detection/dynamic throttling
- Four-tier Stripe-style limiting

---

# 2. Architecture

## High-Level Design

```
Request (tenant_id resolved, TD-004)
│
▼
RateLimitMiddleware
  │ 1. Load tenant limits (from cache/tenants table)
  │ 2. Compute sliding window count via Redis (current + previous buckets)
  │ 3. count > limit? Yes → 429 + Retry-After
  │ 4. Redis unreachable? → log + alert + allow (fail open)
  │ 5. No → set X-RateLimit-* headers, proceed
▼
Routing (TD-004 continues)
```

---

# 3. Components

## New Components

- `ratelimit.SlidingWindowLimiter` — implements the two-bucket sliding window approximation against Redis
- `middleware.RateLimitMiddleware`
- `store.TenantLimitsCache` — reuses/extends TD-004's tenant cache to include rate limit fields

## Modified Components

- `store.TenantStatusCache` (TD-004) — extended to also cache `rate_limit_per_minute`, `rate_limit_per_hour`

---

# 4. Data Model

## New Tables

None (uses `tenants` table from TD-004 for per-tenant overrides).

## Schema Changes

None.

## Redis Keys

- `ratelimit:{tenant_id}:{user_id}:current`
- `ratelimit:{tenant_id}:{user_id}:previous`

(Per `project-overview.md` §11 Redis Keys.)

---

# 5. API Design

## New Endpoints

None — middleware only.

## Endpoint Changes

All proxied routes gain rate-limit response headers and a 429 case:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 12
Retry-After: 34
```

```json
429 { "error": "rate_limit_exceeded", "message": "too many requests" }
```

---

# 6. Sequence Flow

```
Request (tenant_id resolved)
│
Load tenant limits (cache)
│
Redis INCR current bucket, read previous bucket
│
Redis unreachable? ──Yes──> log + alert + allow (fail open)
│ No
Compute weighted count (sliding window formula)
│
count > limit? ──Yes──> 429 + Retry-After
│ No
Set X-RateLimit-* headers
│
Proceed to routing
```

---

# 7. Security

## Authentication

N/A (runs after TD-001/TD-003).

## Authorization

N/A — rate limiting applies uniformly per tenant regardless of role (no bypass for any role in MVP).

## Data Protection

Rate limit keys are namespaced by tenant_id and user_id — no cross-tenant counter leakage.

## Rate Limiting

This is the rate limiting component.

---

# 8. Performance

## Expected Load

Every proxied request performs one Redis round-trip (INCR + read).

## Database Impact

None directly — tenant limits read from cache (populated from `tenants` table, refreshed periodically).

## Caching Strategy

Tenant limit overrides cached alongside tenant status (TD-004); Redis itself is the "cache"/state store for the sliding window counters.

---

# 9. Monitoring

## Metrics

- `rate_limit_checks_total{result="allow|deny|fail_open"}`
- `redis_rate_limit_errors_total`

## Logging

- Every 429 logged with tenant_id, user_id, correlation ID
- Every fail-open event logged at WARN level with correlation ID

## Alerts

- Redis connectivity failure triggering sustained fail-open (paging alert)
- Sustained 429 rate for a single tenant (possible abuse, informational)

---

# 10. Risks

## Risk 1

Redis outage causing full fail-open (no rate limiting enforced platform-wide).

Mitigation: fail-open is an accepted tradeoff (availability > strict enforcement); paired with immediate alerting so operators can respond quickly.

---

## Risk 2

Sliding window boundary error (~0.003%) allowing minor over-limit bursts.

Mitigation: documented, accepted tolerance per Cloudflare's published approach; not a correctness bug.

---

# 11. Rollout Plan

## Deployment

1. Deploy Redis cluster/Sentinel
2. Deploy rate limit middleware behind a feature flag defaulting to enforce-with-fail-open
3. Verify limits via load testing against configured tenant tiers

## Rollback

1. Disable middleware via feature flag (requests proceed unthrottled — acceptable temporary state, not a security issue since routing/authz remain enforced)
2. Roll back deployment

---

# 12. Open Questions

- Exact sliding window formula/library choice (custom implementation vs. existing Go library)

---
