-- +goose Up
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_system_role BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_roles_name ON roles(name);

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

-- +goose Down
DROP TABLE IF EXISTS roles;
