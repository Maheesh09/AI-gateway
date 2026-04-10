CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    owner_id        VARCHAR(100) NOT NULL,
    key_hash        VARCHAR(64)  NOT NULL UNIQUE,
    scopes          TEXT[]       NOT NULL DEFAULT '{}',
    rate_limit_rpm  INTEGER      NOT NULL DEFAULT 60,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_owner ON api_keys(owner_id);
CREATE INDEX idx_api_keys_hash  ON api_keys(key_hash);
