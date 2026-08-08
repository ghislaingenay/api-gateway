-- +goose Up
-- Keycloak becomes the sole source of credentials (FEAT-012): users no
-- longer carries a password, tenant scoping, or login-lifecycle columns —
-- those move to tenant_users (00009) or are dropped outright. A users row
-- now identifies one person, keyed by the Keycloak sub claim, independent
-- of any tenant.
ALTER TABLE users
    DROP CONSTRAINT unique_email_per_tenant,
    DROP COLUMN tenant_id,
    DROP COLUMN role_id,
    DROP COLUMN password_hash,
    DROP COLUMN is_active,
    DROP COLUMN email_verified,
    DROP COLUMN last_login_at,
    DROP COLUMN deleted_at,
    ADD COLUMN keycloak_sub VARCHAR(255) UNIQUE NOT NULL;

-- +goose Down
ALTER TABLE users
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    ADD COLUMN role_id UUID REFERENCES roles(id),
    ADD COLUMN password_hash VARCHAR(255),
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN last_login_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE,
    DROP COLUMN keycloak_sub;

CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_role_id ON users(role_id);
CREATE INDEX idx_users_is_active ON users(is_active) WHERE deleted_at IS NULL;
