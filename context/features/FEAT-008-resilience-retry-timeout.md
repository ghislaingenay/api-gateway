# FEAT-008: Resilience (Retry & Timeout)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-22

Technical Design: [TD-008 - Resilience (Retry & Timeout)](../technical-designs/TD-008-resilience-retry-timeout.md)

---

# 1. Overview

## Summary

Reliability layer for outbound calls to downstream services: retry logic with exponential backoff for transient failures, and timeout handling with deadline propagation to prevent slow downstream services from exhausting gateway resources.

## Problem

Downstream services can fail transiently or become slow under load. Without retries, transient failures become user-facing errors unnecessarily. Without timeouts and deadline propagation, a single slow downstream service can exhaust gateway connections/goroutines and degrade the entire platform.

## Goals

- Retry idempotent (GET, and explicitly safe) requests on transient failures with exponential backoff
- Enforce a request deadline that propagates from the client request through to the downstream call
- Avoid retrying non-idempotent requests (POST/PATCH/DELETE) by default
- Bound total retry time so a request never exceeds the overall gateway timeout budget

## Non-Goals

- Full circuit breaker implementation (post-MVP per project scope)
- Per-downstream-service adaptive retry tuning (post-MVP)

---

# 2. Users

## Primary Users

- API clients (benefit from transparent transient-failure recovery)
- Downstream services (protected from being overwhelmed by tight retry loops)

## Stakeholders

- Engineering
- Operations

---

# 3. User Stories

### Story 1

As an API client

I want transient downstream failures to be retried automatically

So that I don't see spurious errors for momentary blips

### Story 2

As a platform operator

I want every request to have a hard deadline

So that a slow downstream service cannot cause unbounded resource consumption at the gateway

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must retry GET requests to downstream services on transient failures (connection errors, 502/503/504) using exponential backoff, up to a configured max attempt count.

#### Acceptance Criteria

- [ ] Transient failure on first attempt triggers a retry after backoff delay
- [ ] Backoff delay increases exponentially between attempts (with jitter)
- [ ] Max retry attempts is configurable (TODO: default value not specified in overview)
- [ ] Non-idempotent methods (POST/PATCH/DELETE) are not retried by default

---

### FR-2

Every request must have a deadline propagated via `context.Context` from the incoming request through to the downstream HTTP call.

#### Acceptance Criteria

- [ ] Downstream call is cancelled if the overall request deadline is exceeded
- [ ] Client receives 504 Gateway Timeout when deadline is exceeded
- [ ] Deadline is configurable per route (TODO: default timeout value not specified in overview)

---

### FR-3

Total time spent retrying must not exceed the request's overall deadline budget.

#### Acceptance Criteria

- [ ] Retry loop respects `ctx.Done()` and aborts remaining retries once deadline passes
- [ ] Final response after exhausted retries/deadline is a clear timeout/failure error, not a hang

---

## Business Rules

- Only idempotent methods (GET, and explicitly configured safe routes) are retried
- Retry attempts and timeouts must respect the propagated deadline, never exceeding it

---

## Permissions

| Action                      | All Requests |
| ------------------------------ | ------------ |
| Subject to timeout/deadline    | ✅           |
| Subject to retry (GET only)    | ✅           |

---

## User Flow

1. Request passes validation ([[FEAT-007]]) and rate limiting ([[FEAT-005]])
2. Gateway derives a deadline-bound context for the downstream call
3. Gateway attempts the downstream call; on transient failure and idempotent method, retries with exponential backoff
4. If deadline is exceeded at any point, gateway aborts and returns 504
5. Successful response returned to client; failure after exhausted retries returned with appropriate error

---

# 5. Edge Cases

- Downstream service accepts the request but is slow to respond (times out mid-processing)
- Retry succeeds but response arrives after client-facing deadline already passed
- Non-idempotent request fails transiently (must not be retried, surfaces error immediately)
- Backoff jitter causing retry to occur just as deadline expires

---

# 6. Dependencies

## Internal

- [[FEAT-004]] Multi-Tenant Isolation & Routing (retry/timeout wraps the downstream call made here)
- [[FEAT-008]] shares infra with [[FEAT-009]] Observability (retry attempts and timeouts are logged)

## External

- Downstream microservices (mocked for MVP)

## Prerequisites

- FEAT-004 routing implemented

---

# 7. Success Criteria

## Business Metrics

- Reduction in user-facing errors caused by transient downstream blips (target TODO: not specified in overview)

## Technical Metrics

- No request exceeds its configured deadline budget
- Retry overhead adds no more than the configured max backoff time to p99 latency for affected requests

---

# 8. Related Documents

- Technical Design: TD-008
- [API Gateway Patterns](../../../knowledge/02-Software%20Engineering/Architecture/api-gateway-patterns.md)
