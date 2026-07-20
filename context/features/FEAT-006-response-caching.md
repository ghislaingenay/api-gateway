# FEAT-006: Response Caching

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-20

Technical Design: [TD-006 - Response Caching](../technical-designs/TD-006-response-caching.md)

---

# 1. Overview

## Summary

Intelligent response caching for GET requests using Redis, with tenant-scoped cache keys to guarantee cache isolation between tenants and reduce load on downstream services.

## Problem

Repeated identical requests to downstream services waste compute and increase latency. In a multi-tenant system, a naive shared cache risks serving one tenant's cached response to another tenant (cache poisoning/leakage), which is a critical security concern, not just a performance one.

## Goals

- Cache GET responses in Redis with tenant-scoped keys: `cache:{tenant_id}:{method}:{path}:{query_hash}`
- Serve cache hits without calling the downstream service
- Support configurable TTLs per route
- Guarantee no cross-tenant cache reads are possible by construction (key always includes `tenant_id`)

## Non-Goals

- Cache invalidation triggered by downstream write events (post-MVP; MVP relies on TTL expiry)
- Write-through or read-through caching for non-GET methods

---

# 2. Users

## Primary Users

- All tenants making repeated GET requests
- Downstream services (reduced load from cache hits)

## Stakeholders

- Engineering
- Security (cache isolation guarantee)

---

# 3. User Stories

### Story 1

As a tenant user

I want frequently requested data to be served quickly from cache

So that I experience lower latency on repeated reads

### Story 2

As a platform operator

I want cache keys to always include the tenant ID

So that it is structurally impossible for one tenant to receive another tenant's cached response

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must check Redis for a cached response before forwarding a GET request downstream, using key format `cache:{tenant_id}:{method}:{path}:{query_hash}`.

#### Acceptance Criteria

- [x] Cache hit returns the stored response without a downstream call
- [x] Cache miss forwards to downstream and stores the response with configured TTL
- [x] Key always derives `tenant_id` from validated JWT claims, never from request input

---

### FR-2

Cached responses must respect a per-route configurable TTL.

#### Acceptance Criteria

- [x] Default TTL applied when route has no override
- [x] Route-specific TTL overrides the default
- [x] Expired entries are not served (Redis TTL handles eviction)

---

### FR-3

Only successful (2xx) GET responses are cached; error responses and non-GET methods are never cached.

#### Acceptance Criteria

- [x] 4xx/5xx responses are not written to cache
- [x] POST/PUT/PATCH/DELETE requests bypass the cache entirely

---

## Business Rules

- Cache keys must always include `tenant_id` — no shared/global cache entries permitted
- TODO: Define default TTL value (not specified in project overview)

---

## Permissions

| Action                      | All Tenants                  |
| --------------------------- | ---------------------------- |
| Read own cached data        | ✅                           |
| Read another tenant's cache | ❌ (structurally impossible) |

---

## User Flow

1. GET request arrives with `tenant_id` resolved ([[FEAT-004]])
2. Gateway computes cache key and checks Redis
3. On hit, gateway returns cached response immediately
4. On miss, gateway forwards to downstream, receives response, stores in Redis with TTL, returns response to client

---

# 5. Edge Cases

- Query parameter ordering affecting `query_hash` consistency (must normalize before hashing)
- Redis unavailable during cache check (should fail open to downstream call, consistent with FEAT-005 fail-open philosophy)
- Large response bodies exceeding a reasonable cache size limit
- Downstream response with `Cache-Control: no-store` (should be respected — TODO: confirm exact header handling, not specified in overview)

---

# 6. Dependencies

## Internal

- [[FEAT-004]] Multi-Tenant Isolation & Routing (source of `tenant_id`)
- [[FEAT-005]] Distributed Rate Limiting (shares Redis infrastructure)

## External

- Redis 7.0+ / Redis Sentinel

## Prerequisites

- Redis cluster provisioned

---

# 7. Success Criteria

## Business Metrics

- Zero cross-tenant cache leakage incidents

## Technical Metrics

- Cache hit ratio tracked and reported (target TODO: not specified in overview)
- Cache hit response latency < 5ms p99

---

# 8. Related Documents

- Technical Design: TD-006
- ADR-002: Redis for Distributed Rate Limiting and Caching
- [Multi-Tenant Architecture](../../../knowledge/02-Software%20Engineering/Architecture/multi-tenant-architecture.md)
