-- Persist the reason a selected OpenAI API Key was opened as an OAuth fallback.
-- The event key makes repeated worker evaluations idempotent per policy revision.
CREATE TABLE IF NOT EXISTS openai_apikey_fallback_state (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    config_revision TEXT NOT NULL,
    trigger_reason VARCHAR(32) NOT NULL CHECK (trigger_reason IN ('oauth_unavailable', 'metric_below_threshold')),
    original_schedulable BOOLEAN NOT NULL DEFAULT FALSE,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS openai_apikey_fallback_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    config_revision TEXT NOT NULL,
    trigger_reason VARCHAR(32) NOT NULL CHECK (trigger_reason IN ('oauth_unavailable', 'metric_below_threshold')),
    action VARCHAR(16) NOT NULL CHECK (action IN ('opened', 'already_open')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, config_revision, trigger_reason)
);

CREATE INDEX IF NOT EXISTS idx_openai_apikey_fallback_events_recent
    ON openai_apikey_fallback_events (id DESC);
CREATE INDEX IF NOT EXISTS idx_openai_apikey_fallback_state_opened
    ON openai_apikey_fallback_state (opened_at DESC);
