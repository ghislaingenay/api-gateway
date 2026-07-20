# FEAT-004: Multi-Tenant Isolation & Routing

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-20

Technical Design: [TD-004 - Multi-Tenant Isolation & Routing](../technical-designs/TD-004-multi-tenant-routing.md)

---

# 1. Overview

## Summary

Enforces strict tenant isolation by extracting `tenant_id` exclusively from validated JWT claims (never headers), and routes requests to the correct downstream service based on path plus claim attributes (tenant tier, role). This is the core differentiator of the gateway versus generic reverse proxies.

## Problem

Multi-tenant SaaS applications must guarantee that one tenant can never access another tenant's data, even accidentally. Trusting tenant identifiers from request headers is spoofable; routing logic needs a single trusted source of tenant identity that flows into every downstream decision (rate limiting, caching, data access).

## Goals

- Extract `tenant_id` only from validated JWT claims, reject any tenant identifier supplied via header
- Route requests to the correct downstream service based on static path-to-service configuration
- Support attribute-based routing decisions using claims (`tenant_id`, `role`, `permissions`)
- Reject requests for inactive or soft-deleted tenants (`is_active = false`, `deleted_at IS NOT NULL`)
- Propagate tenant context (`tenant_id`) to downstream services via trusted internal header

## Non-Goals

- Dynamic service discovery (Consul/Kubernetes DNS) — static config only for MVP
- Per-tenant custom routing rules beyond tier/role-based decisions

---

# 2. Users

## Primary Users

- All authenticated API clients (every request is tenant-scoped)
- Downstream microservices (receive tenant-scoped, pre-validated requests)

## Stakeholders

- Engineering
- Security (tenant isolation guarantee)

---

# 3. User Stories

### Story 1

As a tenant user

I want my requests to be routed only within my own tenant's data boundary

So that I can never accidentally or maliciously access another tenant's data

### Story 2

As a platform operator

I want inactive or deleted tenants to be blocked at the gateway

So that revoked tenants cannot continue making requests

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must extract `tenant_id` exclusively from validated JWT claims and must ignore/reject any `X-Tenant-ID` or similar header if present.

#### Acceptance Criteria

- [x] `tenant_id` used for routing/rate limiting/caching always comes from `claims.TenantID`
- [x] A request with a conflicting tenant header is either ignored or rejected (resolved 2026-07-20: silently ignored/stripped, not rejected)

---

### FR-2

The gateway must route requests to downstream services based on a static path-to-service configuration table.

#### Acceptance Criteria

- [x] Configured routes (path, method, upstream URL) resolve to the correct downstream service
- [x] Unmatched paths return 404

---

### FR-3

The gateway must reject requests for tenants where `is_active = false` or `deleted_at IS NOT NULL`.

#### Acceptance Criteria

- [x] Request for inactive tenant returns 403 with a clear error message
- [x] Tenant active-status check does not add more than one DB/cache lookup per request (cached)

---

### FR-4

The gateway must forward tenant context to downstream services via a trusted internal header (e.g., `X-Gateway-Tenant-ID`) set only by the gateway itself.

#### Acceptance Criteria

- [x] Downstream services receive `tenant_id` without needing to parse the JWT themselves
- [x] Any client-supplied header with the same name is stripped before forwarding

---

## Business Rules

- Tenant ID is never trusted from headers — claims only (see ADR-003)
- Routing configuration is static for MVP (path, method, upstream URL, auth required, permissions required)

---

## Permissions

| Action                       | Any Authenticated Tenant User |
| ------------------------------ | -------------------------------- |
| Route to own tenant's services | ✅                                |
| Route to another tenant's data | ❌ (structurally impossible)     |

---

## User Flow

1. Request arrives with validated claims (post-FEAT-001, post-FEAT-003)
2. Gateway extracts `tenant_id` from claims
3. Gateway checks tenant `is_active` status (cached lookup)
4. Gateway resolves destination service from static route config
5. Gateway strips client-supplied tenant headers and sets trusted `X-Gateway-Tenant-ID`
6. Request forwarded to downstream service

---

# 5. Edge Cases

- Client sends conflicting `X-Tenant-ID` header alongside valid JWT
- Tenant deactivated mid-session (active JWT, now-inactive tenant)
- Path matches no configured route
- Path matches multiple route patterns (precedence rules)

---

# 6. Dependencies

## Internal

- [[FEAT-001]] JWT Authentication (source of `tenant_id`)
- [[FEAT-003]] Authorization Enforcement (runs before routing)
- [[FEAT-005]] Distributed Rate Limiting (keyed by `tenant_id` from this feature)
- [[FEAT-006]] Response Caching (keyed by `tenant_id` from this feature)

## External

- Downstream microservices (mocked for MVP via Docker Compose)

## Prerequisites

- FEAT-001, FEAT-003 implemented
- Tenants table populated ([[FEAT-002]] migration pattern)

---

# 7. Success Criteria

## Business Metrics

- Zero cross-tenant data leakage incidents in security testing

## Technical Metrics

- Tenant active-status check adds < 2ms p99 latency (via cache)
- Routing decision latency < 1ms

---

# 8. Related Documents

- Technical Design: TD-004
- ADR-002: Redis for Distributed Rate Limiting and Caching
- ADR-003: Extract Tenant ID from JWT Claims Only
- [Multi-Tenant Architecture](../../../knowledge/02-Software%20Engineering/Architecture/multi-tenant-architecture.md)
- [API Gateway Patterns](../../../knowledge/02-Software%20Engineering/Architecture/api-gateway-patterns.md)
