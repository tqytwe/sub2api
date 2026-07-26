-- Device registration and durable privacy-safe push outbox.

CREATE TABLE IF NOT EXISTS mobile_devices (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id UUID NOT NULL,
    platform VARCHAR(16) NOT NULL,
    push_provider VARCHAR(16) NOT NULL DEFAULT 'fcm',
    token_ciphertext TEXT NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    app_version VARCHAR(64) NOT NULL DEFAULT '',
    locale VARCHAR(32) NOT NULL DEFAULT 'zh-CN',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_seen_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT mobile_devices_platform_check CHECK (platform IN ('android', 'ios', 'web')),
    CONSTRAINT mobile_devices_provider_check CHECK (push_provider = 'fcm'),
    UNIQUE (user_id, installation_id)
);

CREATE INDEX IF NOT EXISTS idx_mobile_devices_token_hash ON mobile_devices (token_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mobile_devices_active_token_unique ON mobile_devices (token_hash) WHERE enabled = true;
CREATE UNIQUE INDEX IF NOT EXISTS idx_mobile_devices_active_installation_unique ON mobile_devices (installation_id) WHERE enabled = true;
CREATE INDEX IF NOT EXISTS idx_mobile_devices_user_enabled_seen ON mobile_devices (user_id, enabled, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_devices_revoked ON mobile_devices (revoked_at);

CREATE TABLE IF NOT EXISTS mobile_push_outbox (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dedupe_key_hash VARCHAR(64) NOT NULL UNIQUE,
    event_type VARCHAR(64) NOT NULL,
    source_type VARCHAR(64) NOT NULL DEFAULT '',
    source_id VARCHAR(128) NOT NULL DEFAULT '',
    title_zh VARCHAR(120) NOT NULL,
    body_zh VARCHAR(300) NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error_code VARCHAR(64),
    available_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
	claim_token UUID,
	lease_expires_at TIMESTAMPTZ,
	CONSTRAINT mobile_push_outbox_status_check CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'skipped')),
    CONSTRAINT mobile_push_outbox_attempts_check CHECK (attempts >= 0)
);

ALTER TABLE mobile_push_outbox ADD COLUMN IF NOT EXISTS claim_token UUID;
ALTER TABLE mobile_push_outbox ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE mobile_push_outbox DROP CONSTRAINT IF EXISTS mobile_push_outbox_status_check;
ALTER TABLE mobile_push_outbox
    ADD CONSTRAINT mobile_push_outbox_status_check
    CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'skipped'));

CREATE INDEX IF NOT EXISTS idx_mobile_push_status_available ON mobile_push_outbox (status, available_at);
CREATE INDEX IF NOT EXISTS idx_mobile_push_user_created ON mobile_push_outbox (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_push_source ON mobile_push_outbox (source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_mobile_push_lease ON mobile_push_outbox (lease_expires_at)
    WHERE status = 'processing';

CREATE TABLE IF NOT EXISTS mobile_push_deliveries (
    id BIGSERIAL PRIMARY KEY,
    outbox_id BIGINT NOT NULL REFERENCES mobile_push_outbox(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES mobile_devices(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error_code VARCHAR(64),
    available_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_push_deliveries_status_check CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    CONSTRAINT mobile_push_deliveries_attempts_check CHECK (attempts >= 0),
    UNIQUE (outbox_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_mobile_push_deliveries_pending
    ON mobile_push_deliveries (outbox_id, status, available_at);
CREATE INDEX IF NOT EXISTS idx_mobile_push_deliveries_device
    ON mobile_push_deliveries (device_id, created_at DESC);
