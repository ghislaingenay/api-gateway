# FEAT-010: API Documentation & Dev Environment

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-22

Technical Design: [TD-010 - API Documentation & Dev Environment](../technical-designs/TD-010-api-docs-dev-environment.md)

---

# 1. Overview

## Summary

Developer-facing surface of the project: OpenAPI 3.0/Swagger documentation for all gateway-managed routes, and a Docker Compose setup that runs the gateway, PostgreSQL, Redis, and two mock downstream services for local development and evaluation. This feature is what makes the project usable and reviewable by other engineers, recruiters, and interviewers.

## Problem

A production-grade gateway is only demonstrably production-grade if it's easy to run locally and its API surface is documented. Without this, the portfolio/educational value of the project (a core project goal) is undermined, and other engineers cannot easily evaluate or extend the gateway.

## Goals

- Publish OpenAPI 3.0 spec covering all gateway routes (roles, permissions, health, and proxied routes where applicable)
- Serve interactive Swagger UI documentation
- Provide a `docker-compose.yml` that spins up: gateway, PostgreSQL, Redis, and two mock downstream services
- Ensure `docker compose up` results in a fully working local environment with seed data

## Non-Goals

- Hosted/production API documentation portal (local Swagger UI is sufficient for MVP)
- Admin dashboard UI (excluded from MVP entirely)

---

# 2. Users

## Primary Users

- Engineers evaluating or contributing to the project
- Recruiters/hiring managers/technical interviewers reviewing the portfolio project

## Stakeholders

- Engineering (documentation accuracy)

---

# 3. User Stories

### Story 1

As an engineer evaluating this project

I want to run `docker compose up` and have a fully working gateway with mock services

So that I can explore the system without complex setup

### Story 2

As a developer integrating with the gateway

I want an OpenAPI spec with interactive Swagger UI

So that I can understand available endpoints, auth requirements, and request/response schemas without reading source code

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The project must include an OpenAPI 3.0 spec covering all gateway-managed endpoints (roles, permissions, health, ready, and route configuration for proxied services).

#### Acceptance Criteria

- [x] OpenAPI spec validates against the 3.0 schema
- [x] Every documented endpoint matches actual gateway behavior (auth requirements, response codes)
- [x] Swagger UI is served at a discoverable path (e.g., `/docs`) — TODO: confirm exact path, not specified in overview

---

### FR-2

`docker-compose.yml` must define services for: gateway, PostgreSQL, Redis, and two mock downstream services.

#### Acceptance Criteria

- [x] `docker compose up` starts all services with correct dependency ordering (DB/Redis healthy before gateway starts)
- [x] Database migrations run automatically on startup (or via a documented one-line command)
- [x] Mock downstream services respond to at least one representative route each for demonstration

---

### FR-3

The project README must document setup, architecture, and key design decisions with diagrams.

#### Acceptance Criteria

- [x] README includes an architecture diagram of the request flow
- [x] README includes local setup instructions matching the actual Docker Compose configuration
- [ ] README links to ADRs for key design decisions — no ADR files exist in the repo yet (context/ has no `adrs/` directory); linked to the equivalent TD design-decision docs instead. See report.

---

## Business Rules

- Local dev environment must not require any external paid services (Redis/Postgres run in containers)

---

## Permissions

N/A — this feature has no runtime authorization surface.

---

## User Flow

1. Developer clones the repository
2. Developer runs `docker compose up`
3. Gateway, PostgreSQL, Redis, and mock services start; migrations seed roles/permissions/tenants
4. Developer navigates to Swagger UI to explore documented endpoints
5. Developer sends authenticated requests against mock downstream services to see the gateway's full request flow in action

---

# 5. Edge Cases

- Port conflicts with existing local services (Postgres/Redis default ports)
- Migrations failing on first boot due to service startup ordering
- OpenAPI spec drifting from actual implementation over time (needs a way to detect drift — TODO: not specified in overview, e.g., contract testing)

---

# 6. Dependencies

## Internal

- Documents/exposes: [[FEAT-001]] through [[FEAT-009]] (all gateway behavior)

## External

- Docker & Docker Compose
- Swagger UI (or equivalent OpenAPI renderer)

## Prerequisites

- Core gateway features (FEAT-001 through FEAT-009) sufficiently implemented to document accurately

---

# 7. Success Criteria

## Business Metrics

- Positive engineering/recruiter feedback on ease of local setup (qualitative, per project's portfolio goal)
- 10+ GitHub stars (per project overview success criteria)

## Technical Metrics

- `docker compose up` succeeds on a clean machine in under 2 minutes (TODO: target not specified in overview, reasonable inference)
- Zero manual steps required beyond `docker compose up` for a working local environment

---

# 8. Related Documents

- Technical Design: TD-010
- Project README
- ADR-004: Use Go for API Gateway Implementation
