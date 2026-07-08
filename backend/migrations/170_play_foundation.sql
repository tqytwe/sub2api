-- 170_play_foundation.sql
-- Play / engagement: daily check-in, reward ledger, arena period scaffolding.

CREATE TABLE IF NOT EXISTS play_checkins (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    reward_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_checkins_user_date UNIQUE (user_id, checkin_date)
);

CREATE INDEX IF NOT EXISTS idx_play_checkins_user_id ON play_checkins(user_id);
CREATE INDEX IF NOT EXISTS idx_play_checkins_date ON play_checkins(checkin_date);

COMMENT ON TABLE play_checkins IS 'Daily user check-in records';

CREATE TABLE IF NOT EXISTS play_reward_ledger (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source           VARCHAR(32) NOT NULL,
    amount           DECIMAL(20, 8) NOT NULL,
    idempotency_key  VARCHAR(128) NOT NULL,
    detail           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_reward_ledger_idempotency UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_play_reward_ledger_user_id ON play_reward_ledger(user_id);
CREATE INDEX IF NOT EXISTS idx_play_reward_ledger_source ON play_reward_ledger(source);

COMMENT ON TABLE play_reward_ledger IS 'Audit trail for play-feature balance grants';

CREATE TABLE IF NOT EXISTS play_arena_periods (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    start_at   TIMESTAMPTZ NOT NULL,
    end_at     TIMESTAMPTZ NOT NULL,
    status     VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_play_arena_periods_status CHECK (status IN ('draft', 'active', 'settled'))
);

CREATE INDEX IF NOT EXISTS idx_play_arena_periods_active ON play_arena_periods(status, start_at, end_at);

COMMENT ON TABLE play_arena_periods IS 'Token farm ranking periods';

-- Default settings (opt-in features, disabled until admin enables).
INSERT INTO settings (key, value)
VALUES
    ('play_checkin_enabled', 'false'),
    ('play_checkin_daily_reward', '0.5'),
    ('play_arena_enabled', 'false'),
    ('play_blindbox_enabled', 'false'),
    ('play_quiz_enabled', 'false'),
    ('play_agent_team_enabled', 'false'),
    ('public_models_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
