-- +goose Up
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_permissions_name ON permissions(name);
CREATE INDEX idx_permissions_resource ON permissions(resource);

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

-- +goose Down
DROP TABLE IF EXISTS permissions;
