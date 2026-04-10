DROP INDEX IF EXISTS idx_alerts_severity;
DROP INDEX IF EXISTS idx_alerts_key;
DROP TABLE IF EXISTS anomaly_alerts;
DROP INDEX IF EXISTS idx_logs_flagged;
DROP INDEX IF EXISTS idx_logs_timestamp;
DROP INDEX IF EXISTS idx_logs_key_time;
DROP TABLE IF EXISTS request_logs;
DROP TYPE IF EXISTS alert_severity;
