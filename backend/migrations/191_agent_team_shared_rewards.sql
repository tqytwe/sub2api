-- Preserve team and membership history while allowing one active membership per user.
ALTER TABLE play_teams
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

ALTER TABLE play_team_members
    ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ;

ALTER TABLE play_team_members
    DROP CONSTRAINT IF EXISTS uq_play_team_members_user;

ALTER TABLE play_team_members
    DROP CONSTRAINT IF EXISTS uq_play_team_members_team_user;

CREATE UNIQUE INDEX IF NOT EXISTS uq_play_team_members_active_user
    ON play_team_members(user_id)
    WHERE left_at IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_play_team_members_active_interval'
          AND conrelid = 'play_team_members'::regclass
    ) THEN
        ALTER TABLE play_team_members
            ADD CONSTRAINT chk_play_team_members_active_interval
            CHECK (left_at IS NULL OR left_at >= joined_at);
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS play_team_events (
    id              BIGSERIAL PRIMARY KEY,
    team_id         BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT,
    actor_user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    subject_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    event_type      VARCHAR(32) NOT NULL,
    detail          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_play_team_events_team_created_at
    ON play_team_events(team_id, created_at DESC);

CREATE TABLE IF NOT EXISTS play_team_settlements (
    id                    BIGSERIAL PRIMARY KEY,
    team_id               BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT,
    period_start          DATE NOT NULL,
    window_start          TIMESTAMPTZ NOT NULL,
    window_end            TIMESTAMPTZ NOT NULL,
    team_spend            DECIMAL(20, 8) NOT NULL,
    reached_threshold     DECIMAL(20, 8) NOT NULL,
    reward_rate           DECIMAL(20, 8) NOT NULL,
    pool_amount           DECIMAL(20, 8) NOT NULL,
    cap_amount            DECIMAL(20, 8) NOT NULL,
    status                VARCHAR(16) NOT NULL DEFAULT 'pending',
    last_error            TEXT,
    processing_started_at TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_team_settlements_team_period
        UNIQUE (team_id, period_start),
    CONSTRAINT chk_play_team_settlements_window
        CHECK (window_start < window_end),
    CONSTRAINT chk_play_team_settlements_amounts
        CHECK (
            team_spend >= 0
            AND reached_threshold > 0
            AND reward_rate > 0
            AND reward_rate <= 1
            AND pool_amount >= 0
            AND cap_amount > 0
            AND pool_amount <= cap_amount
        ),
    CONSTRAINT chk_play_team_settlements_status
        CHECK (status IN ('pending', 'processing', 'completed', 'partial', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_play_team_settlements_status_period
    ON play_team_settlements(status, period_start);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_play_team_settlements_period_window'
          AND conrelid = 'play_team_settlements'::regclass
    ) THEN
        ALTER TABLE play_team_settlements
            ADD CONSTRAINT chk_play_team_settlements_period_window
            CHECK (
                period_start = DATE_TRUNC('month', period_start)::date
                AND window_start = (
                    period_start::timestamp AT TIME ZONE 'Asia/Shanghai'
                )
                AND window_end = (
                    (period_start + INTERVAL '1 month')::timestamp
                    AT TIME ZONE 'Asia/Shanghai'
                )
            );
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_play_team_settlements_status_timestamps'
          AND conrelid = 'play_team_settlements'::regclass
    ) THEN
        ALTER TABLE play_team_settlements
            ADD CONSTRAINT chk_play_team_settlements_status_timestamps
            CHECK (
                (status = 'completed') = (completed_at IS NOT NULL)
                AND (
                    status = 'pending'
                    OR processing_started_at IS NOT NULL
                )
            );
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS play_team_reward_allocations (
    id              BIGSERIAL PRIMARY KEY,
    settlement_id   BIGINT NOT NULL REFERENCES play_team_settlements(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    contribution    DECIMAL(20, 8) NOT NULL,
    ratio           DECIMAL(20, 8) NOT NULL,
    reward_amount   DECIMAL(20, 8) NOT NULL,
    payout_status   VARCHAR(16) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(128) NOT NULL,
    paid_at         TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_team_reward_allocations_settlement_user
        UNIQUE (settlement_id, user_id),
    CONSTRAINT uq_play_team_reward_allocations_idempotency
        UNIQUE (idempotency_key),
    CONSTRAINT chk_play_team_reward_allocations_amounts
        CHECK (
            contribution >= 0
            AND ratio >= 0
            AND ratio <= 1
            AND reward_amount >= 0
        ),
    CONSTRAINT chk_play_team_reward_allocations_payout_status
        CHECK (payout_status IN ('pending', 'processing', 'paid', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_play_team_reward_allocations_settlement_status
    ON play_team_reward_allocations(settlement_id, payout_status);

CREATE INDEX IF NOT EXISTS idx_play_team_reward_allocations_user_settlement
    ON play_team_reward_allocations(user_id, settlement_id DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_play_team_reward_allocations_paid_timestamp'
          AND conrelid = 'play_team_reward_allocations'::regclass
    ) THEN
        ALTER TABLE play_team_reward_allocations
            ADD CONSTRAINT chk_play_team_reward_allocations_paid_timestamp
            CHECK (
                (payout_status = 'paid') = (paid_at IS NOT NULL)
            );
    END IF;
END
$$;

-- Seed the approved shared-reward policy without replacing operator-managed values.
INSERT INTO settings (key, value)
VALUES
    ('play_team_shared_reward_enabled', 'true'),
    ('play_team_shared_reward_tiers', '[{"threshold":"20","rate":"0.02"},{"threshold":"100","rate":"0.03"},{"threshold":"500","rate":"0.04"},{"threshold":"2000","rate":"0.05"}]'),
    ('play_team_shared_reward_cap', '250'),
    (
        'play_team_shared_reward_start_month',
        TO_CHAR(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM')
    )
ON CONFLICT (key) DO NOTHING;
