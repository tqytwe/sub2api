-- Count successful public registrations in the same transaction as account creation.
-- Deletion does not refund the quota. Existing accounts are not backfilled.
CREATE TABLE IF NOT EXISTS registration_ip_quotas (
    ip_hash VARCHAR(64) PRIMARY KEY CHECK (length(ip_hash) = 64),
    account_count SMALLINT NOT NULL CHECK (account_count BETWEEN 1 AND 2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
