-- 175_play_campaigns.sql
-- Sprint C: limited-time play campaigns.

CREATE TABLE IF NOT EXISTS play_campaigns (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    start_at   TIMESTAMPTZ NOT NULL,
    end_at     TIMESTAMPTZ NOT NULL,
    rules_json JSONB NOT NULL DEFAULT '{}',
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_play_campaigns_window CHECK (end_at > start_at)
);

CREATE INDEX IF NOT EXISTS idx_play_campaigns_active_window
    ON play_campaigns(enabled, start_at, end_at);

COMMENT ON TABLE play_campaigns IS 'Limited-time play campaigns; rules_json drives perk overlays';

INSERT INTO settings (key, value)
VALUES ('play_campaigns_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
