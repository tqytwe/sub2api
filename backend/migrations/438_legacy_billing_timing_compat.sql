-- The v2.23.1 branch retains runtime readers for queue timing and exact billing
-- quotes, while some long-lived databases no longer carry the original 321/322
-- migration files in their source bundle. Restore only the missing schema pieces
-- with forward-compatible guards; existing production data is never rewritten.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS queue_wait_ms INTEGER;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS queue_timing JSONB;
COMMENT ON COLUMN usage_logs.queue_timing IS
'Measured server/forward timing with source and scope; NULL means unknown; never client E2E';

CREATE TABLE IF NOT EXISTS usage_billing_quotes (
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    quoted_actual_cost NUMERIC(20,10) NOT NULL,
    canonical_actual_cost NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);
