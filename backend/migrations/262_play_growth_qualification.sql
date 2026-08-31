-- 262_play_growth_qualification.sql
-- Immutable qualification evidence for non-cash growth participation.

CREATE TABLE IF NOT EXISTS play_growth_eligibility_snapshots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source VARCHAR(32) NOT NULL CHECK (source IN ('checkin', 'quiz', 'blindbox')),
    action_id VARCHAR(128) NOT NULL CHECK (BTRIM(action_id) <> ''),
    activity_date DATE NOT NULL,
    tier VARCHAR(16) NOT NULL CHECK (tier IN ('explorer', 'active')),
    reward_mode VARCHAR(16) NOT NULL CHECK (reward_mode IN ('energy', 'redeemable')),
    primary_reason VARCHAR(64) NOT NULL,
    email_verified BOOLEAN NOT NULL,
    account_age_days INT NOT NULL CHECK (account_age_days >= 0),
    has_recent_usage BOOLEAN NOT NULL,
    net_balance_recharge_30d DECIMAL(20,2) NOT NULL DEFAULT 0 CHECK (net_balance_recharge_30d >= 0),
    has_active_subscription BOOLEAN NOT NULL,
    rule_version VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_growth_eligibility_snapshot_action UNIQUE (user_id, source, action_id)
);

-- Check-in and quiz are once-per-day activities. Blindbox has multiple paid
-- opens per day and is instead unique by its action id above.
CREATE UNIQUE INDEX IF NOT EXISTS uq_play_growth_eligibility_snapshot_daily_activity
    ON play_growth_eligibility_snapshots (user_id, source, activity_date)
    WHERE source IN ('checkin', 'quiz');

CREATE INDEX IF NOT EXISTS idx_play_growth_eligibility_snapshots_user_created
    ON play_growth_eligibility_snapshots (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS play_growth_energy_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source VARCHAR(32) NOT NULL CHECK (source IN ('checkin', 'quiz')),
    action_id VARCHAR(128) NOT NULL CHECK (BTRIM(action_id) <> ''),
    amount BIGINT NOT NULL CHECK (amount > 0),
    eligibility_snapshot_id BIGINT NOT NULL REFERENCES play_growth_eligibility_snapshots(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_growth_energy_action UNIQUE (action_id)
);

CREATE INDEX IF NOT EXISTS idx_play_growth_energy_ledger_user_created
    ON play_growth_energy_ledger (user_id, created_at DESC);

-- Qualification decisions and non-cash energy grants are audit evidence. Keep
-- both tables append-only at the database boundary; correction requires a new
-- action/snapshot rather than rewriting an existing decision.
CREATE OR REPLACE FUNCTION reject_play_growth_qualification_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% rows are immutable after insertion', TG_TABLE_NAME;
END;
$$;

DROP TRIGGER IF EXISTS play_growth_eligibility_snapshots_immutable ON play_growth_eligibility_snapshots;
CREATE TRIGGER play_growth_eligibility_snapshots_immutable
    BEFORE UPDATE OR DELETE ON play_growth_eligibility_snapshots
    FOR EACH ROW
    EXECUTE FUNCTION reject_play_growth_qualification_mutation();

DROP TRIGGER IF EXISTS play_growth_energy_ledger_immutable ON play_growth_energy_ledger;
CREATE TRIGGER play_growth_energy_ledger_immutable
    BEFORE UPDATE OR DELETE ON play_growth_energy_ledger
    FOR EACH ROW
    EXECUTE FUNCTION reject_play_growth_qualification_mutation();

ALTER TABLE play_checkins
    ADD COLUMN IF NOT EXISTS growth_eligibility_snapshot_id BIGINT NULL
    REFERENCES play_growth_eligibility_snapshots(id) ON DELETE RESTRICT;

ALTER TABLE play_quiz_attempts
    ADD COLUMN IF NOT EXISTS growth_eligibility_snapshot_id BIGINT NULL
    REFERENCES play_growth_eligibility_snapshots(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_play_checkins_growth_snapshot
    ON play_checkins (growth_eligibility_snapshot_id)
    WHERE growth_eligibility_snapshot_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_play_quiz_attempts_growth_snapshot
    ON play_quiz_attempts (growth_eligibility_snapshot_id)
    WHERE growth_eligibility_snapshot_id IS NOT NULL;
