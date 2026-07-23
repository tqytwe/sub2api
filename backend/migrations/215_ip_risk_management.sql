-- IP risk management workbench, policy configuration and action safeguards.

CREATE TABLE IF NOT EXISTS ip_risk_config (
    id SMALLINT PRIMARY KEY DEFAULT 1,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ip_risk_config_singleton CHECK (id = 1)
);

INSERT INTO ip_risk_config (id, config)
VALUES (
    1,
    '{
      "registration_10m_threshold": 3,
      "registration_10m_score": 25,
      "registration_1h_threshold": 5,
      "registration_1h_score": 35,
      "registration_24h_threshold": 10,
      "registration_24h_score": 45,
      "shared_ua_3_threshold": 3,
      "shared_ua_3_score": 15,
      "shared_ua_5_threshold": 5,
      "shared_ua_5_score": 20,
      "email_pattern_threshold": 3,
      "email_pattern_score": 15,
      "shared_api_ip_threshold": 3,
      "shared_api_ip_score": 25,
      "rapid_behavior_threshold": 2,
      "rapid_behavior_score": 15,
      "shared_signup_code_threshold": 3,
      "shared_signup_code_score": 10,
      "trusted_account_score": -15,
      "auto_block_score": 90,
      "auto_block_min_registrations": 5,
      "auto_block_duration_minutes": 30,
      "auto_block_enabled": false,
      "historical_backfill_enabled": false,
      "event_retention_days": 90,
      "case_retention_days": 365
    }'::jsonb
)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE ip_risk_actions
    ADD COLUMN IF NOT EXISTS case_version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE ip_risk_actions
    ADD COLUMN IF NOT EXISTS rollback_of_action_id BIGINT
        REFERENCES ip_risk_actions(id) ON DELETE SET NULL;

ALTER TABLE ip_risk_cases
    ADD COLUMN IF NOT EXISTS last_notified_level VARCHAR(16);

ALTER TABLE ip_risk_cases
    ADD COLUMN IF NOT EXISTS last_notified_at TIMESTAMPTZ;

ALTER TABLE ip_risk_policies
    ADD COLUMN IF NOT EXISTS source_action_id BIGINT
        REFERENCES ip_risk_actions(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ip_risk_actions_preview_token
    ON ip_risk_actions (preview_token_hash)
    WHERE preview_token_hash IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ip_risk_actions_rollback_once
    ON ip_risk_actions (rollback_of_action_id)
    WHERE rollback_of_action_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ip_risk_policies_source_action
    ON ip_risk_policies (source_action_id)
    WHERE source_action_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ip_risk_action_items_target
    ON ip_risk_action_items (target_type, target_id, created_at DESC)
    WHERE target_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ip_risk_cases_level_status
    ON ip_risk_cases (level, status, last_detected_at DESC, id DESC);
