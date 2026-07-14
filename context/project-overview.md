# Multi-Tenant API Gateway

## 1. Executive Summary

### Purpose

A production-grade API gateway with JWT claims-based authentication (CBAC), role-based authorization (RBAC), multi-tenancy support, rate limiting, caching, and request validation. This project demonstrates backend engineering expertise through implementation of distributed systems patterns, modern authentication/authorization patterns, and scalable architecture.

### Questions

- **What is the project?** A comprehensive API gateway built in Go that provides unified claims-based authentication, role-based authorization, intelligent routing, rate limiting, and multi-tenant isolation for microservices architectures.
- **Who is it for?** Backend engineers, companies building multi-tenant SaaS applications, development teams transitioning to microservices, and hiring managers evaluating distributed systems expertise.
- **What outcome does it create?** A portfolio demonstration of production-grade infrastructure engineering, showcasing mastery of API design, modern authentication (CBAC), authorization (RBAC/ABAC), distributed systems patterns, and operational concerns at scale.

---

## 2. Problem Statement

### Purpose

Microservices architectures create complexity in cross-cutting concerns that should not be duplicated across services.

### Questions

- **What pain point exists today?** Microservices need a unified entry point with consistent authentication. Each service shouldn't re-implement authentication, rate limiting, and validation. Multi-tenant applications need tenant isolation and resource limits per tenant. Services need protection from malformed requests and malicious traffic.
- **Why is the current workflow inefficient?** Duplicating authentication, rate limiting, and validation logic across multiple services creates maintenance burden, inconsistent security policies, and increased attack surface. Without centralized control, managing multi-tenant isolation becomes error-prone.
- **What are users doing instead?** Using heavy commercial solutions like Kong or AWS API Gateway (vendor lock-in, high cost), implementing custom middleware in each service (duplication, inconsistency), or using basic reverse proxies like NGINX without proper multi-tenancy support.

---

## 3. Target Users

### Purpose

Define who benefits from the solution.

### Questions

- **Who will use it?**
  - Backend engineers looking to understand API gateway patterns
  - Companies building multi-tenant SaaS applications
  - Development teams transitioning to microservices architecture
  - As portfolio project: recruiters, hiring managers, technical interviewers
  - As learning tool: developers studying API gateway patterns
- **What are their goals?**
  - Learn production-grade API gateway implementation
  - Evaluate technical depth for hiring decisions
  - Find lightweight alternative to heavy commercial gateways
  - Understand multi-tenancy and distributed systems patterns
- **What frustrations do they experience?**
  - Commercial gateways are too complex or expensive for learning/small projects
  - Lack of clear examples demonstrating security best practices
  - Difficulty understanding JWT vulnerabilities and mitigation strategies
  - Challenge of implementing proper tenant isolation

---

## 4. Value Proposition

### Purpose

Stand out as a backend engineer with deep understanding of distributed systems patterns.

### Questions

- **What benefits does it provide?**
  - Demonstrates production-ready code with security-first mindset
  - Shows understanding of scalability patterns (rate limiting, caching, circuit breakers)
  - Proves ability to handle complex cross-cutting concerns
  - Educational resource for learning API gateway implementation
- **What becomes easier, faster, cheaper, or more reliable?**
  - Easier: Centralized authentication and authorization management
  - Faster: Cached responses reduce downstream service load
  - Cheaper: Lightweight Go implementation with minimal infrastructure requirements
  - More reliable: Circuit breakers, graceful degradation, fail-open rate limiting
- **Why is it better than existing alternatives?**
  - Educational clarity vs. production complexity of Kong/Envoy
  - No vendor lock-in vs. AWS API Gateway
  - Multi-tenancy focus vs. generic reverse proxies
  - Modern security patterns vs. dated examples

---

## 5. Existing Solutions

### Purpose

Understand competitors and position as learning-focused alternative.

### Questions

- **What tools already solve this problem?**
  - Kong: Battle-tested, plugin ecosystem, enterprise features
  - AWS API Gateway: Managed, scalable, integrated with AWS
  - NGINX: Fast, widely adopted, flexible
  - Traefik: Easy setup, Docker-friendly, modern
  - Envoy: High performance, service mesh integration
- **Why are they insufficient?**
  - Too complex for learning (Kong, Envoy)
  - Vendor lock-in and expensive (AWS API Gateway)
  - Low-level configuration burden (NGINX)
  - Immature multi-tenancy support (Traefik)
- **What gap exists in the market?**
  - No clear educational example demonstrating JWT security best practices
  - Limited open-source examples of proper multi-tenant isolation
  - Few projects showing production-ready Go API gateway implementation
  - Gap in portfolio: demonstrating distributed systems expertise

---

## 6. Project Goals

### Purpose

Create a standout backend project demonstrating senior-level engineering skills.

### Questions

- **What should users accomplish?**
  - Understand API gateway architecture and implementation patterns
  - Learn JWT security vulnerabilities and proper mitigation
  - See working example of multi-tenant isolation strategies
  - Reference production-ready code for their own projects
- **What business or learning goals does this support?**
  - **Learning:** Master API gateway patterns, multi-tenancy, distributed system concerns
  - **Portfolio:** Create standout backend project for senior backend roles
  - **Open Source:** Build educational resource for the community
  - **Career Positioning:** Strengthen positioning as backend/systems engineer
  - **Interview Prep:** Have deep technical project to discuss in interviews
- **What outcomes are expected?**
  - Working MVP deployed publicly
  - Comprehensive documentation with architecture diagrams
  - LinkedIn article explaining design decisions
  - GitHub stars showing community interest
  - Successful discussion in technical interviews

---

## 7. Core Features

### Purpose

Demonstrate mastery of distributed systems patterns and security.

### Questions

- **What are the essential features?** JWT authentication with RBAC, multi-tenant routing and isolation, distributed rate limiting, intelligent caching, request validation, observability (logging, metrics, tracing)
- **Which features create the most value?** Multi-tenant isolation (differentiator), JWT security (demonstrates security expertise), distributed rate limiting (shows understanding of scale), graceful degradation (production readiness)
- **What can be excluded initially?** OAuth2 full flow, admin dashboard UI, GraphQL federation, Kubernetes operators, full circuit breaker implementation

### Feature List

**Core Features (MVP)**

- ✅ **JWT Claims-Based Authentication (CBAC):** Token creation and validation with algorithm allowlist
- ✅ **Multi-tenant isolation:** Tenant ID extracted from validated JWT claims (not headers)
- ✅ **Role-Based Authorization (RBAC):** Granular permission system with Admin, Manager, Viewer roles
  - Separate `roles` and `permissions` tables
  - Permission format: `resource:action` (e.g., `users:create`, `billing:read`)
  - Admin: Full CRUD + billing + global settings
  - Manager: Operations + user management - NO billing access
  - Viewer: Read-only access
- ✅ **Attribute-Based routing:** Route decisions using claims (tenant_id, role, permissions)
- ✅ **Request validation:** Schema validation, reject malformed requests
- ✅ **Distributed rate limiting:** Per-tenant using Redis with sliding window algorithm
- ✅ **Intelligent caching:** Redis with tenant-scoped keys for response caching
- ✅ **Retry logic** with exponential backoff
- ✅ **Timeout handling** with deadline propagation
- ✅ **Structured logging** with correlation IDs
- ✅ **Health check endpoint**
- ✅ **OpenAPI documentation**
- ✅ **Docker Compose setup** with mock downstream services

**Advanced Features (Post-MVP)**

- Circuit breakers per downstream service
- Hot tenant detection and dynamic throttling
- JWT signature validation caching
- Distributed tracing with W3C Trace Context
- Prometheus metrics with Grafana dashboards
- Configuration hot reload without dropping requests

---

## 8. User Workflow

### Purpose

Explain request flow through the gateway.

### Questions

- **What is the user's journey?** Client → Gateway (auth, rate limit, route) → Downstream Service → Gateway (cache, log) → Client
- **What steps do they follow?**
  1. Client sends request with JWT token in Authorization header
  2. Gateway validates JWT signature and extracts tenant ID from claims
  3. Gateway checks tenant rate limits (Redis)
  4. Gateway validates request schema
  5. Gateway routes to appropriate downstream service based on path
  6. Gateway caches response (for GET requests) with tenant-scoped key
  7. Gateway returns response with correlation ID in headers
  8. All steps logged with structured logging
- **What happens from start to finish?** Request arrives → Auth check → Rate limit check → Validation → Route → Cache check → Downstream call → Cache store → Response → Metrics recorded → Correlation ID logged

---

## 9. MVP Scope

### Purpose

Ship working gateway within reasonable timeline.

### Questions

- **What must exist in version 1?** Core routing, JWT auth, multi-tenancy, rate limiting, caching, basic observability
- **What can wait until later?** Circuit breakers, advanced metrics, UI dashboard, OAuth2, GraphQL support, Kubernetes deployment
- **Can this be built within the target timeline?** Yes, MVP achievable in 4-6 weeks with focused implementation

### Included

- JWT authentication and validation with security best practices
- Multi-tenant routing with tenant ID from JWT claims
- Role-based access control (Admin, Manager, Viewer)
- Distributed rate limiting using Redis (sliding window algorithm)
- Response caching with tenant-scoped Redis keys
- Request validation and schema checking
- Structured logging with correlation IDs
- Health check and readiness endpoints
- OpenAPI/Swagger documentation
- Docker Compose local development setup
- Two mock downstream services for testing
- Comprehensive README with architecture diagrams

### Excluded

- Full OAuth2 authorization code flow
- Token refresh mechanism
- Admin web dashboard UI
- Prometheus/Grafana integration (metrics exposed but not dashboards)
- WebSocket support
- GraphQL federation
- Full circuit breaker implementation
- Blue-green deployment automation
- Kubernetes manifests and Helm charts
- Load testing suite and benchmarks

---

## 10. Technical Overview

### Purpose

Production-grade Go implementation with Redis for distributed state.

### Questions

- **What technologies will be used?** Go for gateway implementation, Redis for rate limiting and caching, Docker Compose for local development
- **What external services are required?** Redis cluster, downstream microservices (mocked for MVP)
- **How do major components interact?** Gateway receives requests → validates JWT → checks Redis for rate limits → routes to service → caches response in Redis → returns to client

### Stack

- **Language:** Go 1.21+ (chosen for performance, concurrency, single binary deployment)
- **Gateway Framework:** Standard library `net/http` with custom middleware
- **Authentication Model:** CBAC (Claims-Based Access Control) using JWT
- **Authorization Model:** RBAC (Role-Based) with ABAC capabilities (attribute-based routing)
- **JWT Library:** golang-jwt/jwt v5 for JWT validation with algorithm allowlist
- **Database:** PostgreSQL 15+ for tenant, user, and profile data
- **Migrations:** Goose for database schema versioning and migrations
- **Database Driver:** pgx/v5 (high-performance PostgreSQL driver)
- **Distributed State:** Redis 7.0+ / Redis Sentinel for rate limiting and caching
- **Caching:** Redis with tenant-scoped key namespacing
- **Logging:** Structured JSON logging with correlation IDs
- **Documentation:** OpenAPI 3.0 / Swagger UI
- **Containerization:** Docker + Docker Compose
- **Testing:** Go standard testing, httptest for integration tests, testcontainers for DB tests
- **Build:** Makefile + Go modules

---

## 11. Data Model Overview

### Purpose

Identify key entities stored in PostgreSQL, Redis, and JWT claims.

### Questions

- **What data must be stored?** Tenant configurations, user accounts, user profiles, JWT claims, rate limit counters, cached responses
- **How do entities relate to each other?** User belongs to Tenant (many-to-one), User has one Profile (one-to-one), Tenant has rate limit quotas and feature flags

### Database Schema (PostgreSQL)

Default rate limiting is defined in environment variables and can be overridden per tenant in the `tenants` table. Each tenant has its own rate limit configuration, which is enforced by the gateway using Redis counters.

**Roles Table**

Defines the available roles with their permissions. Roles are immutable system-defined entities.

```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL, -- admin, manager, viewer
    display_name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_system_role BOOLEAN NOT NULL DEFAULT true, -- System roles cannot be deleted
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_roles_name ON roles(name);

-- Seed system roles
INSERT INTO roles (name, display_name, description, permissions, is_system_role) VALUES
(
    'admin',
    'Administrator',
    'Full access to all resources including billing, global settings, and user management',
    '[
        "users:create", "users:read", "users:update", "users:delete",
        "tenants:create", "tenants:read", "tenants:update", "tenants:delete",
        "billing:read", "billing:update", "billing:delete",
        "settings:read", "settings:update",
        "roles:read", "roles:assign",
        "audit_logs:read",
        "api_keys:create", "api_keys:read", "api_keys:revoke"
    ]'::jsonb,
    true
),
(
    'manager',
    'Manager',
    'Manage day-to-day operations and team members, but no access to billing or global settings',
    '[
        "users:create", "users:read", "users:update",
        "tenants:read",
        "settings:read",
        "roles:read", "roles:assign",
        "audit_logs:read",
        "api_keys:create", "api_keys:read", "api_keys:revoke"
    ]'::jsonb,
    true
),
(
    'viewer',
    'Viewer',
    'Read-only access to resources, cannot make changes',
    '[
        "users:read",
        "tenants:read",
        "settings:read",
        "roles:read",
        "audit_logs:read"
    ]'::jsonb,
    true
);
```

**Permissions Table**

Defines all available permissions in the system. Permissions follow the format `resource:action`.

```sql
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL, -- Format: "resource:action" (e.g., "users:create")
    resource VARCHAR(50) NOT NULL, -- users, tenants, billing, settings, etc.
    action VARCHAR(50) NOT NULL, -- create, read, update, delete
    description TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_permissions_resource ON permissions(resource);
CREATE INDEX idx_permissions_name ON permissions(name);

-- Seed permissions
INSERT INTO permissions (name, resource, action, description) VALUES
-- User management
('users:create', 'users', 'create', 'Create new users within tenant'),
('users:read', 'users', 'read', 'View user information'),
('users:update', 'users', 'update', 'Update user details and roles'),
('users:delete', 'users', 'delete', 'Delete users from tenant'),

-- Tenant management
('tenants:create', 'tenants', 'create', 'Create new tenants (super admin only)'),
('tenants:read', 'tenants', 'read', 'View tenant information'),
('tenants:update', 'tenants', 'update', 'Update tenant settings and configuration'),
('tenants:delete', 'tenants', 'delete', 'Delete tenant (super admin only)'),

-- Billing
('billing:read', 'billing', 'read', 'View billing information and invoices'),
('billing:update', 'billing', 'update', 'Update payment methods and subscription'),
('billing:delete', 'billing', 'delete', 'Cancel subscription and delete payment methods'),

-- Settings
('settings:read', 'settings', 'read', 'View application settings'),
('settings:update', 'settings', 'update', 'Update application settings and configuration'),

-- Roles
('roles:read', 'roles', 'read', 'View available roles and permissions'),
('roles:assign', 'roles', 'assign', 'Assign roles to users'),

-- Audit logs
('audit_logs:read', 'audit_logs', 'read', 'View audit logs and activity history'),

-- API Keys
('api_keys:create', 'api_keys', 'create', 'Generate new API keys'),
('api_keys:read', 'api_keys', 'read', 'View API keys'),
('api_keys:revoke', 'api_keys', 'revoke', 'Revoke API keys');
```

**Tenants Table**

```sql
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    tier VARCHAR(50) NOT NULL DEFAULT 'free', -- free, professional, enterprise
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
    rate_limit_per_hour INTEGER NOT NULL DEFAULT 1000,
    max_users INTEGER NOT NULL DEFAULT 10,
    features JSONB DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_is_active ON tenants(is_active) WHERE deleted_at IS NULL;
```

**Users Table**

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT unique_email_per_tenant UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_role_id ON users(role_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_is_active ON users(is_active) WHERE deleted_at IS NULL;
```

**Audit Logs Table**

Track all actions for compliance and security.

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL, -- Format: "resource:action" (e.g., "users:update")
    resource_type VARCHAR(50) NOT NULL, -- users, tenants, billing, etc.
    resource_id UUID, -- ID of affected resource
    ip_address INET,
    user_agent TEXT,
    request_id UUID, -- Correlation ID from gateway
    metadata JSONB DEFAULT '{}'::jsonb, -- Additional context
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
```

**Profiles Table**

```sql
CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    avatar_url TEXT,
    timezone VARCHAR(50) DEFAULT 'UTC',
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_profiles_user_id ON profiles(user_id);
```

### Go Data Models

**Following Go Best Practices:**

- Pointer fields for nullable database columns
- `time.Time` for timestamps
- JSON tags for API serialization
- Database tags for sqlx/pgx
- Validation tags for request validation

```go
package models

import (
    "time"
    "github.com/google/uuid"
)

// Role represents a system role with associated permissions
type Role struct {
    ID           uuid.UUID              `json:"id" db:"id"`
    Name         string                 `json:"name" db:"name" validate:"required,oneof=admin manager viewer"`
    DisplayName  string                 `json:"display_name" db:"display_name" validate:"required,min=2,max=100"`
    Description  string                 `json:"description" db:"description" validate:"required"`
    Permissions  []string               `json:"permissions" db:"permissions"` // Array of permission strings
    IsSystemRole bool                   `json:"is_system_role" db:"is_system_role"`
    CreatedAt    time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

// HasPermission checks if the role has a specific permission
func (r *Role) HasPermission(permission string) bool {
    for _, p := range r.Permissions {
        if p == permission {
            return true
        }
    }
    return false
}

// Permission represents a granular permission in the system
type Permission struct {
    ID          uuid.UUID `json:"id" db:"id"`
    Name        string    `json:"name" db:"name" validate:"required,permission"` // Format: "resource:action"
    Resource    string    `json:"resource" db:"resource" validate:"required"`
    Action      string    `json:"action" db:"action" validate:"required,oneof=create read update delete assign revoke"`
    Description string    `json:"description" db:"description" validate:"required"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Tenant represents a multi-tenant organization
type Tenant struct {
    ID                  uuid.UUID              `json:"id" db:"id"`
    Name                string                 `json:"name" db:"name" validate:"required,min=2,max=255"`
    Slug                string                 `json:"slug" db:"slug" validate:"required,min=2,max=100,slug"`
    Tier                string                 `json:"tier" db:"tier" validate:"required,oneof=free professional enterprise"`
    RateLimitPerMinute  int                    `json:"rate_limit_per_minute" db:"rate_limit_per_minute" validate:"required,min=1"`
    RateLimitPerHour    int                    `json:"rate_limit_per_hour" db:"rate_limit_per_hour" validate:"required,min=1"`
    MaxUsers            int                    `json:"max_users" db:"max_users" validate:"required,min=1"`
    Features            map[string]interface{} `json:"features" db:"features"`
    IsActive            bool                   `json:"is_active" db:"is_active"`
    CreatedAt           time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt           time.Time              `json:"updated_at" db:"updated_at"`
    DeletedAt           *time.Time             `json:"deleted_at,omitempty" db:"deleted_at"`
}

// User represents an authenticated user within a tenant
type User struct {
    ID            uuid.UUID  `json:"id" db:"id"`
    TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id" validate:"required"`
    RoleID        uuid.UUID  `json:"role_id" db:"role_id" validate:"required"`
    Email         string     `json:"email" db:"email" validate:"required,email"`
    PasswordHash  string     `json:"-" db:"password_hash"` // Never expose in JSON
    IsActive      bool       `json:"is_active" db:"is_active"`
    EmailVerified bool       `json:"email_verified" db:"email_verified"`
    LastLoginAt   *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
    CreatedAt     time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`

    // Relationships (not stored in DB, populated via JOIN)
    Tenant  *Tenant  `json:"tenant,omitempty" db:"-"`
    Role    *Role    `json:"role,omitempty" db:"-"`
    Profile *Profile `json:"profile,omitempty" db:"-"`
}

// HasPermission checks if user has a specific permission through their role
func (u *User) HasPermission(permission string) bool {
    if u.Role == nil {
        return false
    }
    return u.Role.HasPermission(permission)
}

// IsAdmin checks if user has admin role
func (u *User) IsAdmin() bool {
    return u.Role != nil && u.Role.Name == "admin"
}

// IsManager checks if user has manager role
func (u *User) IsManager() bool {
    return u.Role != nil && u.Role.Name == "manager"
}

// IsViewer checks if user has viewer role
func (u *User) IsViewer() bool {
    return u.Role != nil && u.Role.Name == "viewer"
}

// CanManageUsers checks if user can manage other users
func (u *User) CanManageUsers() bool {
    return u.IsAdmin() || u.IsManager()
}

// AuditLog represents an audit trail entry
type AuditLog struct {
    ID           uuid.UUID              `json:"id" db:"id"`
    TenantID     uuid.UUID              `json:"tenant_id" db:"tenant_id"`
    UserID       *uuid.UUID             `json:"user_id,omitempty" db:"user_id"`
    Action       string                 `json:"action" db:"action" validate:"required"`
    ResourceType string                 `json:"resource_type" db:"resource_type" validate:"required"`
    ResourceID   *uuid.UUID             `json:"resource_id,omitempty" db:"resource_id"`
    IPAddress    string                 `json:"ip_address" db:"ip_address"`
    UserAgent    string                 `json:"user_agent" db:"user_agent"`
    RequestID    uuid.UUID              `json:"request_id" db:"request_id"`
    Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
    CreatedAt    time.Time              `json:"created_at" db:"created_at"`
}

// Profile represents additional user profile information
type Profile struct {
    ID        uuid.UUID              `json:"id" db:"id"`
    UserID    uuid.UUID              `json:"user_id" db:"user_id" validate:"required"`
    FirstName *string                `json:"first_name,omitempty" db:"first_name" validate:"omitempty,min=1,max=100"`
    LastName  *string                `json:"last_name,omitempty" db:"last_name" validate:"omitempty,min=1,max=100"`
    AvatarURL *string                `json:"avatar_url,omitempty" db:"avatar_url" validate:"omitempty,url"`
    Timezone  string                 `json:"timezone" db:"timezone" validate:"required,timezone"`
    Metadata  map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
    CreatedAt time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// TableName methods for explicit table naming (if using an ORM)
func (Role) TableName() string      { return "roles" }
func (Permission) TableName() string { return "permissions" }
func (Tenant) TableName() string    { return "tenants" }
func (User) TableName() string      { return "users" }
func (Profile) TableName() string   { return "profiles" }
func (AuditLog) TableName() string  { return "audit_logs" }
```

**JWT Claims (in token payload)**

```go
package auth

import (
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

// CustomClaims extends jwt.RegisteredClaims with tenant and role information
type CustomClaims struct {
    jwt.RegisteredClaims
    TenantID    uuid.UUID `json:"tenant_id"`    // Tenant isolation
    UserID      uuid.UUID `json:"user_id"`      // User identification
    Role        string    `json:"role"`         // Role name (admin, manager, viewer)
    RoleID      uuid.UUID `json:"role_id"`      // Role UUID
    Permissions []string  `json:"permissions"`  // Flattened permission list for quick checks
    Email       string    `json:"email"`        // User email
}

// HasPermission checks if claims contain a specific permission
func (c *CustomClaims) HasPermission(permission string) bool {
    for _, p := range c.Permissions {
        if p == permission {
            return true
        }
    }
    return false
}

// IsAdmin checks if user has admin role
func (c *CustomClaims) IsAdmin() bool {
    return c.Role == "admin"
}

// IsManager checks if user has manager role
func (c *CustomClaims) IsManager() bool {
    return c.Role == "manager"
}

// IsViewer checks if user has viewer role
func (c *CustomClaims) IsViewer() bool {
    return c.Role == "viewer"
}

// CanAccessBilling checks if user can access billing information
func (c *CustomClaims) CanAccessBilling() bool {
    return c.HasPermission("billing:read") || c.IsAdmin()
}

// CanManageUsers checks if user can manage other users
func (c *CustomClaims) CanManageUsers() bool {
    return c.HasPermission("users:update") || c.IsAdmin() || c.IsManager()
}
```

**Permission Checking Middleware Example**

```go
package middleware

import (
    "net/http"
    "context"
)

// RequirePermission middleware checks if user has required permission
func RequirePermission(permission string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := r.Context().Value("claims").(*auth.CustomClaims)

            if !claims.HasPermission(permission) {
                http.Error(w, `{"error":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// RequireRole middleware checks if user has one of the required roles
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := r.Context().Value("claims").(*auth.CustomClaims)

            hasRole := false
            for _, role := range roles {
                if claims.Role == role {
                    hasRole = true
                    break
                }
            }

            if !hasRole {
                http.Error(w, `{"error":"forbidden","message":"insufficient role"}`, http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

**Role-Based Access Control (RBAC) Permission Matrix**

| Permission              | Admin | Manager | Viewer | Description                          |
| ----------------------- | ----- | ------- | ------ | ------------------------------------ |
| **User Management**     |
| `users:create`          | ✅    | ✅      | ❌     | Create new users within tenant       |
| `users:read`            | ✅    | ✅      | ✅     | View user information and profiles   |
| `users:update`          | ✅    | ✅      | ❌     | Update user details, roles, status   |
| `users:delete`          | ✅    | ❌      | ❌     | Delete users from tenant             |
| **Tenant Management**   |
| `tenants:create`        | ✅    | ❌      | ❌     | Create new tenants (super admin)     |
| `tenants:read`          | ✅    | ✅      | ✅     | View tenant information              |
| `tenants:update`        | ✅    | ❌      | ❌     | Update tenant settings, tier, limits |
| `tenants:delete`        | ✅    | ❌      | ❌     | Delete tenant (super admin)          |
| **Billing**             |
| `billing:read`          | ✅    | ❌      | ❌     | View invoices, payment methods       |
| `billing:update`        | ✅    | ❌      | ❌     | Update payment, subscription tier    |
| `billing:delete`        | ✅    | ❌      | ❌     | Cancel subscription, delete payment  |
| **Settings**            |
| `settings:read`         | ✅    | ✅      | ✅     | View application configuration       |
| `settings:update`       | ✅    | ❌      | ❌     | Update global settings, features     |
| **Roles & Permissions** |
| `roles:read`            | ✅    | ✅      | ✅     | View available roles                 |
| `roles:assign`          | ✅    | ✅      | ❌     | Assign roles to users                |
| **Audit & Compliance**  |
| `audit_logs:read`       | ✅    | ✅      | ✅     | View audit logs, activity history    |
| **API Keys**            |
| `api_keys:create`       | ✅    | ✅      | ❌     | Generate API keys for integrations   |
| `api_keys:read`         | ✅    | ✅      | ✅     | View existing API keys               |
| `api_keys:revoke`       | ✅    | ✅      | ❌     | Revoke/delete API keys               |

**Role Definitions:**

1. **Admin (Administrator)**
   - Full CRUD access to all resources
   - Billing and subscription management
   - Global settings and configuration
   - User management (create, update, delete)
   - Tenant management
   - Complete audit log access
   - **Use Case:** Organization owner, finance team
   - **Restrictions:** None

2. **Manager**
   - Day-to-day operational control
   - User management (create, update, but not delete)
   - Team member role assignment
   - Read-only access to billing
   - API key management
   - **Use Case:** Team leads, project managers
   - **Restrictions:** No billing access, no global settings, no user deletion, no tenant deletion

3. **Viewer**
   - Read-only access to all resources
   - Cannot make any changes
   - View users, settings, audit logs
   - **Use Case:** Stakeholders, auditors, support staff
   - **Restrictions:** No write/update/delete operations

**Permission Hierarchy:**

```
Admin (Full Access)
  ├─ All Manager permissions
  ├─ billing:* (read, update, delete)
  ├─ tenants:* (create, update, delete)
  ├─ settings:update
  └─ users:delete

Manager (Operational Access)
  ├─ All Viewer permissions
  ├─ users:create, users:update
  ├─ roles:assign
  ├─ api_keys:create, api_keys:revoke
  └─ settings:read (but not update)

Viewer (Read-Only Access)
  ├─ users:read
  ├─ tenants:read
  ├─ settings:read
  ├─ roles:read
  ├─ audit_logs:read
  └─ api_keys:read
```

**Redis Keys**

- Rate limit counters: `ratelimit:{tenant_id}:{user_id}:current` and `:previous`
- Cached responses: `cache:{tenant_id}:{method}:{path}:{query_hash}`
- JWT blacklist: `jwt:blacklist:{jti}` (for token revocation)
- Session data: `session:{tenant_id}:{user_id}:{session_id}`
- Permission cache: `permissions:{role_id}` (cached role permissions, TTL 5 minutes)

**Configuration (in-memory/file)**

- Routes (path, method, upstream service URL, auth required, permissions required)
- JWT signing keys (with key ID for rotation)
- Feature flags per tenant tier
- Role-permission mappings (loaded from database at startup)

---

## 12. Risks & Challenges

### Purpose

Identify and mitigate critical security and scalability risks.

### Questions

- **What assumptions might be wrong?**
  - Redis always available (mitigation: fail-open with alerts)
  - JWT libraries secure by default (mitigation: explicit algorithm validation)
  - Tenant ID can be trusted from headers (mitigation: extract only from JWT claims)
- **What technical challenges exist?**
  - **JWT algorithm confusion attacks:** Must explicitly validate algorithm type, reject alg=none
  - **Distributed rate limiter accuracy:** Sliding window has 0.003% error rate at boundaries
  - **Redis as SPOF:** Redis outage breaks rate limiting and caching
  - **Hot tenant monopolization:** One tenant's traffic spike degrades all tenants
  - **Cache poisoning:** Malicious upstream responses cached for other tenants
- **What external dependencies could fail?**
  - Redis cluster failure → fail-open with local in-memory fallback
  - Downstream service degradation → circuit breaker and graceful degradation
  - Token storm at mass expiry → jittered TTLs and refresh-ahead logic

---

## 13. Unknowns & Open Questions

- **JWT key rotation strategy:** How to rotate signing keys without downtime? (Solution: Multiple active keys with key ID)
- **Token revocation:** How to revoke JWTs without central state? (Solution: Short-lived tokens + Redis blacklist for critical cases)
- **Multi-region deployment:** How to handle Redis state across regions? (Future: Regional Redis clusters with eventual consistency)
- **Performance at scale:** What is maximum throughput per gateway instance? (Need: Load testing and profiling)
- **Cost model:** How do Redis operations and egress fees scale with tenant count? (Need: Cost attribution tracking)
- **Service discovery:** Static config vs Consul vs Kubernetes DNS? (MVP: Static config, future: dynamic discovery)
- **WebSocket support:** How to handle long-lived connections? (Future: Separate WebSocket gateway)
- **Configuration reload:** How to update routes without dropping requests? (Solution: Graceful connection draining + atomic config swap)

---

## 14. Key Learnings & Research

### Security Best Practices

- **Claims-based authentication:** Extract all identity/tenant information from validated JWT claims only
- **Algorithm validation:** Reject JWT tokens with `alg=none`, validate algorithm matches expected type (prevent HMAC/RSA confusion)
- **Tenant isolation:** Extract tenant ID exclusively from validated JWT claims, never from headers (prevent tenant spoofing)
- **Short-lived tokens:** Use 5-15 minute access tokens with separate refresh tokens
- **Role-based authorization:** Enforce role checks (admin/manager/viewer) from validated claims
- **Attribute-based routing:** Use claims attributes (tenant_id, role, email domain) for routing decisions
- **Granular permissions:** Use `resource:action` format (e.g., `users:create`, `billing:read`) for fine-grained access control
- **Permission caching:** Cache role permissions in JWT claims to avoid database lookups on every request
- **Audit logging:** Track all permission checks and access attempts with correlation IDs

### RBAC Implementation Patterns

- **Separate roles table:** Store roles with immutable system definitions and permission arrays
- **Permission inheritance:** Viewer permissions ⊂ Manager permissions ⊂ Admin permissions
- **Flattened permissions in JWT:** Include permission array in JWT claims for O(1) permission checks
- **Middleware-based authorization:** Use composable permission and role middleware for route protection
- **Permission format standardization:** Consistent `resource:action` naming (users:create, billing:read, settings:update)
- **Role assignment restrictions:** Managers can assign roles but cannot escalate to admin
- **Billing access separation:** Explicit billing permissions, not included in manager role
- **Read-only enforcement:** Viewer role has zero write permissions for true read-only access

### Rate Limiting Patterns

- **Token bucket algorithm:** Standard approach with refill rate and burst capacity
- **Sliding window approximation:** Cloudflare's approach with 0.003% error rate
- **Four-tier strategy (Stripe):** Request rate → Concurrent → Fleet usage → Worker utilization
- **Fail-open approach:** If Redis fails, allow requests through with monitoring

### Multi-Tenancy Patterns

- **Tenant isolation:** Namespace all Redis keys with tenant ID
- **Hot tenant detection:** Monitor per-tenant traffic and dynamically adjust limits
- **Resource quotas:** Per-tenant connection pool limits, rate limits, cache quotas
- **Cost attribution:** Track resource consumption per tenant for fair usage enforcement

### Production Readiness

- Structured logging with correlation IDs
- Graceful shutdown with connection draining
- Health checks and readiness probes
- Distributed tracing with W3C Trace Context
- RED metrics (Request rate, Error rate, Duration) per tenant per endpoint

---

## 15. Success Criteria

- ✅ Working MVP with all core features implemented
- ✅ Deployed publicly (Railway, Render, or DigitalOcean)
- ✅ Comprehensive README with architecture diagrams
- ✅ OpenAPI documentation accessible via Swagger UI
- ✅ Security best practices documented (JWT vulnerabilities and mitigations)
- ✅ LinkedIn article explaining design decisions and learnings
- ✅ GitHub repository with clean commit history
- 🎯 10+ GitHub stars showing community interest
- 🎯 Successfully discussed in technical interview
- 🎯 Code quality: well-structured, tested, production-grade

---

## 16. References & Resources

### Architecture Decision Records (ADRs)

- [ADR-001: CBAC with RBAC for Authentication and Authorization](decisions/001-cbac-rbac-hybrid-auth.md)
- [ADR-002: Redis for Distributed Rate Limiting and Caching](decisions/002-redis-distributed-state.md)
- [ADR-003: Extract Tenant ID from JWT Claims Only](decisions/003-tenant-isolation-from-jwt.md)
- [ADR-004: Use Go for API Gateway Implementation](decisions/004-go-implementation.md)
- [ADR-005: Sliding Window Algorithm for Rate Limiting](decisions/005-sliding-window-rate-limiting.md)

### Architecture Patterns

- [API Gateway Pattern - microservices.io](https://microservices.io/patterns/apigateway.html)
- [AWS Multi-tenant SaaS Architecture](https://docs.aws.amazon.com/whitepapers/latest/saas-architecture-fundamentals/)
- [Microsoft Azure Multi-tenant Guide](https://learn.microsoft.com/en-us/azure/architecture/guide/multitenant/overview)

### Implementation Guides

- [Stripe: Scaling your API with rate limiters](https://stripe.com/blog/rate-limiters)
- [Cloudflare: Rate limiting at scale](https://blog.cloudflare.com/counting-things-a-lot-of-different-things/)
- [Netflix: API Gateway Redesign (BFF pattern)](http://techblog.netflix.com/2012/07/embracing-differences-inside-netflix.html)

### Security

- [Auth0: Critical JWT vulnerabilities](https://auth0.com/blog/critical-vulnerabilities-in-json-web-token-libraries/)
- [OWASP JWT Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)

### Knowledge Base

- [RBAC vs ABAC vs PBAC: Access Control Models](../../knowledge/02-Software%20Engineering/Security/rbac-abac-pbac-access-control.md)
- [JWT Security Vulnerabilities](../../knowledge/02-Software%20Engineering/Security/jwt-security-vulnerabilities.md)
- [Rate Limiting Patterns](../../knowledge/02-Software%20Engineering/Architecture/rate-limiting-patterns.md)
- [Multi-Tenant Architecture](../../knowledge/02-Software%20Engineering/Architecture/multi-tenant-architecture.md)
- [API Gateway Patterns](../../knowledge/02-Software%20Engineering/Architecture/api-gateway-patterns.md)
