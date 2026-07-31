-- Preserve safe installation context and an append-only administrative status history.
ALTER TABLE mobile_feedback
    ADD COLUMN IF NOT EXISTS installation_id VARCHAR(160) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS app_channel VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS install_referrer VARCHAR(256) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_installation_id
    ON mobile_feedback (installation_id, created_at DESC, id DESC)
    WHERE installation_id <> '';

CREATE TABLE IF NOT EXISTS mobile_feedback_admin_audits (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES mobile_feedback(id) ON DELETE CASCADE,
    actor_admin_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    from_status VARCHAR(32) NOT NULL DEFAULT '',
    to_status VARCHAR(32) NOT NULL DEFAULT '',
    from_admin_note TEXT NOT NULL DEFAULT '',
    to_admin_note TEXT NOT NULL DEFAULT '',
    from_version BIGINT NOT NULL,
    to_version BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_admin_audits_feedback_created
    ON mobile_feedback_admin_audits (feedback_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mobile_feedback_admin_audits_actor_created
    ON mobile_feedback_admin_audits (actor_admin_id, created_at DESC, id DESC);
