-- +goose Up
-- Replaces users.tenant_id/users.role_id (FEAT-012): tenant membership and
-- per-tenant role assignment become a proper many-to-many, since a user may
-- now belong to more than one tenant.
CREATE TABLE tenant_users (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

CREATE INDEX idx_tenant_users_user_id ON tenant_users(user_id);
CREATE INDEX idx_tenant_users_role_id ON tenant_users(role_id);

-- +goose Down
DROP TABLE IF EXISTS tenant_users;
