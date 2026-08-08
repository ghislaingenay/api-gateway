-- +goose Up
-- Replaces roles.permissions (a denormalized JSONB snapshot with no
-- foreign-key integrity against the permissions table it duplicates,
-- FEAT-012) with a normalized join table.
CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id),
    permission_id UUID NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_permission_id ON role_permissions(permission_id);

-- No backfill here: cmd/seed populates role_permissions directly for the
-- roles it seeds, rather than this migration deriving rows from the JSONB
-- column being dropped below.
ALTER TABLE roles DROP COLUMN permissions;

-- +goose Down
ALTER TABLE roles ADD COLUMN permissions JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE roles r
SET permissions = COALESCE((
    SELECT jsonb_agg(p.name)
    FROM role_permissions rp
    JOIN permissions p ON p.id = rp.permission_id
    WHERE rp.role_id = r.id
), '[]'::jsonb);

DROP TABLE IF EXISTS role_permissions;
