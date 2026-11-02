-- V2 Candy probes are explicitly configured and disabled by default.
ALTER TABLE channel_monitor_v2_config
    ADD COLUMN IF NOT EXISTS candy_probes JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS candy_history_limit INTEGER NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS card_timeline_mode TEXT NOT NULL DEFAULT 'windows',
    ADD COLUMN IF NOT EXISTS group_order BIGINT[] NOT NULL DEFAULT '{}';

DO $$ BEGIN
    ALTER TABLE channel_monitor_v2_config
        ADD CONSTRAINT channel_monitor_v2_candy_history_limit_check
        CHECK (candy_history_limit BETWEEN 15 AND 100);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    ALTER TABLE channel_monitor_v2_config
        ADD CONSTRAINT channel_monitor_v2_card_timeline_mode_check
        CHECK (card_timeline_mode IN ('windows', 'samples'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS channel_monitor_v2_candy_results (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    config_key TEXT NOT NULL,
    schedule_slot TIMESTAMPTZ NOT NULL,
    config_version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'finished')),
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    verdict TEXT NOT NULL DEFAULT 'error',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    answer_preview TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    UNIQUE (group_id, config_key, schedule_slot)
);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_v2_candy_history
    ON channel_monitor_v2_candy_results (group_id, config_key, checked_at DESC)
    WHERE status = 'finished';

CREATE INDEX IF NOT EXISTS idx_channel_monitor_v2_candy_retention
    ON channel_monitor_v2_candy_results (checked_at)
    WHERE status = 'finished';
