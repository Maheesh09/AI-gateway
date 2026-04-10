CREATE TYPE alert_severity AS ENUM ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL');

CREATE TABLE request_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id  UUID         REFERENCES api_keys(id) ON DELETE SET NULL,
    route_id    UUID         REFERENCES proxy_routes(id) ON DELETE SET NULL,
    method      VARCHAR(10)  NOT NULL,
    path        TEXT         NOT NULL,
    status_code INTEGER      NOT NULL,
    latency_ms  INTEGER      NOT NULL,
    ip_address  INET         NOT NULL,
    ai_flagged  BOOLEAN      NOT NULL DEFAULT FALSE,
    ai_reason   TEXT,
    timestamp   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_logs_key_time  ON request_logs(api_key_id, timestamp DESC);
CREATE INDEX idx_logs_timestamp ON request_logs(timestamp DESC);
CREATE INDEX idx_logs_flagged   ON request_logs(ai_flagged) WHERE ai_flagged = TRUE;

CREATE TABLE anomaly_alerts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id     UUID           NOT NULL REFERENCES api_keys(id),
    severity       alert_severity NOT NULL,
    trigger_type   VARCHAR(50)    NOT NULL,
    ai_explanation TEXT,
    auto_blocked   BOOLEAN        NOT NULL DEFAULT FALSE,
    resolved_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alerts_key      ON anomaly_alerts(api_key_id);
CREATE INDEX idx_alerts_severity ON anomaly_alerts(severity);
