# TD-010: API Documentation & Dev Environment

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Feature Spec: [FEAT-010 - API Documentation & Dev Environment](../features/FEAT-010-api-docs-dev-environment.md)

---

# 1. Overview

## Summary

An OpenAPI 3.0 spec generated/maintained for all gateway-managed endpoints and served via Swagger UI, plus a `docker-compose.yml` orchestrating the gateway, PostgreSQL, Redis, and two mock downstream services for a fully self-contained local development environment.

## Goals

- OpenAPI 3.0 spec accurately documenting all gateway endpoints
- Swagger UI served locally
- One-command (`docker compose up`) local environment with migrations and seed data applied automatically

## Non-Goals

- Hosted documentation portal
- Admin dashboard UI

---

# 2. Architecture

## High-Level Design

```
docker-compose.yml
├── postgres (with healthcheck)
├── redis (with healthcheck)
├── gateway (depends_on: postgres healthy, redis healthy; runs migrations on start)
│    └── serves /docs (Swagger UI) from openapi.yaml
├── mock-service-a (simple echo/CRUD mock)
└── mock-service-b (simple echo/CRUD mock)
```

---

# 3. Components

## New Components

- `openapi.yaml` — OpenAPI 3.0 spec covering `/roles`, `/permissions`, `/health`, `/ready`, and proxied route patterns
- `handlers.SwaggerUIHandler` — serves Swagger UI at `/docs` backed by `openapi.yaml`
- `docker-compose.yml` — full local stack
- `cmd/migrate` — one-shot migration runner invoked at gateway container startup
- Two minimal mock downstream services (e.g., simple Go or Node HTTP servers returning static/echo responses)

## Modified Components

- README.md — updated with architecture diagram and setup instructions matching the compose file

---

# 4. Data Model

None new — documents the schemas already defined in TD-001 through TD-009.

---

# 5. API Design

## New Endpoints

### GET /docs

Purpose: serves interactive Swagger UI.

Response: `200` HTML (Swagger UI) referencing `openapi.yaml`.

---

## Endpoint Changes

None — this feature documents existing endpoints, does not add gateway-facing behavior beyond `/docs`.

---

# 6. Sequence Flow

```
docker compose up
│
postgres starts, healthcheck passes
redis starts, healthcheck passes
│
gateway container starts
  │ runs migrations (TD-002 roles/permissions, TD-004 tenants)
  │ starts HTTP server, serves /docs
│
mock-service-a, mock-service-b start
│
Developer opens http://localhost:PORT/docs
```

---

# 7. Security

## Authentication

`/docs` is unauthenticated in local dev (documentation surface only, no data exposure).

## Authorization

N/A.

## Data Protection

Mock services use only synthetic/seed data, no real tenant data in local dev.

## Rate Limiting

`/docs` excluded from rate limiting, consistent with `/health`/`/ready` (TD-009).

---

# 8. Performance

## Expected Load

Local development only — not a production performance concern.

## Database Impact

Migrations run once at container startup.

## Caching Strategy

N/A.

---

# 9. Monitoring

## Metrics

N/A — local dev tooling, not a production monitoring surface.

## Logging

Compose logs aggregate all service output to stdout for local debugging.

## Alerts

N/A.

---

# 10. Risks

## Risk 1

OpenAPI spec drifting from actual gateway behavior over time.

Mitigation: TODO — consider contract testing (e.g., validating responses against the spec in CI) as a post-MVP improvement; not specified in overview.

---

## Risk 2

Port conflicts with the developer's existing local Postgres/Redis instances.

Mitigation: document non-default ports in `docker-compose.yml` and README, or use Docker's internal networking exclusively with only the gateway port exposed to the host.

---

# 11. Rollout Plan

## Deployment

1. Author `openapi.yaml` covering all endpoints from TD-001 through TD-009
2. Build `docker-compose.yml` with healthchecks and startup ordering
3. Build minimal mock downstream services
4. Verify clean-machine `docker compose up` end-to-end

## Rollback

N/A — local dev tooling only, no production rollback concern.

---

# 12. Open Questions

- Contract testing approach to keep OpenAPI spec in sync with implementation

---

# 13. ADR References

- ADR-004: Use Go for API Gateway Implementation
