# Generate Features and Technical Designs Skill

## Overview

This skill automates the creation of structured feature specifications (FEAT-XXX.md) and paired technical design documents (TD-XXX.md) from a project overview. It includes intelligent PR decomposition strategies to help break down complex features into smaller, reviewable pull requests.

## What It Does

1. **Analyzes** your project overview document
2. **Identifies** core features and their dependencies
3. **Generates** structured feature specifications following a proven template
4. **Creates** paired technical design documents with implementation details
5. **Recommends** PR decomposition strategies for complex features
6. **Maintains** cross-references between features and technical designs

## When to Use

Invoke this skill when you need to:

- Document a new project's feature set systematically
- Break down a project overview into implementable chunks
- Create feature specifications for existing undocumented functionality
- Plan implementation with clear PR boundaries
- Establish a documentation baseline for your project

## How to Use

### Basic Usage

```
@workspace /generate-features-and-technical-designs context/project-overview.md
```

### Generate Specific Feature

```
@workspace /generate-features-and-technical-designs create FEAT-013 for GraphQL support
```

### With Custom PR Strategy

```
@workspace /generate-features-and-technical-designs context/project-overview.md --pr-size=small
```

## Output Structure

```
context/
├── features/
│   ├── FEAT-001-feature-name.md    ← Feature specification
│   ├── FEAT-002-feature-name.md
│   └── README.md                    ← Updated index
└── technical-designs/
    ├── TD-001-feature-name.md      ← Technical design
    ├── TD-002-feature-name.md
    └── README.md                    ← Updated index
```

## Key Features

### Consistent Documentation

- Every feature follows the same template
- Bidirectional links between FEAT and TD files
- Numbered sequence for easy reference

### PR Decomposition Intelligence

- Analyzes feature complexity (LOC, files, dependencies)
- Recommends single-PR vs. multi-PR strategies
- Provides concrete split patterns (data/logic/API, vertical slice, etc.)
- Includes PR sizing guidance (target 200-400 LOC)

### Best Practices Built-In

- Clear acceptance criteria (testable, measurable)
- Dependency tracking between features
- Security, performance, and observability sections
- Migration and deployment notes

## Templates Included

- **Feature Specification Template**: Complete structure for FEAT-XXX.md files
- **Technical Design Template**: Implementation-ready TD-XXX.md structure
- **Multi-PR Example**: Real-world RBAC feature split across 3 PRs

## PR Decomposition Strategies

The skill analyzes features and recommends one of these approaches:

### Single PR (< 500 LOC)

- Self-contained, small features
- No complex dependencies
- Quick to review and merge

### Multi-PR: Layer Split

```
PR1: Data Model (schema + migrations)
PR2: Service Layer (business logic)
PR3: API Layer (HTTP handlers)
```

### Multi-PR: Capability Split

```
PR1: Read Operations (full stack)
PR2: Write Operations (full stack)
PR3: Delete Operations (full stack)
```

### Multi-PR: Foundation First

```
PR1: Core Infrastructure (interfaces, middleware)
PR2: Feature A (using infrastructure)
PR3: Feature B (using infrastructure)
```

## Examples in This Repository

Study the existing features to see the pattern:

- **Simple Feature**: [FEAT-001 JWT Authentication](../../context/features/FEAT-001-jwt-authentication.md)
- **Complex Feature**: [FEAT-005 Distributed Rate Limiting](../../context/features/FEAT-005-distributed-rate-limiting.md)
- **Multi-PR Example**: FEAT-002 RBAC (split into 3 PRs: schema → cache → API)

## Quality Checklist

After generation, the skill ensures:

- ✅ Sequential feature numbering with no gaps
- ✅ Bidirectional FEAT ↔ TD links
- ✅ Dependencies documented
- ✅ Acceptance criteria are testable
- ✅ PR decomposition strategy present for complex features
- ✅ README indexes updated

## Benefits

### For Developers

- Clear implementation roadmap
- Reviewable PR sizes
- Testable acceptance criteria
- No missing requirements

### For Reviewers

- Smaller, focused PRs
- Clear context for each change
- Explicit dependencies
- Known review focus areas

### For Project Managers

- Feature tracking via status field
- Dependency visualization
- Progress measurement via checklists
- Realistic effort estimates

## References

- [Feature Template](./references/feature-template.md) - Complete FEAT-XXX.md structure
- [Technical Design Template](./references/technical-design-template.md) - Complete TD-XXX.md structure
- [Multi-PR Example](./references/example-multi-pr-plan.md) - Real-world decomposition strategy

## Related Skills

- `implement-feature` - Implements a feature from its FEAT/TD documents
- `review-technical-design` - Reviews implementation against technical design

## Skill Metadata

- **Location**: `.github/skills/generate-features-and-technical-designs/`
- **Trigger**: Type `/generate-features-and-technical-designs` in chat
- **Scope**: Workspace-level documentation generation
- **Dependencies**: None (standalone skill)
