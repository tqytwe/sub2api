-- Durable recovery for OpenAI Live / Realtime response.done billing.
-- The rows contain only a billing snapshot; no transcript, audio frame,
-- upstream response identifier, credential, or attestation is persisted.
CREATE TABLE IF NOT EXISTS live_usage_settlement_outbox (
    id BIGSERIAL PRIMARY KEY,
    call_hash VARCHAR(128) NOT NULL UNIQUE,
    account_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    subscription_id BIGINT NOT NULL DEFAULT 0,
    lease_id VARCHAR(128) NOT NULL,
    model VARCHAR(512) NOT NULL,
    call_created_at TIMESTAMPTZ NOT NULL,
    call_expires_at TIMESTAMPTZ NOT NULL,
    inbound_endpoint VARCHAR(512) NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address VARCHAR(128) NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    input_audio_tokens INTEGER NOT NULL DEFAULT 0,
    image_input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    output_audio_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_audio_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_audio_tokens INTEGER NOT NULL DEFAULT 0,
    image_output_tokens INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'closing',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    claimed_by VARCHAR(128),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT live_usage_settlement_outbox_status_check CHECK (status IN ('closing', 'ready'))
);

CREATE INDEX IF NOT EXISTS idx_live_usage_settlement_outbox_ready
    ON live_usage_settlement_outbox (status, available_at, id)
    WHERE status = 'ready';

CREATE INDEX IF NOT EXISTS idx_live_usage_settlement_outbox_closing
    ON live_usage_settlement_outbox (status, call_expires_at, id)
    WHERE status = 'closing';
