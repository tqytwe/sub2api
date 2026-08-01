-- Immutable arena season evidence for the growth competition surface.
-- Existing settlement and reward-ledger rows remain authoritative; this table
-- only records a public-safe ranking snapshot for periods settled after rollout.

ALTER TABLE play_arena_periods
    ADD COLUMN IF NOT EXISTS reward_rules_json JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE play_arena_periods
    ADD COLUMN IF NOT EXISTS reward_rules_frozen_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS play_arena_season_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    period_id     BIGINT NOT NULL REFERENCES play_arena_periods(id) ON DELETE RESTRICT,
    rank          INT NOT NULL,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_sum     BIGINT NOT NULL,
    reward_amount DECIMAL(20, 8) NOT NULL,
    payout_status VARCHAR(16) NOT NULL DEFAULT 'paid',
    paid_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_arena_season_snapshots_period_rank UNIQUE (period_id, rank),
    CONSTRAINT uq_play_arena_season_snapshots_period_user UNIQUE (period_id, user_id),
    CONSTRAINT chk_play_arena_season_snapshots_rank CHECK (rank > 0),
    CONSTRAINT chk_play_arena_season_snapshots_tokens CHECK (token_sum >= 0),
    CONSTRAINT chk_play_arena_season_snapshots_reward CHECK (reward_amount >= 0),
    CONSTRAINT chk_play_arena_season_snapshots_status CHECK (payout_status IN ('paid'))
);

CREATE INDEX IF NOT EXISTS idx_play_arena_season_snapshots_period_rank
    ON play_arena_season_snapshots(period_id, rank);

CREATE INDEX IF NOT EXISTS idx_play_arena_periods_monthly_due
    ON play_arena_periods(end_at ASC, id ASC)
    WHERE period_type = 'monthly' AND status = 'active';

COMMENT ON COLUMN play_arena_periods.reward_rules_json IS
    'Monthly reward tiers frozen when the season first becomes active; operators can only affect later seasons.';
COMMENT ON TABLE play_arena_season_snapshots IS
    'Immutable monthly token-farm ranking and paid reward proof, evaluated in Asia/Shanghai season windows.';
