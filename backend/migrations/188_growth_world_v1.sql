-- 188_growth_world_v1.sql
-- Truthful public metrics, consistent arena aggregates, team workflows, and reward audit.

CREATE TABLE IF NOT EXISTS public_metric_snapshots (
    snapshot_id          VARCHAR(64) PRIMARY KEY,
    bucket_at            TIMESTAMPTZ NOT NULL UNIQUE,
    source               VARCHAR(16) NOT NULL DEFAULT 'live',
    methodology_version  VARCHAR(32) NOT NULL DEFAULT 'growth-world-v1',
    requests_24h         BIGINT NOT NULL DEFAULT 0,
    requests_total       BIGINT NOT NULL DEFAULT 0,
    active_users_7d      BIGINT NOT NULL DEFAULT 0,
    tokens_total         BIGINT NOT NULL DEFAULT 0,
    success_rate_30d     DECIMAL(7, 4),
    p50_ttft_ms          BIGINT,
    p95_ttft_ms          BIGINT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_public_metric_snapshot_source CHECK (source IN ('live', 'estimated', 'demo'))
);

CREATE INDEX IF NOT EXISTS idx_public_metric_snapshots_bucket
    ON public_metric_snapshots(bucket_at DESC);

CREATE TABLE IF NOT EXISTS play_usage_aggregates (
    aggregate_type VARCHAR(16) NOT NULL,
    subject_id     BIGINT NOT NULL,
    period_type    VARCHAR(16) NOT NULL,
    period_start   DATE NOT NULL,
    request_count  BIGINT NOT NULL DEFAULT 0,
    token_sum      BIGINT NOT NULL DEFAULT 0,
    active_days    INT NOT NULL DEFAULT 0,
    score          BIGINT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (aggregate_type, subject_id, period_type, period_start),
    CONSTRAINT chk_play_usage_aggregate_type CHECK (aggregate_type IN ('user', 'team')),
    CONSTRAINT chk_play_usage_period_type CHECK (period_type IN ('daily', 'monthly', 'weekly'))
);

CREATE INDEX IF NOT EXISTS idx_play_usage_aggregates_rank
    ON play_usage_aggregates(period_type, period_start, aggregate_type, score DESC);

CREATE TABLE IF NOT EXISTS play_activity_events (
    id           BIGSERIAL PRIMARY KEY,
    event_key    VARCHAR(160) NOT NULL UNIQUE,
    event_type   VARCHAR(48) NOT NULL,
    actor_hash   VARCHAR(16) NOT NULL,
    subject_type VARCHAR(16),
    subject_id   BIGINT,
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_public    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_play_activity_events_public_time
    ON play_activity_events(is_public, created_at DESC);

CREATE TABLE IF NOT EXISTS play_reward_audit (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source            VARCHAR(48) NOT NULL,
    pool_version      VARCHAR(64),
    open_source       VARCHAR(24),
    idempotency_key   VARCHAR(128) NOT NULL UNIQUE,
    cost_amount       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    reward_amount     DECIMAL(20, 8) NOT NULL DEFAULT 0,
    detail            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS play_blindbox_ticket_ledger (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source           VARCHAR(48) NOT NULL,
    quantity         INT NOT NULL,
    idempotency_key  VARCHAR(128) NOT NULL UNIQUE,
    detail           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_play_blindbox_ticket_user
    ON play_blindbox_ticket_ledger(user_id, created_at DESC);

ALTER TABLE play_blindbox_opens
    ADD COLUMN IF NOT EXISTS pool_version VARCHAR(64) NOT NULL DEFAULT 'legacy-v1',
    ADD COLUMN IF NOT EXISTS open_source VARCHAR(24) NOT NULL DEFAULT 'paid';

ALTER TABLE play_teams
    ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS max_members INT NOT NULL DEFAULT 8,
    ADD COLUMN IF NOT EXISTS level INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS invite_version INT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS play_team_join_requests (
    id           BIGSERIAL PRIMARY KEY,
    team_id      BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       VARCHAR(16) NOT NULL DEFAULT 'pending',
    reviewed_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_team_join_request UNIQUE (team_id, user_id),
    CONSTRAINT chk_play_team_join_request_status CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_play_team_join_requests_team_status
    ON play_team_join_requests(team_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS play_team_weekly_progress (
    team_id         BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE CASCADE,
    week_start      DATE NOT NULL,
    token_target    BIGINT NOT NULL DEFAULT 100000,
    request_target  BIGINT NOT NULL DEFAULT 20,
    token_sum       BIGINT NOT NULL DEFAULT 0,
    request_count   BIGINT NOT NULL DEFAULT 0,
    active_days     INT NOT NULL DEFAULT 0,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, week_start)
);

INSERT INTO settings (key, value)
VALUES
    ('play_blindbox_pool_json', '{"version":"season-1-v1","cost":0.5,"rtp_cap":0.9,"tiers":[{"amount":0.05,"weight":4000},{"amount":0.2,"weight":3000},{"amount":0.5,"weight":1800},{"amount":1,"weight":800},{"amount":3,"weight":300},{"amount":10,"weight":90},{"amount":20,"weight":10}]}'),
    ('play_blindbox_paid_enabled', 'false'),
    ('play_blindbox_region_enabled', 'false'),
	('play_blindbox_first_request_tickets', '1'),
	('play_blindbox_team_weekly_tickets', '1'),
    ('play_team_max_members', '8'),
    ('play_team_weekly_token_target', '100000'),
    ('play_team_weekly_request_target', '20'),
    ('play_public_activity_min_count', '1'),
    ('play_founder_season_json', '{"name":"Founding Season","duration_weeks":6,"enabled":true}'),
    ('play_growth_experiment_json', '{"holdout_pct":5,"enabled":false}')
ON CONFLICT (key) DO NOTHING;

-- The blind-box pool upgrade is intentionally disabled until a separate VIP pool
-- is implemented. Remove the legacy perk from persisted tier configuration while
-- preserving every other configured perk and tier field.
UPDATE settings
SET value = (
    SELECT jsonb_agg(
        CASE
            WHEN jsonb_typeof(item->'perks') = 'array' THEN jsonb_set(
                item,
                '{perks}',
                COALESCE(
                    (
                        SELECT jsonb_agg(perk ORDER BY perk_order)
                        FROM jsonb_array_elements(item->'perks') WITH ORDINALITY AS perks(perk, perk_order)
                        WHERE perk <> to_jsonb('blindbox_pool_upgrade'::text)
                    ),
                    '[]'::jsonb
                )
            )
            ELSE item
        END
        ORDER BY item_order
    )::text
    FROM jsonb_array_elements(settings.value::jsonb) WITH ORDINALITY AS tiers(item, item_order)
)
WHERE key = 'play_vip_tiers'
  AND value LIKE '%blindbox_pool_upgrade%';
