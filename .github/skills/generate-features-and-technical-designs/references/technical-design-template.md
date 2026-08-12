# Technical Design Template

This template is used for all TD-XXX.md files in the `context/technical-designs/` directory.

## File Naming

- Format: `TD-XXX-kebab-case-name.md`
- Must match corresponding feature: `FEAT-XXX-same-name.md` → `TD-XXX-same-name.md`
- Example: `TD-005-distributed-rate-limiting.md`

## Complete Template

```markdown
# TD-XXX: Feature Title

Status: Not Started | In Progress | Done

Owner: [Name]
Created: YYYY-MM-DD
Last Updated: YYYY-MM-DD

Feature Spec: [FEAT-XXX - Feature Title](../features/FEAT-XXX-feature-name.md)

---

# 1. Overview

## Summary

[Technical approach summary - what technologies, patterns, and architecture will be used]

## Goals

[Technical objectives this design achieves]

## Non-Goals

[Technical scope exclusions - what approaches are NOT being used and why]

---

# 2. Architecture

## High-Level Design

[ASCII diagram or description showing component interaction flow]
```

Client Request
│
▼
Middleware A
│
▼
Middleware B
│
▼
Service Layer
│
▼
Database/Cache

```

## Technology Choices

[Rationale for specific libraries, patterns, and tools]

- **Library X**: [Why chosen over alternatives]
- **Pattern Y**: [Why this approach fits the problem]

---

# 3. Components

## New Components

- **Component 1** (`path/to/component1.go`): [Purpose and responsibilities]
- **Component 2** (`path/to/component2.go`): [Purpose and responsibilities]

## Modified Components

- **Existing Component** (`path/to/existing.go`): [What changes and why]

## Component Diagram

```

┌─────────────┐
│ Handler │
└──────┬──────┘
│
▼
┌─────────────┐
│ Service │
└──────┬──────┘
│
▼
┌─────────────┐
│ Repository │
└─────────────┘

````

---

# 4. Data Model

## New Tables

### table_name

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uuid | PK | Primary key |
| name | varchar(100) | NOT NULL, UNIQUE | Entity name |
| created_at | timestamptz | NOT NULL | Creation timestamp |

**Indexes**:
- `idx_table_name_field` on (field)
- `idx_table_name_composite` on (field1, field2)

**Constraints**:
- Foreign key to `other_table(id)` on delete cascade

---

## Schema Changes

### Migration: `NNNNN_add_field_to_table.sql`

```sql
-- Up
ALTER TABLE existing_table
ADD COLUMN new_field varchar(50) NOT NULL DEFAULT 'value';

-- Down
ALTER TABLE existing_table
DROP COLUMN new_field;
````

---

## Redis Keys

| Pattern                         | Example                     | Purpose          | TTL  |
| ------------------------------- | --------------------------- | ---------------- | ---- |
| `prefix:tenant:{tenant_id}:key` | `cache:tenant:123:user:456` | Per-tenant cache | 5min |
| `lock:{resource}:{id}`          | `lock:user:789`             | Distributed lock | 30s  |

---

# 5. API Design

## New Endpoints

### `POST /api/v1/resource`

**Description**: [What this endpoint does]

**Authentication**: Required (JWT)

**Authorization**: Requires `resource:create` permission

**Request**:

```json
{
  "field1": "string",
  "field2": 123,
  "optional_field": "string"
}
```

**Validation**:

- `field1`: Required, 1-100 characters
- `field2`: Required, positive integer

**Response** (201 Created):

```json
{
  "id": "uuid",
  "field1": "string",
  "field2": 123,
  "created_at": "2026-08-11T10:00:00Z"
}
```

**Errors**:

- `400`: Invalid request body
- `401`: Missing or invalid JWT
- `403`: Insufficient permissions
- `409`: Resource already exists

---

### `GET /api/v1/resource/:id`

**Description**: [What this endpoint does]

**Authentication**: Required (JWT)

**Authorization**: Requires `resource:read` permission

**Path Parameters**:

- `id`: UUID of the resource

**Response** (200 OK):

```json
{
  "id": "uuid",
  "field1": "string",
  "field2": 123,
  "created_at": "2026-08-11T10:00:00Z",
  "updated_at": "2026-08-11T11:00:00Z"
}
```

**Errors**:

- `401`: Missing or invalid JWT
- `403`: Insufficient permissions
- `404`: Resource not found

---

## Modified Endpoints

### `PUT /api/v1/existing-resource/:id`

**Changes**:

- Added new field `additional_field` to request/response
- Now requires `resource:update` permission (previously `resource:write`)

---

# 6. Sequence Flow

## Happy Path

```
Client                Gateway              Service            Database
  │                      │                    │                  │
  │ 1. POST /resource    │                    │                  │
  │─────────────────────>│                    │                  │
  │                      │ 2. Validate JWT    │                  │
  │                      │                    │                  │
  │                      │ 3. Check Permission│                  │
  │                      │                    │                  │
  │                      │ 4. Call Service    │                  │
  │                      │───────────────────>│                  │
  │                      │                    │ 5. Insert Row    │
  │                      │                    │─────────────────>│
  │                      │                    │<─────────────────│
  │                      │<───────────────────│                  │
  │<─────────────────────│                    │                  │
  │ 6. 201 Created       │                    │                  │
```

**Steps**:

1. Client sends request with JWT in Authorization header
2. Gateway validates JWT signature and extracts claims
3. Authorization middleware checks user has required permission
4. Request forwarded to service layer
5. Service validates business rules and inserts into database
6. Response returned to client

## Error Flow

```
Client                Gateway              Service
  │                      │                    │
  │ 1. POST /resource    │                    │
  │─────────────────────>│                    │
  │                      │ 2. Validate JWT    │
  │                      │    (FAILS)         │
  │<─────────────────────│                    │
  │ 3. 401 Unauthorized  │                    │
```

---

# 7. Error Handling

| Scenario              | HTTP Status | Error Code                 | Response Body                                                                  | Logging |
| --------------------- | ----------- | -------------------------- | ------------------------------------------------------------------------------ | ------- |
| Invalid JWT signature | 401         | `invalid_token`            | `{"error": "invalid_token", "message": "Token signature validation failed"}`   | WARN    |
| Missing permission    | 403         | `insufficient_permissions` | `{"error": "insufficient_permissions", "message": "Requires resource:create"}` | INFO    |
| Validation failure    | 400         | `validation_failed`        | `{"error": "validation_failed", "fields": {"field": "error message"}}`         | INFO    |
| Resource not found    | 404         | `not_found`                | `{"error": "not_found", "message": "Resource not found"}`                      | INFO    |
| Database error        | 500         | `internal_error`           | `{"error": "internal_error", "message": "An unexpected error occurred"}`       | ERROR   |

## Retry Policy

- **Transient failures** (network, timeout): Retry with exponential backoff (3 attempts max)
- **Non-retryable failures** (4xx errors): Fail immediately
- **Circuit breaker**: Open after 5 consecutive failures, half-open after 30s

---

# 8. Testing Strategy

## Unit Tests

### Component 1 Tests (`component1_test.go`)

- [ ] Test valid input handling
- [ ] Test validation error cases
- [ ] Test dependency injection
- [ ] Test error propagation

### Component 2 Tests (`component2_test.go`)

- [ ] Test business logic paths
- [ ] Test edge cases
- [ ] Mock external dependencies

**Target Coverage**: 80% minimum per package

---

## Integration Tests

### API Integration Tests (`api_test.go`)

- [ ] Test complete request flow (auth → validation → processing)
- [ ] Test multi-tenant isolation
- [ ] Test concurrent requests
- [ ] Test rate limiting behavior

**Setup**: Use `testcontainers` for PostgreSQL, Redis

---

## End-to-End Tests

- [ ] Test feature through actual HTTP API
- [ ] Test with real JWT tokens
- [ ] Test failure scenarios (network, database down)

---

# 9. Implementation Phases (PR Mapping)

[Only include if feature uses multi-PR strategy]

## Phase 1: PR1 - Data Model and Migrations

**Branch**: `feat-XXX/part-1-data-model`

**Technical Scope**:

- **Files**:
  - `internal/database/migrations/NNNNN_create_table.sql`
  - `internal/models/entity.go`
  - `internal/models/entity_test.go`
- **Tests**: Unit tests for model validation
- **Migration**: Create table with indexes and constraints

**Review Focus**:

- Schema design correctness
- Index strategy
- Migration up/down reversibility

**Merge Criteria**:

- [ ] Migration tested locally
- [ ] All unit tests pass
- [ ] Schema reviewed by 2+ engineers

---

## Phase 2: PR2 - Service Layer

**Branch**: `feat-XXX/part-2-service`

**Base**: Merge after Phase 1

**Technical Scope**:

- **Files**:
  - `internal/service/resource_service.go`
  - `internal/service/resource_service_test.go`
  - `internal/repository/resource_repo.go`
- **Tests**: Unit tests with mocked repository
- **Dependencies**: Uses models from Phase 1

**Review Focus**:

- Business logic correctness
- Error handling completeness
- Test coverage (target 85%)

**Merge Criteria**:

- [ ] All tests pass
- [ ] Coverage > 80%
- [ ] No database queries in service layer tests

---

## Phase 3: PR3 - API Layer

**Branch**: `feat-XXX/part-3-api`

**Base**: Merge after Phase 2

**Technical Scope**:

- **Files**:
  - `internal/handlers/resource_handler.go`
  - `internal/handlers/resource_handler_test.go`
  - `config/routes.go` (add new routes)
  - `internal/apidocs/openapi.yaml` (update spec)
- **Tests**: Integration tests for HTTP endpoints
- **Dependencies**: Uses service from Phase 2

**Review Focus**:

- API design consistency
- OpenAPI spec accuracy
- Request/response validation

**Merge Criteria**:

- [ ] All tests pass
- [ ] OpenAPI spec validated
- [ ] Postman collection updated
- [ ] Integration tests cover happy + error paths

---

# 10. Security Considerations

## Authentication

[How is identity verified?]

## Authorization

[What permission checks are enforced?]

## Input Validation

[How is user input sanitized?]

## Secrets Management

[How are sensitive values stored/accessed?]

## Audit Logging

[What security-relevant events are logged?]

---

# 11. Performance Considerations

## Caching Strategy

[What is cached? TTL? Invalidation?]

## Query Optimization

[Any indexes? Query patterns? N+1 prevention?]

## Rate Limiting

[Per-tenant? Per-endpoint? Limits?]

## Resource Limits

[Connection pools? Goroutine limits? Memory?]

---

# 12. Observability

## Logging

```go
logger.Info("action completed",
    "user_id", userID,
    "tenant_id", tenantID,
    "duration_ms", duration,
)
```

## Metrics

- `resource_requests_total` (counter) - Total requests by status
- `resource_duration_seconds` (histogram) - Request duration
- `resource_errors_total` (counter) - Errors by type

## Tracing

- Span: `resource.Create` with attributes: `tenant.id`, `user.id`

---

# 13. Deployment Notes

## Prerequisites

- PostgreSQL migration NNNNN must run first
- Redis must be available
- New environment variables: `FEATURE_ENABLED=true`

## Migration Steps

1. Run database migration: `make migrate-up`
2. Restart gateway service
3. Verify health check passes: `GET /health`

## Rollback Plan

1. Revert code deployment
2. Run down migration: `make migrate-down`
3. Verify system stability

## Configuration Changes

```yaml
# config.yaml
feature:
  enabled: true
  cache_ttl: 300
  rate_limit: 100
```

---

# 14. Open Questions

- [ ] [Technical question requiring decision]
- [ ] [Performance threshold to validate]
- [ ] [Integration point to clarify]

---

# 15. References

- [Related Design Doc]
- [External Library Documentation]
- [RFC or Standard]

```

## Field Descriptions

### Architecture Section
Include ASCII diagrams showing component flow. Keep it high-level but clear.

### Components Section
List every file that will be created or modified. Use Go package paths.

### Data Model Section
- Full table schemas with types and constraints
- Show migration SQL
- Document indexes rationally (why needed)
- Document Redis key patterns with TTLs

### API Design Section
- Complete request/response examples
- All error codes
- Authentication/authorization requirements
- Validation rules

### Sequence Flow Section
Use ASCII sequence diagrams to show step-by-step interaction between components.

### Implementation Phases
Only include if using multi-PR strategy. Map each PR to specific files and tests.

### Security/Performance/Observability
These are critical sections - don't skip them. Address:
- What could go wrong?
- How do we prevent it?
- How do we measure it?
- How do we debug it?

## Examples

See:
- `context/technical-designs/TD-001-jwt-authentication.md` for a foundational component
- `context/technical-designs/TD-005-distributed-rate-limiting.md` for a complex, multi-PR feature
```
