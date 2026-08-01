-- Team competition discovery, admission audit, and immutable public season history.
-- Existing settlements remain authoritative; this migration only records additive snapshots.

ALTER TABLE play_team_members
    ADD COLUMN IF NOT EXISTS reward_eligible_at TIMESTAMPTZ;

UPDATE play_team_members
SET reward_eligible_at = CASE
    WHEN EXTRACT(DAY FROM joined_at AT TIME ZONE 'Asia/Shanghai') > 25
        THEN DATE_TRUNC('month', (joined_at AT TIME ZONE 'Asia/Shanghai') + INTERVAL '1 month')
            AT TIME ZONE 'Asia/Shanghai'
    ELSE joined_at
END
WHERE reward_eligible_at IS NULL;

ALTER TABLE play_team_members
    ALTER COLUMN reward_eligible_at SET DEFAULT NOW();

ALTER TABLE play_team_members
    ALTER COLUMN reward_eligible_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_play_team_members_competition_window
    ON play_team_members (team_id, user_id, joined_at, reward_eligible_at, left_at);

CREATE INDEX IF NOT EXISTS idx_usage_logs_team_competition_positive_cost
    ON usage_logs (user_id, created_at)
    WHERE actual_cost > 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_play_team_members_reward_eligible_at'
          AND conrelid = 'play_team_members'::regclass
    ) THEN
        ALTER TABLE play_team_members
            ADD CONSTRAINT chk_play_team_members_reward_eligible_at
            CHECK (reward_eligible_at >= joined_at);
    END IF;
END
$$;

ALTER TABLE play_teams
    ALTER COLUMN invite_code TYPE VARCHAR(64);

ALTER TABLE play_teams
    ADD COLUMN IF NOT EXISTS invite_code_expires_at TIMESTAMPTZ;

ALTER TABLE play_teams
    ADD COLUMN IF NOT EXISTS invite_code_rotated_at TIMESTAMPTZ;

ALTER TABLE play_teams
    ADD COLUMN IF NOT EXISTS is_recruiting BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE play_teams
SET invite_code_expires_at = COALESCE(invite_code_expires_at, NOW() + INTERVAL '30 days'),
    invite_code_rotated_at = COALESCE(invite_code_rotated_at, NOW());

ALTER TABLE play_teams
    ALTER COLUMN invite_code_expires_at SET NOT NULL;

ALTER TABLE play_teams
    ALTER COLUMN invite_code_rotated_at SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_play_teams_invite_window'
          AND conrelid = 'play_teams'::regclass
    ) THEN
        ALTER TABLE play_teams
            ADD CONSTRAINT chk_play_teams_invite_window
            CHECK (invite_code_expires_at >= invite_code_rotated_at);
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS play_team_join_applications (
    id                BIGSERIAL PRIMARY KEY,
    team_id           BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT,
    applicant_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status            VARCHAR(16) NOT NULL DEFAULT 'pending',
    message           VARCHAR(300) NOT NULL DEFAULT '',
    requested_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sla_due_at        TIMESTAMPTZ NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    handled_at        TIMESTAMPTZ,
    handled_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    decision_note     VARCHAR(300) NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_play_team_join_applications_status
        CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn', 'expired')),
    CONSTRAINT chk_play_team_join_applications_window
        CHECK (requested_at <= sla_due_at AND sla_due_at <= expires_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_play_team_join_applications_pending
    ON play_team_join_applications(team_id, applicant_user_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_play_team_join_applications_team_status
    ON play_team_join_applications(team_id, status, requested_at DESC);

CREATE INDEX IF NOT EXISTS idx_play_team_join_applications_user_status
    ON play_team_join_applications(applicant_user_id, status, requested_at DESC);

CREATE INDEX IF NOT EXISTS idx_play_team_join_applications_expiry
    ON play_team_join_applications(status, expires_at)
    WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS play_team_join_application_events (
    id             BIGSERIAL PRIMARY KEY,
    application_id BIGINT NOT NULL REFERENCES play_team_join_applications(id) ON DELETE RESTRICT,
    team_id        BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT,
    actor_user_id  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    event_type     VARCHAR(32) NOT NULL,
    from_status    VARCHAR(16) NOT NULL DEFAULT '',
    to_status      VARCHAR(16) NOT NULL DEFAULT '',
    detail         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_play_team_join_application_events_application
    ON play_team_join_application_events(application_id, created_at ASC);

CREATE TABLE IF NOT EXISTS play_team_seasons (
    id           BIGSERIAL PRIMARY KEY,
    period_start DATE NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end   TIMESTAMPTZ NOT NULL,
    rules_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    status       VARCHAR(16) NOT NULL DEFAULT 'active',
    frozen_at    TIMESTAMPTZ,
    settled_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_team_seasons_period UNIQUE (period_start),
    CONSTRAINT chk_play_team_seasons_period
        CHECK (
            period_start = DATE_TRUNC('month', period_start)::date
            AND window_start = period_start::timestamp AT TIME ZONE 'Asia/Shanghai'
            AND window_end = (period_start + INTERVAL '1 month')::timestamp AT TIME ZONE 'Asia/Shanghai'
        ),
    CONSTRAINT chk_play_team_seasons_status
        CHECK (status IN ('active', 'settling', 'settled', 'failed', 'legacy'))
);

CREATE TABLE IF NOT EXISTS play_team_season_rankings (
    id                BIGSERIAL PRIMARY KEY,
    season_id         BIGINT NOT NULL REFERENCES play_team_seasons(id) ON DELETE RESTRICT,
    team_id           BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT,
    team_name         VARCHAR(64) NOT NULL,
    rank              INT NOT NULL,
    member_count      INT NOT NULL DEFAULT 0,
    team_spend        DECIMAL(20, 8) NOT NULL DEFAULT 0,
    reached_threshold DECIMAL(20, 8) NOT NULL DEFAULT 0,
    reward_rate       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    pool_amount       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    paid_amount       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    settlement_id     BIGINT REFERENCES play_team_settlements(id) ON DELETE SET NULL,
    settlement_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_team_season_rankings_team UNIQUE (season_id, team_id),
    CONSTRAINT uq_play_team_season_rankings_rank UNIQUE (season_id, rank),
    CONSTRAINT chk_play_team_season_rankings_values
        CHECK (rank > 0 AND member_count >= 0 AND team_spend >= 0 AND pool_amount >= 0 AND paid_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_play_team_season_rankings_season_rank
    ON play_team_season_rankings(season_id, rank ASC);

ALTER TABLE play_team_reward_allocations
    ADD COLUMN IF NOT EXISTS payout_lease_expires_at TIMESTAMPTZ;

ALTER TABLE play_team_reward_allocations
    ADD COLUMN IF NOT EXISTS payout_attempts INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_play_team_reward_allocations_lease
    ON play_team_reward_allocations(settlement_id, payout_status, payout_lease_expires_at)
    WHERE reward_amount > 0;

-- Backfill existing paid reward months as immutable legacy snapshots. The old
-- settlement amount is retained rather than recalculated under a new rule.
INSERT INTO play_team_seasons (
    period_start, window_start, window_end, rules_json, status, frozen_at, settled_at
)
SELECT
    s.period_start,
    MIN(s.window_start),
    MAX(s.window_end),
    jsonb_build_object(
        'source', 'legacy_settlement_snapshot',
        'scoring_version', 'legacy-v1',
        'timezone', 'Asia/Shanghai'
    ),
    'legacy',
    MIN(s.created_at),
    MAX(s.completed_at)
FROM play_team_settlements s
GROUP BY s.period_start
ON CONFLICT (period_start) DO NOTHING;

WITH legacy_settlements AS (
    SELECT
        season.id AS season_id,
        s.id AS settlement_id,
        s.team_id,
        t.name AS team_name,
        s.period_start,
        s.window_start,
        s.window_end,
        s.team_spend,
        s.reached_threshold,
        s.reward_rate,
        s.pool_amount,
        s.status AS settlement_status,
        COALESCE(SUM(a.reward_amount) FILTER (WHERE a.payout_status = 'paid'), 0) AS paid_amount
    FROM play_team_settlements s
    JOIN play_team_seasons season ON season.period_start = s.period_start
    JOIN play_teams t ON t.id = s.team_id
    LEFT JOIN play_team_reward_allocations a ON a.settlement_id = s.id
    GROUP BY
        season.id, s.id, s.team_id, t.name, s.period_start, s.window_start, s.window_end,
        s.team_spend, s.reached_threshold, s.reward_rate, s.pool_amount, s.status
), ranked AS (
    SELECT
        ls.*,
        ROW_NUMBER() OVER (
            PARTITION BY ls.period_start
            ORDER BY ls.team_spend DESC, ls.team_id ASC
        )::int AS rank,
        (
            SELECT COUNT(DISTINCT m.user_id)::int
            FROM play_team_members m
            WHERE m.team_id = ls.team_id
              AND m.joined_at < ls.window_end
              AND (m.left_at IS NULL OR m.left_at > ls.window_start)
        ) AS member_count
    FROM legacy_settlements ls
)
INSERT INTO play_team_season_rankings (
    season_id, team_id, team_name, rank, member_count, team_spend,
    reached_threshold, reward_rate, pool_amount, paid_amount,
    settlement_id, settlement_status
)
SELECT
    season_id, team_id, team_name, rank, member_count, team_spend,
    reached_threshold, reward_rate, pool_amount, paid_amount,
    settlement_id, settlement_status
FROM ranked
WHERE rank <= 10
ON CONFLICT (season_id, team_id) DO NOTHING;
