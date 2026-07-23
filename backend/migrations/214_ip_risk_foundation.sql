-- IP risk detection foundation.
-- CP1 is shadow-only: these tables collect evidence and assessments, but no
-- migration enables automatic registration blocking or account changes.

-- Keep malformed historical usage-log IP strings from aborting an entire risk
-- scan while remaining compatible with the supported PostgreSQL 14 baseline.
CREATE OR REPLACE FUNCTION ip_risk_try_parse_inet(value TEXT)
RETURNS INET
LANGUAGE plpgsql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $$
BEGIN
    RETURN value::INET;
EXCEPTION
    WHEN invalid_text_representation THEN
        RETURN NULL;
END;
$$;

CREATE TABLE IF NOT EXISTS auth_risk_events (
    id BIGSERIAL PRIMARY KEY,
    dedupe_key VARCHAR(192) NOT NULL UNIQUE,
    event_type VARCHAR(32) NOT NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ip_address INET NOT NULL,
    ip_network CIDR NOT NULL,
    user_agent_summary VARCHAR(160) NOT NULL DEFAULT '',
    user_agent_hmac BYTEA,
    email_pattern_hmac BYTEA,
    email_pattern_template BOOLEAN NOT NULL DEFAULT FALSE,
    invitation_hmac BYTEA,
    affiliate_hmac BYTEA,
    signup_source VARCHAR(32) NOT NULL DEFAULT '',
    request_id VARCHAR(64) NOT NULL DEFAULT '',
    evidence_confidence VARCHAR(16) NOT NULL DEFAULT 'exact',
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT auth_risk_events_type_check
        CHECK (event_type IN ('register', 'successful_login')),
    CONSTRAINT auth_risk_events_confidence_check
        CHECK (evidence_confidence IN ('exact', 'inferred'))
);

CREATE INDEX IF NOT EXISTS idx_auth_risk_events_network_time
    ON auth_risk_events (ip_network, occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_auth_risk_events_ip_time
    ON auth_risk_events (ip_address, occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_auth_risk_events_user_time
    ON auth_risk_events (user_id, occurred_at DESC, id DESC)
    WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_auth_risk_events_register_time
    ON auth_risk_events (occurred_at DESC, id DESC)
    WHERE event_type = 'register';
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_risk_events_register_user
    ON auth_risk_events (user_id)
    WHERE event_type = 'register'
      AND user_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS ip_risk_cases (
    id BIGSERIAL PRIMARY KEY,
    case_key VARCHAR(192) NOT NULL UNIQUE,
    primary_ip INET NOT NULL,
    primary_network CIDR NOT NULL,
    score SMALLINT NOT NULL DEFAULT 0,
    level VARCHAR(16) NOT NULL DEFAULT 'low',
    status VARCHAR(24) NOT NULL DEFAULT 'open',
    evidence_confidence VARCHAR(16) NOT NULL DEFAULT 'exact',
    evidence_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    recommended_actions JSONB NOT NULL DEFAULT '[]'::jsonb,
    auto_block_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    first_detected_at TIMESTAMPTZ NOT NULL,
    last_detected_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_risk_cases_score_check CHECK (score BETWEEN 0 AND 100),
    CONSTRAINT ip_risk_cases_level_check
        CHECK (level IN ('low', 'medium', 'high', 'severe', 'critical')),
    CONSTRAINT ip_risk_cases_status_check
        CHECK (status IN ('open', 'observing', 'processing', 'resolved', 'ignored')),
    CONSTRAINT ip_risk_cases_confidence_check
        CHECK (evidence_confidence IN ('exact', 'inferred', 'mixed'))
);

CREATE INDEX IF NOT EXISTS idx_ip_risk_cases_status_score
    ON ip_risk_cases (status, score DESC, last_detected_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_ip_risk_cases_network
    ON ip_risk_cases (primary_network, last_detected_at DESC);

CREATE TABLE IF NOT EXISTS ip_risk_case_users (
    id BIGSERIAL PRIMARY KEY,
    case_id BIGINT NOT NULL REFERENCES ip_risk_cases(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    relation_type VARCHAR(32) NOT NULL,
    evidence_confidence VARCHAR(16) NOT NULL DEFAULT 'exact',
    evidence_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    recommended_selected BOOLEAN NOT NULL DEFAULT FALSE,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_risk_case_users_case_user_unique UNIQUE (case_id, user_id),
    CONSTRAINT ip_risk_case_users_relation_check
        CHECK (relation_type IN ('suspected_new', 'trusted_existing', 'disabled')),
    CONSTRAINT ip_risk_case_users_confidence_check
        CHECK (evidence_confidence IN ('exact', 'inferred'))
);

CREATE INDEX IF NOT EXISTS idx_ip_risk_case_users_user_case
    ON ip_risk_case_users (user_id, case_id);

CREATE TABLE IF NOT EXISTS ip_risk_policies (
    id BIGSERIAL PRIMARY KEY,
    mode VARCHAR(32) NOT NULL,
    ip_network CIDR,
    exact_ip INET,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_risk_policies_mode_check
        CHECK (mode IN ('allowlist', 'observe', 'shared_network', 'block_registration')),
    CONSTRAINT ip_risk_policies_target_check
        CHECK (ip_network IS NOT NULL OR exact_ip IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_ip_risk_policies_active_network
    ON ip_risk_policies (mode, ip_network, expires_at)
    WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_ip_risk_policies_active_ip
    ON ip_risk_policies (mode, exact_ip, expires_at)
    WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS ip_risk_scans (
    id BIGSERIAL PRIMARY KEY,
    scan_type VARCHAR(24) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    requested_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    range_start TIMESTAMPTZ NOT NULL,
    range_end TIMESTAMPTZ NOT NULL,
    progress SMALLINT NOT NULL DEFAULT 0,
    candidate_count INT NOT NULL DEFAULT 0,
    case_count INT NOT NULL DEFAULT 0,
    inferred_event_count INT NOT NULL DEFAULT 0,
    error_message VARCHAR(1000) NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_risk_scans_type_check
        CHECK (scan_type IN ('incremental', 'reconcile', 'daily', 'manual', 'historical_backfill')),
    CONSTRAINT ip_risk_scans_status_check
        CHECK (status IN ('pending', 'running', 'completed', 'failed', 'canceled')),
    CONSTRAINT ip_risk_scans_progress_check CHECK (progress BETWEEN 0 AND 100),
    CONSTRAINT ip_risk_scans_range_check CHECK (range_end >= range_start)
);

CREATE INDEX IF NOT EXISTS idx_ip_risk_scans_status_created
    ON ip_risk_scans (status, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS ip_risk_actions (
    id BIGSERIAL PRIMARY KEY,
    case_id BIGINT REFERENCES ip_risk_cases(id) ON DELETE SET NULL,
    action_type VARCHAR(40) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    actor_type VARCHAR(16) NOT NULL DEFAULT 'admin',
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason VARCHAR(1000) NOT NULL,
    preview_token_hash BYTEA,
    preview_expires_at TIMESTAMPTZ,
    action_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    rollback_status VARCHAR(24) NOT NULL DEFAULT 'not_requested',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT ip_risk_actions_status_check
        CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed', 'rolled_back')),
    CONSTRAINT ip_risk_actions_actor_check
        CHECK (actor_type IN ('admin', 'system')),
    CONSTRAINT ip_risk_actions_rollback_check
        CHECK (rollback_status IN ('not_requested', 'eligible', 'partial', 'completed', 'conflict', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_ip_risk_actions_case_created
    ON ip_risk_actions (case_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_ip_risk_actions_created
    ON ip_risk_actions (created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS ip_risk_action_items (
    id BIGSERIAL PRIMARY KEY,
    action_id BIGINT NOT NULL REFERENCES ip_risk_actions(id) ON DELETE CASCADE,
    target_type VARCHAR(24) NOT NULL,
    target_id BIGINT,
    target_ip INET,
    before_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    action_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    error_message VARCHAR(1000) NOT NULL DEFAULT '',
    rollback_status VARCHAR(24) NOT NULL DEFAULT 'not_requested',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_risk_action_items_target_check
        CHECK (target_id IS NOT NULL OR target_ip IS NOT NULL),
    CONSTRAINT ip_risk_action_items_type_check
        CHECK (target_type IN ('user', 'api_key', 'ip_policy', 'case')),
    CONSTRAINT ip_risk_action_items_status_check
        CHECK (status IN ('pending', 'completed', 'skipped', 'failed')),
    CONSTRAINT ip_risk_action_items_rollback_check
        CHECK (rollback_status IN ('not_requested', 'eligible', 'completed', 'conflict', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_ip_risk_action_items_action
    ON ip_risk_action_items (action_id, id);
