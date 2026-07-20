# FEAT-005: Distributed Rate Limiting

Status: Doing

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-20

Technical Design: [TD-005 - Distributed Rate Limiting](../technical-designs/TD-005-distributed-rate-limiting.md)

---

# 1. Overview

## Summary

Per-tenant distributed rate limiting using Redis and a sliding-window algorithm, protecting downstream services from abusive or runaway traffic while allowing each tenant's limits to be configured independently (with environment-variable defaults, overridable per tenant).

## Problem

Without centralized rate limiting, individual services must each implement their own throttling, leading to inconsistent limits and duplicated effort. Multi-tenant systems additionally need per-tenant isolation so one tenant's traffic spike cannot degrade service for others.

## Goals

- Enforce per-tenant rate limits using Redis with a sliding-window algorithm
- Support per-minute and per-hour limits, defaulting from environment variables, overridable per tenant (`tenants.rate_limit_per_minute`, `tenants.rate_limit_per_hour`)
- Fail open (allow requests) if Redis is unavailable, with alerting
- Return standard rate-limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`)

## Non-Goals

- Hot tenant detection and dynamic throttling (post-MVP)
- Per-endpoint or per-user rate limiting beyond tenant-level (post-MVP unless trivially derived)
- Four-tier Stripe-style strategy (concurrent/fleet/worker limits) — post-MVP

---

# 2. Users

## Primary Users

- All tenants making API requests through the gateway
- Downstream services (protected from traffic spikes)

## Stakeholders

- Engineering
- Operations (Redis availability, alerting)

---

# 3. User Stories

### Story 1

As a tenant on the free tier

I want my request rate capped at my plan's limit

So that the platform remains fair and predictable for all tenants

### Story 2

As a platform operator

I want the gateway to fail open if Redis becomes unavailable

So that a Redis outage does not cause a full platform outage

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must check and increment a per-tenant sliding-window counter in Redis before routing each request.

#### Acceptance Criteria

- [ ] Requests within limit proceed with `X-RateLimit-Remaining` header set
- [ ] Requests exceeding limit return 429 with `Retry-After` header
- [ ] Sliding window error rate stays within the documented ~0.003% tolerance

---

### FR-2

Rate limits must default from environment/config values and be overridable per tenant via the `tenants` table (`rate_limit_per_minute`, `rate_limit_per_hour`).

#### Acceptance Criteria

- [ ] Tenant with no override uses environment default
- [ ] Tenant with a configured override uses tenant-specific limit
- [ ] Both per-minute and per-hour windows are enforced independently

---

### FR-3

If Redis is unavailable, the gateway must fail open (allow the request) and emit an alertable log/metric.

#### Acceptance Criteria

- [ ] Redis connection failure does not block requests
- [ ] Failure event is logged with correlation ID and triggers a monitorable metric

---

## Business Rules

- Rate limit keys are namespaced per tenant: `ratelimit:{tenant_id}:{user_id}:current` / `:previous`
- Fail-open behavior is intentional per ADR/risk analysis — availability prioritized over strict enforcement during Redis outages

---

## Permissions

| Action                  | All Tenants |
| ------------------------ | ----------- |
| Subject to rate limiting | ✅          |
| Bypass rate limiting     | ❌ (no bypass in MVP) |

---

## User Flow

1. Request arrives with `tenant_id` resolved ([[FEAT-004]])
2. Gateway reads/increments sliding-window counters in Redis for the tenant
3. If within limit, request proceeds with rate-limit headers attached
4. If over limit, gateway returns 429 immediately (no downstream call)
5. If Redis is unreachable, gateway logs the failure and allows the request through

---

# 5. Edge Cases

- Redis connection timeout mid-check
- Tenant rate limit override set to 0 or negative (should be rejected at config validation)
- Sliding window boundary conditions (burst exactly at window edge)
- Hot tenant causing Redis contention for other tenants (mitigated post-MVP by hot tenant detection)

---

# 6. Dependencies

## Internal

- [[FEAT-004]] Multi-Tenant Isolation & Routing (source of `tenant_id`)
- [[FEAT-008]] Observability & Health Checks (fail-open events logged/alerted)

## External

- Redis 7.0+ / Redis Sentinel

## Prerequisites

- Redis cluster provisioned
- Tenants table with rate limit columns ([[FEAT-002]] schema pattern)

---

# 7. Success Criteria

## Business Metrics

- No tenant exceeds their configured limit undetected

## Technical Metrics

- Rate limit check adds < 5ms p99 latency
- Sliding window error rate ≤ 0.003%
- Fail-open activates within one Redis health-check interval of an outage

---

# 8. Related Documents

- Technical Design: TD-005
- ADR-002: Redis for Distributed Rate Limiting and Caching
- ADR-005: Sliding Window Algorithm for Rate Limiting
- [Rate Limiting Patterns](../../../knowledge/02-Software%20Engineering/Architecture/rate-limiting-patterns.md)
