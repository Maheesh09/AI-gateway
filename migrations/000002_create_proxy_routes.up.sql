CREATE TABLE proxy_routes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    path_pattern    VARCHAR(255) NOT NULL UNIQUE,
    target_url      VARCHAR(500) NOT NULL,
    allowed_methods TEXT[]       NOT NULL DEFAULT '{GET,POST,PUT,DELETE,PATCH}',
    strip_prefix    BOOLEAN      NOT NULL DEFAULT FALSE,
    timeout_ms      INTEGER      NOT NULL DEFAULT 5000,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routes_active ON proxy_routes(is_active) WHERE is_active = TRUE;
