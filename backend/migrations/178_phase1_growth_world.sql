-- 178_phase1_growth_world.sql
-- Phase 1: Image Studio jobs, daily quests, arena period type.

CREATE TABLE IF NOT EXISTS image_studio_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    template_id     TEXT NOT NULL,
    prompt_hash     TEXT NOT NULL DEFAULT '',
    size            TEXT NOT NULL,
    count           INT NOT NULL DEFAULT 1,
    status          TEXT NOT NULL DEFAULT 'pending',
    estimated_cost  DECIMAL(20, 8),
    actual_cost     DECIMAL(20, 8),
    api_key_id      BIGINT,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    CONSTRAINT chk_image_studio_jobs_status CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_image_studio_jobs_user_created
    ON image_studio_jobs(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS image_studio_assets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID NOT NULL REFERENCES image_studio_jobs(id) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_image_studio_assets_job
    ON image_studio_assets(job_id, sort_order);

COMMENT ON TABLE image_studio_jobs IS 'Image Studio generation jobs for logged-in users';
COMMENT ON TABLE image_studio_assets IS 'Generated image URLs belonging to a studio job';

CREATE TABLE IF NOT EXISTS play_quest_progress (
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    quest_date     DATE NOT NULL,
    quest_key      TEXT NOT NULL,
    completed      BOOLEAN NOT NULL DEFAULT false,
    completed_at   TIMESTAMPTZ,
    reward_claimed BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, quest_date, quest_key)
);

CREATE INDEX IF NOT EXISTS idx_play_quest_progress_date
    ON play_quest_progress(quest_date, quest_key);

COMMENT ON TABLE play_quest_progress IS 'Daily cross-feature quest completion tracking';

ALTER TABLE play_arena_periods
    ADD COLUMN IF NOT EXISTS period_type TEXT NOT NULL DEFAULT 'monthly';

INSERT INTO settings (key, value)
VALUES
    ('image_studio_enabled', 'true'),
    ('play_daily_quests_enabled', 'true'),
    ('play_daily_arena_enabled', 'true'),
    ('play_daily_quests', '[{"key":"checkin","energy":10,"auto":true},{"key":"image_generate","energy":20,"min_count":1},{"key":"api_call","energy":15,"min_tokens":100}]'),
    ('play_daily_arena_top_rewards', '[{"rank_max":1,"amount":0.5},{"rank_max":3,"amount":0.2},{"rank_max":10,"amount":0.1}]')
ON CONFLICT (key) DO NOTHING;
