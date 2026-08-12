# Example: Multi-PR Implementation Plan

This example shows how to decompose a complex feature (RBAC) into multiple, reviewable PRs.

## Feature: FEAT-002 - RBAC Data Model

### Complexity Analysis

**Estimated Scope**:

- 8 files to create/modify
- ~700 LOC total
- Database migrations required
- New HTTP endpoints
- In-memory cache layer

**Decision**: Split into 3 PRs

### Why Split?

1. **Migration Risk**: Database schema changes should be reviewed separately
2. **Layer Separation**: Data, service, and API layers have different reviewers
3. **Review Quality**: Smaller PRs get better review attention
4. **Merge Safety**: Can deploy data model before exposing APIs
5. **Testing**: Each layer can be tested independently

---

## PR Decomposition

### PR1: FEAT-002a - RBAC Schema and Models

**Branch**: `feat-002/part-1-schema`

**Scope**: Database foundation only

**Files**:

- `internal/database/migrations/NNNNN_create_permissions.sql`
- `internal/models/permission.go`
- `internal/models/permission_test.go`

**Why Separate?**:

- Schema changes need careful review from DBA/senior engineers
- Migration must be tested independently
- Can deploy and run migration before code uses it (safe deployment)

**Deliverables**:

- [ ] Migration creates `permissions` table
- [ ] Migration seeds all permissions from matrix
- [ ] Model structs with validation tags
- [ ] Unit tests for model validation

**Size**: 3 files, ~200 LOC

**Review Focus**:

- Schema correctness (types, constraints, indexes)
- Seed data accuracy (matches permission matrix)
- Migration reversibility (up/down both work)

**Merge Criteria**:

- [ ] Migration tested locally against PostgreSQL
- [ ] All unit tests pass
- [ ] Schema reviewed by 2+ engineers
- [ ] Seed data verified against documented matrix

---

### PR2: FEAT-002b - RBAC Cache and Repository

**Branch**: `feat-002/part-2-cache`

**Base**: Merge after PR1

**Scope**: In-memory cache layer for permission lookups

**Files**:

- `internal/auth/role_cache.go` (interface + implementation)
- `internal/auth/role_cache_test.go`
- `internal/repository/role_repository.go`
- `internal/repository/role_repository_test.go`

**Why Separate?**:

- Service layer logic can be tested without HTTP overhead
- Cache warming strategy needs performance validation
- Repository patterns should be consistent across codebase

**Deliverables**:

- [ ] `RoleCache` interface with `GetRole()` and `All()` methods
- [ ] Constructor loads from database once at startup
- [ ] Repository layer with proper error handling
- [ ] Unit tests with mocked database
- [ ] Cache performance tests (< 1µs lookup time)

**Size**: 4 files, ~350 LOC

**Review Focus**:

- Interface design (constructor-injected, not global)
- Cache warming efficiency (single query)
- Error handling completeness
- Test coverage (target 85%+)

**Merge Criteria**:

- [ ] All tests pass
- [ ] No N+1 queries
- [ ] Cache lookup performance validated
- [ ] Works with existing `auth.KeyStore` pattern

---

### PR3: FEAT-002c - RBAC HTTP Endpoints

**Branch**: `feat-002/part-3-api`

**Base**: Merge after PR2

**Scope**: Expose read-only API for roles and permissions

**Files**:

- `internal/handlers/roles_handler.go`
- `internal/handlers/roles_handler_test.go`
- `config/routes.go` (add new routes)
- `internal/apidocs/openapi.yaml` (update spec)

**Why Separate?**:

- API design should be reviewed separately from implementation
- OpenAPI spec changes need API team review
- Integration tests take longer to run

**Deliverables**:

- [ ] `GET /roles` endpoint with permission check
- [ ] `GET /permissions` endpoint with permission check
- [ ] 403 responses for unauthorized access
- [ ] Integration tests for both endpoints
- [ ] OpenAPI spec updated with new endpoints
- [ ] Postman collection includes new endpoints

**Size**: 4 files, ~250 LOC

**Review Focus**:

- API consistency with existing endpoints
- OpenAPI spec accuracy
- Authorization enforcement correctness
- Error response format consistency

**Merge Criteria**:

- [ ] All tests pass (unit + integration)
- [ ] OpenAPI spec validates
- [ ] Postman collection tested
- [ ] Authorization works end-to-end

---

## Implementation Timeline

### Week 1

- Day 1-2: PR1 (schema) - review, merge
- Day 3: Deploy PR1 to staging, run migration
- Day 4-5: PR2 (cache) - review, merge

### Week 2

- Day 1: Deploy PR2 to staging, verify cache loading
- Day 2-3: PR3 (API) - review, merge
- Day 4: Deploy PR3 to staging, full integration testing
- Day 5: Deploy to production

---

## Testing Strategy

### Per-PR Testing

**PR1**:

```bash
# Local migration test
make db-reset
make migrate-up
psql -c "SELECT * FROM permissions;"  # Verify seed data
make migrate-down
```

**PR2**:

```bash
# Unit tests with mocked DB
go test ./internal/auth/... -v
go test ./internal/repository/... -v

# Performance test
go test ./internal/auth/role_cache_test.go -bench=.
```

**PR3**:

```bash
# Integration tests
go test ./internal/handlers/... -tags=integration -v

# OpenAPI validation
npx swagger-cli validate internal/apidocs/openapi.yaml

# Manual testing
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/roles
```

### Cross-PR Testing

After all PRs merged:

```bash
# Full end-to-end flow
make docker-up
./scripts/seed-test-data.sh
./scripts/run-api-tests.sh
make load-test
```

---

## Alternative Decomposition Strategies

### Option B: Vertical Slice (Not Chosen)

```
PR1: Read-only operations (GET /roles, GET /permissions) - full stack
PR2: Role assignment operations (POST /users/:id/role)
PR3: Permission checks middleware
```

**Why not chosen**: Schema changes mixed with API code makes review harder

### Option C: Component-Based (Not Chosen)

```
PR1: All models (Role, Permission, User updates)
PR2: All repositories
PR3: All handlers
```

**Why not chosen**: Can't test/deploy incrementally; first PR doesn't provide value until PR3

---

## Success Criteria

Feature is considered "done" when:

- [ ] All 3 PRs merged to main
- [ ] Database migration run in production
- [ ] `GET /roles` and `GET /permissions` endpoints working
- [ ] Authorization checks enforced (`roles:read` permission required)
- [ ] No N+1 queries in role cache loading
- [ ] Cache lookup performance < 1µs
- [ ] Integration tests cover all endpoints
- [ ] OpenAPI spec updated and validated
- [ ] Production monitoring shows no errors

---

## Common Pitfalls to Avoid

1. **Don't Merge Out of Order**: PR2 depends on PR1's schema, PR3 depends on PR2's cache
2. **Don't Skip Integration Tests in PR3**: Unit tests alone don't catch routing issues
3. **Don't Deploy All at Once**: Deploy PR1, verify, then PR2, verify, then PR3
4. **Don't Ignore Migration Rollback**: Always test down migrations
5. **Don't Duplicate Test Data**: Seed once in migration, reference in tests

---

## Lessons Learned

**What Worked**:

- Separating schema from logic allowed better DBA review
- Smaller PRs got reviewed within 24 hours vs. 3-4 days
- Each PR was independently testable
- Cache layer could be performance-tested in isolation

**What Could Improve**:

- Could have included cache performance benchmarks in PR2 criteria
- Should have updated OpenAPI spec in PR1 (empty schemas) to reduce PR3 size
- Integration test setup could be shared between PR2 and PR3

---

## Template for Your Feature

Use this structure:

1. **Analyze complexity**: Files, LOC, dependencies
2. **Identify boundaries**: What can be tested independently?
3. **Order by risk**: Migrations first, APIs last
4. **Size PRs**: Target 200-400 LOC each
5. **Define success**: What makes each PR "done"?
6. **Document dependencies**: What must merge first?
7. **Test incrementally**: Don't wait for PR3 to start testing

Remember: **PR size is about reviewability, not arbitrary line counts.**
