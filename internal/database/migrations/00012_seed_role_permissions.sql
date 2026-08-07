-- +goose Up
-- Explicit grants replacing the roles.permissions JSONB dropped in
-- 00008_create_role_permissions.sql. Each role's permission list here
-- matches exactly what 00002_create_roles.sql/00007_grant_orders_permissions.sql
-- previously encoded as JSONB, so this is a like-for-like migration of the
-- role catalog's data, not a design change.
--
-- "owner" (FEAT-012): the automatic role granted to whoever creates a
-- tenant via POST /onboarding. Full access within that tenant, same grant
-- set as admin.
INSERT INTO roles (name, display_name, description, is_system_role)
VALUES (
    'owner',
    'Owner',
    'Full access within a tenant the caller created via onboarding; granted automatically, never assigned manually',
    true
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT (SELECT id FROM roles WHERE name = 'admin'), id FROM permissions
WHERE name IN (
    'users:create', 'users:read', 'users:update', 'users:delete',
    'tenants:create', 'tenants:read', 'tenants:update', 'tenants:delete',
    'billing:read', 'billing:update', 'billing:delete',
    'settings:read', 'settings:update',
    'roles:read', 'roles:assign',
    'permissions:read',
    'audit_logs:read',
    'orders:read', 'orders:create'
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT (SELECT id FROM roles WHERE name = 'owner'), id FROM permissions
WHERE name IN (
    'users:create', 'users:read', 'users:update', 'users:delete',
    'tenants:create', 'tenants:read', 'tenants:update', 'tenants:delete',
    'billing:read', 'billing:update', 'billing:delete',
    'settings:read', 'settings:update',
    'roles:read', 'roles:assign',
    'permissions:read',
    'audit_logs:read',
    'orders:read', 'orders:create'
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT (SELECT id FROM roles WHERE name = 'manager'), id FROM permissions
WHERE name IN (
    'users:create', 'users:read', 'users:update',
    'tenants:read',
    'settings:read',
    'roles:read', 'roles:assign',
    'audit_logs:read'
);

INSERT INTO role_permissions (role_id, permission_id)
SELECT (SELECT id FROM roles WHERE name = 'viewer'), id FROM permissions
WHERE name IN (
    'users:read',
    'tenants:read',
    'settings:read',
    'roles:read',
    'audit_logs:read'
);

-- +goose Down
DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE name IN ('admin', 'owner', 'manager', 'viewer'));
DELETE FROM roles WHERE name = 'owner';
