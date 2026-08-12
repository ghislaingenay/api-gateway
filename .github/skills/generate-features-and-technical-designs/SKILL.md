---
name: generate-features-and-technical-designs
description: "Generate feature specifications and technical design documents from a project overview. Creates FEAT-XXX.md and TD-XXX.md files following established patterns, with PR decomposition strategy for incremental implementation. Use when: creating feature specs from overview, generating technical designs, planning feature implementation, breaking down large features into smaller PRs, structuring project documentation."
argument-hint: "Path to project overview file or feature description"
user-invocable: true
---

# Generate Features and Technical Designs

## Purpose

Transform a high-level project overview into structured feature specifications (FEAT-XXX.md) and paired technical designs (TD-XXX.md), following a consistent documentation pattern. Additionally, analyzes features to determine optimal PR decomposition strategy for incremental, reviewable implementation.

## When to Use

- Creating feature specs from a project overview document
- Generating technical design documents for planned features
- Structuring undocumented features into the standard format
- Planning feature implementation with PR boundaries
- Breaking down large features into smaller, reviewable PRs
- Establishing project documentation baseline

## Inputs

Required:

- Project overview file (e.g., `context/project-overview.md`)

Optional:

- Specific feature to extract (if generating one feature instead of all)
- Target feature numbering (if continuing an existing sequence)
- PR size preference (small/medium/large)

## Outputs

For each identified feature:

1. `context/features/FEAT-XXX-feature-name.md` - Feature specification
2. `context/technical-designs/TD-XXX-feature-name.md` - Technical design
3. PR decomposition strategy (inline in FEAT/TD or separate plan file)

## Procedure

### Step 1: Analyze Project Overview

1. Read the project overview document
2. Identify the "Core Features" section
3. Extract each feature with its description
4. Understand dependencies between features
5. Review existing features to determine next available feature number

```bash
# Check existing features
ls -la context/features/
ls -la context/technical-designs/
```

### Step 2: Determine Feature Boundaries and PR Strategy

For each identified feature:

1. **Assess Feature Complexity**
   - Lines of code estimate
   - Number of files to be created/modified
   - External dependencies
   - Data model changes
   - API surface changes

2. **Evaluate PR Decomposition**

   **Keep as Single PR** if:
   - Feature touches < 10 files
   - No breaking changes to existing APIs
   - Can be implemented in < 500 LOC
   - No complex migrations
   - Minimal cross-cutting concerns

   **Split into Multiple PRs** if:
   - Feature requires data model + logic + API layers
   - Can separate foundation vs. implementation
   - Has multiple independent capabilities
   - Includes migrations that should land separately
   - Touches > 15 files or > 800 LOC total

3. **Define PR Boundaries**

   Common split patterns:
   - **Data Model First**: Schema, migrations, models → Business logic → HTTP handlers
   - **Foundation + Features**: Core infrastructure → Individual capabilities
   - **Phase by Layer**: Persistence → Service → API → Integration
   - **By Subsystem**: Authentication → Authorization → Audit

   Example (RBAC feature):

   ```
   PR1: FEAT-002a - RBAC Data Model (roles + permissions tables, migrations)
   PR2: FEAT-002b - RBAC Read APIs (GET /roles, GET /permissions)
   PR3: FEAT-002c - RBAC Assignment Logic (helper functions for checks)
   ```

### Step 3: Generate Feature Specifications

For each feature, create `context/features/FEAT-XXX-feature-name.md`:

**Template Structure** (see [reference](./references/feature-template.md)):

```markdown
# FEAT-XXX: Feature Title

Status: Not Started | In Progress | Done
Owner: [Name]
Created: [Date]
Last Updated: [Date]

Technical Design: [TD-XXX - Feature Title](../technical-designs/TD-XXX-feature-name.md)

---

# 1. Overview

## Summary

[2-3 sentence description of what this feature does]

## Problem

[What pain point does this solve?]

## Goals

- [Concrete, measurable goal 1]
- [Concrete, measurable goal 2]

## Non-Goals

- [Explicitly excluded scope]

---

# 2. Users

## Primary Users

[Who directly uses this feature?]

## Stakeholders

[Who is affected by this feature?]

---

# 3. User Stories

### Story 1

As a [role]
I want to [action]
So that [benefit]

---

# 4. Product Requirements

## Functional Requirements

### FR-1

[Requirement statement]

#### Acceptance Criteria

- [ ] [Testable criterion 1]
- [ ] [Testable criterion 2]

---

# 5. Success Metrics

[How do we measure success?]

---

# 6. Dependencies

- Depends on: [FEAT-XXX]
- Blocks: [FEAT-YYY]

---

# 7. Implementation Plan

## PR Decomposition Strategy

[Choose one of the following]

### Single PR Implementation

**Scope**: Entire feature in one pull request
**Rationale**: [Why it's small enough / cohesive enough for one PR]
**Estimated Size**: [XX files, ~YYY LOC]

### Multi-PR Implementation

#### PR1: [FEAT-XXX-a] - [Component Name]

**Scope**: [What's included]
**Deliverables**:

- [ ] [File/component 1]
- [ ] [File/component 2]
      **Estimated Size**: [XX files, ~YYY LOC]

#### PR2: [FEAT-XXX-b] - [Component Name]

**Scope**: [What's included]
**Dependencies**: Must merge after PR1
**Deliverables**:

- [ ] [File/component 1]
- [ ] [File/component 2]
      **Estimated Size**: [XX files, ~YYY LOC]

---

# 8. Open Questions

- [ ] [Question requiring clarification]
```

### Step 4: Generate Technical Designs

For each feature, create `context/technical-designs/TD-XXX-feature-name.md`:

**Template Structure** (see [reference](./references/technical-design-template.md)):

```markdown
# TD-XXX: Feature Title

Status: Not Started | In Progress | Done
Owner: [Name]
Created: [Date]
Last Updated: [Date]

Feature Spec: [FEAT-XXX - Feature Title](../features/FEAT-XXX-feature-name.md)

---

# 1. Overview

## Summary

[Technical approach summary]

## Goals

[Technical objectives]

## Non-Goals

[Technical scope exclusions]

---

# 2. Architecture

## High-Level Design

[ASCII diagram or description of component interaction]
```

Component A → Component B → Component C

````

## Technology Choices
[Rationale for libraries, patterns, tools]

---

# 3. Components

## New Components
- [Component 1]: [Purpose]
- [Component 2]: [Purpose]

## Modified Components
- [Existing component]: [Changes needed]

---

# 4. Data Model

## New Tables
[Schema definitions]

## Schema Changes
[Migrations for existing tables]

## Redis Keys
[Cache/session key patterns]

---

# 5. API Design

## New Endpoints

### `POST /resource`
**Request**:
```json
{
  "field": "value"
}
````

**Response**:

```json
{
  "id": "uuid",
  "status": "created"
}
```

---

# 6. Sequence Flow

```
Client → Gateway → Service → Database
  ↓        ↓         ↓          ↓
 [1]      [2]       [3]        [4]
```

1. [Step description]
2. [Step description]

---

# 7. Error Handling

| Scenario      | Error Code | Response                         |
| ------------- | ---------- | -------------------------------- |
| Invalid input | 400        | `{"error": "validation_failed"}` |

---

# 8. Testing Strategy

## Unit Tests

- [ ] [Component test 1]
- [ ] [Component test 2]

## Integration Tests

- [ ] [Flow test 1]
- [ ] [Flow test 2]

---

# 9. Implementation Phases (PR Mapping)

[If multi-PR approach]

## Phase 1: PR1 - [Component Name]

**Technical Scope**:

- Files: [List of files to create/modify]
- Tests: [List of test files]
- Migration: [Migration file, if any]

## Phase 2: PR2 - [Component Name]

**Technical Scope**:

- Files: [List of files to create/modify]
- Tests: [List of test files]
- Depends on: Phase 1 merged

---

# 10. Security Considerations

[Authentication, authorization, input validation, secrets]

---

# 11. Performance Considerations

[Caching, query optimization, rate limiting]

---

# 12. Deployment Notes

[Migration steps, config changes, rollback plan]

---

# 13. Open Questions

- [ ] [Technical question requiring decision]

````

### Step 5: Create Reference Index

Update `context/features/README.md` with new entries:

```markdown
| FEAT-XXX | [Feature Title](FEAT-XXX-feature-name.md) | Not Started | [TD-XXX](../technical-designs/TD-XXX-feature-name.md) |
````

### Step 6: Validate Cross-References

Ensure:

- Each FEAT-XXX.md links to its corresponding TD-XXX.md
- Each TD-XXX.md links back to its FEAT-XXX.md
- Dependencies between features are explicitly documented
- PR decomposition is consistent between FEAT and TD
- Feature numbers are sequential and don't conflict

### Step 7: Generate Implementation Guidance (Optional)

For features with multi-PR strategy, optionally create:

`context/plans/feat-XXX-implementation-plan.md`:

```markdown
# FEAT-XXX Implementation Plan

## Overview

[Brief description]

## PR Sequence

### PR1: Foundation

- **Branch**: `feat-XXX/part-1-foundation`
- **Scope**: Data models, migrations
- **Review Focus**: Schema design, migration safety
- **Merge Requirements**: All tests pass, migrations tested

### PR2: Business Logic

- **Branch**: `feat-XXX/part-2-logic`
- **Base**: `feat-XXX/part-1-foundation` (after merge)
- **Scope**: Service layer, validation
- **Review Focus**: Business logic correctness, error handling
- **Merge Requirements**: All tests pass, 80%+ coverage

### PR3: API Layer

- **Branch**: `feat-XXX/part-3-api`
- **Base**: `feat-XXX/part-2-logic` (after merge)
- **Scope**: HTTP handlers, OpenAPI updates
- **Review Focus**: API design, documentation completeness
- **Merge Requirements**: All tests pass, OpenAPI validated

## Integration Testing

[How to test the complete feature after all PRs merge]
```

## Best Practices

### Feature Specification Guidelines

1. **Be Specific**: Use concrete acceptance criteria, not vague descriptions
2. **User-Centric**: Frame requirements from user perspective
3. **Testable**: Every requirement should be verifiable
4. **Bounded**: Clear goals AND non-goals
5. **Referenced**: Link to related features explicitly

### Technical Design Guidelines

1. **Implementation-Ready**: Enough detail for an engineer to start coding
2. **Technology-Specific**: Name actual libraries, patterns, file paths
3. **Data-Driven**: Include actual schemas, API examples, sequence diagrams
4. **Test-Conscious**: Specify testing strategy for each component
5. **Deployment-Aware**: Include migration steps, config changes

### PR Decomposition Guidelines

1. **Reviewable Size**: Target 200-400 LOC per PR (max 600)
2. **Functional Boundaries**: Each PR should deliver a testable unit
3. **Merge Safety**: Earlier PRs shouldn't break main branch
4. **Clear Dependencies**: Explicitly state PR ordering requirements
5. **Independent Testing**: Each PR should have its own test coverage

### Common Patterns

**Pattern 1: Data-First**

```
PR1: Schema + Models
PR2: Repository Layer
PR3: Service Layer
PR4: API Layer
```

**Pattern 2: Vertical Slice**

```
PR1: Read Operations (full stack)
PR2: Write Operations (full stack)
PR3: Delete Operations (full stack)
```

**Pattern 3: Foundation + Features**

```
PR1: Core Infrastructure (middleware, interfaces)
PR2: Feature A (using infrastructure)
PR3: Feature B (using infrastructure)
```

## Anti-Patterns to Avoid

1. **Mega Features**: Don't create one feature that should be 3-4 separate features
2. **Missing Dependencies**: Always document what must exist first
3. **Vague Requirements**: "Should be fast" → "Must respond within 200ms at p95"
4. **Copy-Paste**: Don't duplicate content between FEAT and TD; cross-reference
5. **Monolithic PRs**: Don't create 2000-line PRs when it could be 4×500-line PRs
6. **Premature Splitting**: Don't split a 300-LOC feature just to have multiple PRs

## Examples

See the reference directory for complete examples:

- [Example Feature: JWT Authentication](./references/example-feature.md)
- [Example Technical Design: JWT Authentication](./references/example-technical-design.md)
- [Example Multi-PR Plan: RBAC](./references/example-multi-pr-plan.md)

## Quality Checklist

Before completing, verify:

- [ ] Feature numbers are sequential and unique
- [ ] All FEAT ↔ TD bidirectional links are correct
- [ ] Dependencies between features are documented
- [ ] Each feature has clear acceptance criteria
- [ ] Each technical design has concrete implementation details
- [ ] PR decomposition strategy is present for features > 500 LOC
- [ ] Cross-cutting concerns (security, performance, deployment) are addressed
- [ ] README index is updated with new features
- [ ] Status field is set appropriately
- [ ] Owner and dates are filled in
