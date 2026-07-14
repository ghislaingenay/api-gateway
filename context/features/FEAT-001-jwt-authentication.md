# FEAT-001: JWT Authentication (CBAC)

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Technical Design: [TD-001 - JWT Authentication (CBAC)](../technical-designs/TD-001-jwt-authentication.md)

---

# 1. Overview

## Summary

Claims-Based Access Control (CBAC) authentication layer that validates JWT tokens on every incoming request, enforces a strict signing-algorithm allowlist, and extracts identity (user, tenant, role, permissions) exclusively from validated claims. This is the foundational security layer every other gateway feature (RBAC, multi-tenancy, routing) depends on.

## Problem

Microservices need a unified, consistent way to authenticate requests without each service re-implementing token validation. Naive JWT implementations are vulnerable to algorithm confusion attacks (`alg=none`, HMAC/RSA confusion) and often trust unvalidated request headers for identity, which allows spoofing.

## Goals

- Validate JWT signature and standard claims (`exp`, `nbf`, `iat`) on every request
- Explicitly allowlist accepted signing algorithms and reject `alg=none`
- Extract `tenant_id`, `user_id`, `role`, `permissions`, `email` from validated claims only
- Support short-lived access tokens (5-15 minutes)
- Support key rotation via key ID (`kid`) without downtime

## Non-Goals

- Full OAuth2 authorization code flow (excluded from MVP)
- Token refresh mechanism (excluded from MVP)
- User login/credential issuance endpoints (assumed to be handled by an identity provider or seed data for MVP)

---

# 2. Users

## Primary Users

- Downstream microservices relying on the gateway for pre-authenticated requests
- API clients (web/mobile apps, third-party integrations) sending requests with JWTs

## Stakeholders

- Engineering (gateway implementation)
- Security (algorithm allowlist, token lifetime policy)

---

# 3. User Stories

### Story 1

As an API client

I want to authenticate once via a JWT in the `Authorization` header

So that I don't need to re-authenticate with every downstream service individually

### Story 2

As a platform operator

I want tokens signed with unapproved algorithms or `alg=none` to be rejected

So that the gateway is not vulnerable to algorithm confusion attacks

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must validate the JWT signature against an explicit algorithm allowlist (e.g., RS256) before trusting any claim.

#### Acceptance Criteria

- [ ] Tokens with `alg=none` are rejected with 401
- [ ] Tokens signed with an algorithm outside the allowlist are rejected with 401
- [ ] Tokens with invalid or missing signature are rejected with 401

---

### FR-2

The gateway must extract `tenant_id`, `user_id`, `role`, `role_id`, `permissions`, and `email` from validated JWT claims and attach them to the request context for downstream middleware.

#### Acceptance Criteria

- [ ] Valid token populates request context with `CustomClaims`
- [ ] Missing required claims (`tenant_id`, `user_id`) result in 401
- [ ] Expired (`exp`) or not-yet-valid (`nbf`) tokens are rejected with 401

---

### FR-3

The gateway must support multiple active signing keys identified by `kid` to allow rotation without downtime.

#### Acceptance Criteria

- [ ] Tokens signed with any currently active key are accepted
- [ ] Unknown `kid` values are rejected with 401

---

## Business Rules

- Tenant ID and role must never be trusted from request headers, only from validated JWT claims
- Access tokens should be short-lived (5-15 minutes)
- TODO: Define exact token issuance/login flow — not specified in project overview (assumed external to MVP gateway scope)

---

## Permissions

| Action                  | Any Authenticated Request |
| ------------------------ | -------------------------- |
| Access protected route  | Requires valid JWT         |
| Access health endpoint  | No auth required           |

---

## User Flow

1. Client sends request with `Authorization: Bearer <jwt>` header
2. Gateway parses JWT header, validates `alg` against allowlist and `kid` against active keys
3. Gateway verifies signature and standard claims (`exp`, `nbf`, `iat`)
4. Gateway extracts custom claims and attaches to request context
5. Request proceeds to authorization (RBAC) and routing middleware

---

# 5. Edge Cases

- Missing `Authorization` header
- Malformed JWT (not three base64 segments)
- Token signed with `alg=none`
- Token signed with unexpected algorithm (HMAC/RSA confusion)
- Expired token
- Token with unknown `kid`
- Token missing required custom claims

---

# 6. Dependencies

## Internal

- [[FEAT-002]] RBAC Data Model (roles/permissions referenced in claims)
- [[FEAT-003]] Authorization Enforcement (consumes claims from this feature)
- [[FEAT-004]] Multi-Tenant Isolation & Routing (consumes `tenant_id` from claims)

## External

- JWT signing key management (key generation/storage — TODO: specify KMS or file-based key storage, not defined in overview)

## Prerequisites

- None (foundational feature, implemented first)

---

# 7. Success Criteria

## Business Metrics

- Zero successful algorithm-confusion or `alg=none` bypass attempts in security testing

## Technical Metrics

- JWT validation latency < 5ms p99 (excluding network)
- 100% of protected routes reject unauthenticated requests

---

# 8. Related Documents

- Technical Design: TD-001
- ADR-001: CBAC with RBAC for Authentication and Authorization
- ADR-004: Use Go for API Gateway Implementation
- [JWT Security Vulnerabilities](../../../knowledge/02-Software%20Engineering/Security/jwt-security-vulnerabilities.md)
