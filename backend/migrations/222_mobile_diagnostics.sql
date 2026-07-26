-- Privacy-safe mobile network and crash diagnostics. No request or response
-- content, credentials, complete URLs or chat text may be stored here.

CREATE TABLE IF NOT EXISTS mobile_diagnostics (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id UUID,
    operation VARCHAR(32) NOT NULL,
    category VARCHAR(32) NOT NULL,
    path VARCHAR(256) NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    network_type VARCHAR(16) NOT NULL DEFAULT 'unknown',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    app_version VARCHAR(64) NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_diagnostics_operation_check CHECK (operation IN ('sync','chat','image','file','payment','support','other')),
    CONSTRAINT mobile_diagnostics_category_check CHECK (category IN ('network','timeout','http','server','client','cancelled','other')),
    CONSTRAINT mobile_diagnostics_network_check CHECK (network_type IN ('wifi','cellular','ethernet','offline','unknown')),
    CONSTRAINT mobile_diagnostics_status_check CHECK (status_code BETWEEN 0 AND 599),
    CONSTRAINT mobile_diagnostics_duration_check CHECK (duration_ms BETWEEN 0 AND 600000),
    CONSTRAINT mobile_diagnostics_retry_check CHECK (retry_count BETWEEN 0 AND 20)
);

CREATE INDEX IF NOT EXISTS idx_mobile_diagnostics_user_occurred ON mobile_diagnostics (user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_diagnostics_category_occurred ON mobile_diagnostics (category, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_diagnostics_operation_occurred ON mobile_diagnostics (operation, occurred_at DESC);
