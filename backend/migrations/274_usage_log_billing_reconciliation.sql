SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Forward-only identity. Historical rows remain unbound and are never adopted.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS billing_request_fingerprint TEXT,
    ADD COLUMN IF NOT EXISTS billing_settled BOOLEAN NOT NULL DEFAULT FALSE;
