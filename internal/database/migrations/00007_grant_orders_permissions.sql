-- +goose Up
-- The "orders" resource permissions were added to the permissions catalog
-- (00005_create_permissions.sql) but never granted to any role, so
-- config/routes.json's permissions_required: ["orders:read"]/["orders:create"]
-- on /api/orders/* was unreachable by any seeded user. Grant it to admin.
UPDATE roles
SET permissions = permissions || '["orders:read", "orders:create"]'::jsonb
WHERE name = 'admin';

-- +goose Down
UPDATE roles
SET permissions = permissions - 'orders:read' - 'orders:create'
WHERE name = 'admin';
