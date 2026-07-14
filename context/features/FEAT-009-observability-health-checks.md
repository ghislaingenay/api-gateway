# FEAT-009: Observability & Health Checks

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Technical Design: [TD-009 - Observability & Health Checks](../technical-designs/TD-009-observability-health-checks.md)

---

# 1. Overview

## Summary

Structured JSON logging with correlation IDs across every gateway component, plus health/readiness endpoints for orchestration and monitoring. This feature underpins debuggability and audit requirements for every other feature (authorization denials, rate-limit fail-opens, retries, cache hits/misses).

## Problem

Without structured logging and correlation IDs, tracing a single request's journey across authentication, authorization, rate limiting, caching, and downstream calls is nearly impossible. Operators also need a reliable way to know if the gateway (and its dependencies) is healthy for deployment orchestration.

## Goals

- Emit structured JSON logs with a correlation ID generated (or propagated) per request
- Attach correlation ID to response headers so clients/support can reference it
- Provide `/health` (liveness) and `/ready` (readiness, checks Redis/Postgres connectivity) endpoints
- Log key lifecycle events: auth failures, authorization denials, rate-limit fail-opens, cache hit/miss, retries, timeouts

## Non-Goals

- Prometheus/Grafana dashboards (metrics exposed but dashboards excluded from MVP)
- Distributed tracing with W3C Trace Context (post-MVP)

---

# 2. Users

## Primary Users

- Operations/SRE (health checks, log-based debugging)
- Engineering (request tracing during development/incident response)

## Stakeholders

- Operations
- Security/Compliance (audit trail via logs)

---

# 3. User Stories

### Story 1

As an operator

I want a correlation ID attached to every request and log line

So that I can trace a single request's full path through the gateway during an incident

### Story 2

As a deployment orchestrator (Docker Compose/Kubernetes)

I want `/health` and `/ready` endpoints

So that I can determine when the gateway is safe to route traffic to

---

# 4. Product Requirements

## Functional Requirements

### FR-1

Every incoming request must be assigned a correlation ID (generated if not present in `X-Correlation-ID` header, propagated if present) and included in all log lines and the response header.

#### Acceptance Criteria

- [ ] Correlation ID present in every structured log line for a request's lifecycle
- [ ] Correlation ID returned in response header
- [ ] Client-supplied correlation ID is honored if present and well-formed

---

### FR-2

The gateway must expose `GET /health` (liveness, no dependency checks) returning 200 if the process is running.

#### Acceptance Criteria

- [ ] `/health` returns 200 with no auth required
- [ ] `/health` does not check Redis/Postgres (liveness only)

---

### FR-3

The gateway must expose `GET /ready` (readiness) returning 200 only if Redis and PostgreSQL are reachable, 503 otherwise.

#### Acceptance Criteria

- [ ] `/ready` returns 200 when both dependencies are healthy
- [ ] `/ready` returns 503 with dependency status detail when either is unreachable
- [ ] No auth required

---

### FR-4

Key lifecycle events across other features must emit structured logs: auth failures ([[FEAT-001]]), authorization denials ([[FEAT-003]]), rate-limit fail-opens ([[FEAT-005]]), cache hit/miss ([[FEAT-006]]), retries/timeouts ([[FEAT-008]]).

#### Acceptance Criteria

- [ ] Each event type has a distinct, greppable log field (e.g., `event_type`)
- [ ] Logs are valid JSON, parseable by standard log aggregation tools

---

## Business Rules

- All logs are structured JSON (no unstructured/plaintext logs in production paths)
- Health endpoints are never subject to authentication, rate limiting, or caching middleware

---

## Permissions

| Action              | All Callers (including unauthenticated) |
| --------------------- | ------------------------------------------- |
| Access `/health`     | ✅                                           |
| Access `/ready`      | ✅                                           |

---

## User Flow

1. Request arrives, gateway assigns/propagates correlation ID before any other middleware runs
2. All subsequent middleware (auth, authz, rate limit, cache, routing) log their decisions with the correlation ID
3. Response returned with correlation ID header
4. Orchestrator periodically polls `/health` and `/ready` to determine gateway status

---

# 5. Edge Cases

- Malformed client-supplied `X-Correlation-ID` (should be replaced with a generated one, not trusted blindly)
- `/ready` check itself timing out (should have its own short timeout, not hang indefinitely)
- Log volume under high load (log sampling — TODO: not specified in overview, likely out of MVP scope)

---

# 6. Dependencies

## Internal

- Consumed by/logs from: [[FEAT-001]], [[FEAT-003]], [[FEAT-005]], [[FEAT-006]], [[FEAT-008]]

## External

- PostgreSQL 15+ (readiness check)
- Redis 7.0+ (readiness check)

## Prerequisites

- None (foundational, implemented early alongside FEAT-001)

---

# 7. Success Criteria

## Business Metrics

- 100% of production incidents traceable via correlation ID across logs

## Technical Metrics

- `/health` and `/ready` respond in < 50ms p99
- 100% of gateway log lines are valid structured JSON

---

# 8. Related Documents

- Technical Design: TD-009
- [Multi-Tenant Architecture](../../../knowledge/02-Software%20Engineering/Architecture/multi-tenant-architecture.md)
