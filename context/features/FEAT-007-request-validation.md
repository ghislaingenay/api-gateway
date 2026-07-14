# FEAT-007: Request Validation

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Technical Design: [TD-007 - Request Validation](../technical-designs/TD-007-request-validation.md)

---

# 1. Overview

## Summary

Schema-based validation of incoming requests (headers, path/query parameters, and JSON bodies) at the gateway layer, rejecting malformed or malicious requests before they reach downstream services or consume rate-limit/cache resources unnecessarily.

## Problem

Downstream services need protection from malformed requests and malicious traffic. Without centralized validation, each service must implement its own defensive checks, leading to duplicated logic and inconsistent error responses.

## Goals

- Validate request bodies against defined schemas per route
- Validate required path/query parameters and headers
- Reject malformed requests with a consistent 400 error format before routing/rate-limiting overhead
- Support Go struct validation tags (`validate:"required,email"`, etc.) consistent with the data model

## Non-Goals

- Business-logic validation that requires downstream service state (e.g., uniqueness checks) — gateway performs structural/schema validation only

---

# 2. Users

## Primary Users

- All API clients (subject to validation)
- Downstream services (protected from malformed input)

## Stakeholders

- Engineering

---

# 3. User Stories

### Story 1

As an API client

I want to receive a clear, consistent error when my request is malformed

So that I can fix my request without needing to inspect downstream service behavior

### Story 2

As a downstream service owner

I want the gateway to reject invalid requests before they reach my service

So that my service doesn't need to duplicate input validation logic

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must validate JSON request bodies against a per-route schema (using struct validation tags consistent with the models defined in the project's data model).

#### Acceptance Criteria

- [ ] Valid body proceeds to next middleware
- [ ] Invalid body returns 400 with field-level error details
- [ ] Malformed (non-JSON) body returns 400, not 500

---

### FR-2

The gateway must validate required path/query parameters per route configuration.

#### Acceptance Criteria

- [ ] Missing required query/path parameter returns 400
- [ ] Type-mismatched parameter (e.g., non-UUID where UUID expected) returns 400

---

### FR-3

Validation errors must be returned in a consistent JSON error format across all routes.

#### Acceptance Criteria

- [ ] All validation failures return `{"error":"validation_failed","message":"...","fields":[...]}` (TODO: confirm exact schema — not specified in overview)

---

## Business Rules

- Validation runs after authentication/authorization and before rate limiting/caching, to avoid consuming those resources on malformed requests (TODO: confirm exact middleware ordering — reasonable inference, not explicitly specified in overview)

---

## Permissions

| Action                     | All Authenticated Clients |
| ---------------------------- | ---------------------------- |
| Requests subject to validation | ✅                          |

---

## User Flow

1. Request arrives, passes authentication/authorization
2. Gateway validates request against route's schema (body, params, headers)
3. Valid requests proceed to rate limiting and routing
4. Invalid requests return 400 immediately with error details

---

# 5. Edge Cases

- Empty body when body is required
- Body present when route expects none
- Oversized request body (needs a size limit — TODO: define max body size, not specified in overview)
- Unicode/encoding edge cases in string fields

---

# 6. Dependencies

## Internal

- [[FEAT-003]] Authorization Enforcement (validation runs after auth per business rule above)
- [[FEAT-005]] Distributed Rate Limiting (validation runs before rate limit consumption)

## External

- Go `validator` package (or equivalent) consistent with struct tags in the data model

## Prerequisites

- Route configuration schema defined (path, method, upstream, auth/permissions required)

---

# 7. Success Criteria

## Business Metrics

- Reduction in malformed-request errors reaching downstream services (target TODO: not specified in overview)

## Technical Metrics

- Validation latency < 2ms p99 for typical payloads

---

# 8. Related Documents

- Technical Design: TD-007
- [API Gateway Patterns](../../../knowledge/02-Software%20Engineering/Architecture/api-gateway-patterns.md)
