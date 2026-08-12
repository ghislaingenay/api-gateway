# Feature Specification Template

This template is used for all FEAT-XXX.md files in the `context/features/` directory.

## File Naming

- Format: `FEAT-XXX-kebab-case-name.md`
- Example: `FEAT-005-distributed-rate-limiting.md`

## Complete Template

```markdown
# FEAT-XXX: Feature Title

Status: Not Started | In Progress | Done

Owner: [Name]
Created: YYYY-MM-DD
Last Updated: YYYY-MM-DD

Technical Design: [TD-XXX - Feature Title](../technical-designs/TD-XXX-feature-name.md)

---

# 1. Overview

## Summary

[2-3 sentences describing what this feature does and why it exists]

## Problem

[What pain point does this solve? What happens without this feature?]

## Goals

- [Concrete, measurable goal 1]
- [Concrete, measurable goal 2]
- [Concrete, measurable goal 3]

## Non-Goals

- [Explicitly excluded scope item 1]
- [Explicitly excluded scope item 2]

---

# 2. Users

## Primary Users

[Who directly uses or interacts with this feature?]

## Stakeholders

[Who is affected by this feature? Who cares about its success?]

---

# 3. User Stories

### Story 1

As a [role/persona]

I want to [action/capability]

So that [benefit/outcome]

### Story 2

As a [role/persona]

I want to [action/capability]

So that [benefit/outcome]

---

# 4. Product Requirements

## Functional Requirements

### FR-1

[Clear statement of what the system must do]

#### Acceptance Criteria

- [ ] [Specific, testable criterion 1]
- [ ] [Specific, testable criterion 2]
- [ ] [Specific, testable criterion 3]

---

### FR-2

[Another functional requirement]

#### Acceptance Criteria

- [ ] [Criterion 1]
- [ ] [Criterion 2]

---

## Non-Functional Requirements

### NFR-1: Performance

[Performance requirement with specific numbers]

#### Acceptance Criteria

- [ ] [Measurable performance criterion]

---

### NFR-2: Security

[Security requirement]

#### Acceptance Criteria

- [ ] [Testable security criterion]

---

# 5. Success Metrics

[How do we measure that this feature is successful?]

- [Metric 1: e.g., "Reduces authentication failures by 95%"]
- [Metric 2: e.g., "P95 latency under 50ms"]

---

# 6. Dependencies

## Depends On

- [FEAT-XXX]: [Why this feature must exist first]

## Blocks

- [FEAT-YYY]: [What features are waiting for this one]

---

# 7. Implementation Plan

## PR Decomposition Strategy

[Choose ONE of the following approaches]

### Option A: Single PR Implementation

**Scope**: Entire feature in one pull request

**Rationale**: [Explain why this is appropriate - e.g., "Small, self-contained feature touching only 5 files with ~300 LOC"]

**Estimated Size**: [XX files, ~YYY LOC]

**Branch**: `feat-XXX/feature-name`

**Deliverables**:

- [ ] [Component 1]
- [ ] [Component 2]
- [ ] [Tests]
- [ ] [Documentation]

---

### Option B: Multi-PR Implementation

#### PR1: [FEAT-XXX-a] - [Component Name]

**Scope**: [What's included in this PR - e.g., "Data model and migrations"]

**Rationale**: [Why this should be separate - e.g., "Schema must be reviewed separately"]

**Branch**: `feat-XXX/part-1-component-name`

**Deliverables**:

- [ ] [File/component 1]
- [ ] [File/component 2]
- [ ] [Migration files]
- [ ] [Unit tests]

**Estimated Size**: [XX files, ~YYY LOC]

**Review Focus**: [What reviewers should focus on]

---

#### PR2: [FEAT-XXX-b] - [Component Name]

**Scope**: [What's included in this PR]

**Dependencies**: Must merge after PR1 ([FEAT-XXX-a])

**Branch**: `feat-XXX/part-2-component-name`

**Deliverables**:

- [ ] [File/component 1]
- [ ] [File/component 2]
- [ ] [Integration tests]

**Estimated Size**: [XX files, ~YYY LOC]

**Review Focus**: [What reviewers should focus on]

---

# 8. Open Questions

- [ ] [Question requiring clarification before implementation]
- [ ] [Decision point that needs stakeholder input]

---

# 9. Future Enhancements

[Features or improvements explicitly deferred to later versions]

- [Enhancement 1]
- [Enhancement 2]
```

## Field Descriptions

### Status

- **Not Started**: Feature spec written, not yet in development
- **In Progress**: Implementation has begun
- **Done**: Feature fully implemented, tested, and merged

### Overview Section

- **Summary**: Elevator pitch - what and why in 2-3 sentences
- **Problem**: The pain point this solves - helps justify the feature
- **Goals**: Positive outcomes this feature enables
- **Non-Goals**: Explicit scope exclusions to prevent scope creep

### User Stories

Use the classic "As a... I want... So that..." format. Focus on the outcome/benefit, not just the action.

### Functional Requirements (FR)

Each FR must:

- Be a single, clear requirement
- Have measurable acceptance criteria
- Use checkboxes for tracking
- Be testable

### PR Decomposition Strategy

Choose based on:

- **Single PR**: < 10 files, < 500 LOC, no complex dependencies
- **Multi PR**: > 15 files, > 800 LOC, or has natural boundaries (data/logic/API)

Common split patterns:

1. **Layer Split**: Data → Logic → API
2. **Capability Split**: Read → Write → Delete
3. **Foundation Split**: Core → Feature A → Feature B

### Dependencies

Always document what must exist before and what's waiting for this feature.

## Examples

See:

- `context/features/FEAT-001-jwt-authentication.md` for a foundational feature
- `context/features/FEAT-005-distributed-rate-limiting.md` for a complex feature with multi-PR approach
