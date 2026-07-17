-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    tier VARCHAR(50) NOT NULL DEFAULT 'free',
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

-- +goose Down
DROP TABLE IF EXISTS tenants;
