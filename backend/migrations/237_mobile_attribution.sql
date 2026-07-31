-- Anonymous APP acquisition attribution and event funnel.
-- Raw attribution and idempotency tokens are intentionally never persisted.
CREATE TABLE IF NOT EXISTS mobile_installations (
    installation_id UUID PRIMARY KEY,
    user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    platform VARCHAR(16) NOT NULL,
    app_version VARCHAR(64) NOT NULL DEFAULT '',
    locale VARCHAR(32) NOT NULL DEFAULT '',
    campaign_id BIGINT NULL REFERENCES play_campaigns(id) ON DELETE SET NULL,
    referrer_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    attribution_digest BYTEA NULL,
    attribution_verified_at TIMESTAMPTZ NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_installations_platform_check CHECK (platform IN ('android', 'ios', 'web'))
);

CREATE INDEX IF NOT EXISTS idx_mobile_installations_user_seen
    ON mobile_installations(user_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_installations_campaign_seen
    ON mobile_installations(campaign_id, first_seen_at DESC);

CREATE TABLE IF NOT EXISTS mobile_attribution_events (
    id BIGSERIAL PRIMARY KEY,
    installation_id UUID NOT NULL REFERENCES mobile_installations(installation_id) ON DELETE CASCADE,
    user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    event_type VARCHAR(16) NOT NULL,
    idempotency_key_digest BYTEA NOT NULL,
    attribution_digest BYTEA NULL,
    campaign_id BIGINT NULL REFERENCES play_campaigns(id) ON DELETE SET NULL,
    referrer_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    platform VARCHAR(16) NOT NULL,
    app_version VARCHAR(64) NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	verified BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mobile_attribution_events_type_check CHECK (event_type IN ('download','click','open','register','login','active','share')),
    CONSTRAINT mobile_attribution_events_platform_check CHECK (platform IN ('android', 'ios', 'web')),
    CONSTRAINT mobile_attribution_events_idempotency_unique UNIQUE (idempotency_key_digest)
);

CREATE INDEX IF NOT EXISTS idx_mobile_attribution_events_type_time
    ON mobile_attribution_events(event_type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_attribution_events_campaign_time
    ON mobile_attribution_events(campaign_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_attribution_events_installation_time
    ON mobile_attribution_events(installation_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_attribution_events_verified_type_time
    ON mobile_attribution_events(verified, event_type, occurred_at DESC);
