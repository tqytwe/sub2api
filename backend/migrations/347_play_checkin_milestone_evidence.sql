-- New check-ins freeze the milestone rules alongside their original eligibility.
-- Historical rows are deliberately not backfilled from today's configuration.
CREATE TABLE IF NOT EXISTS play_checkin_rule_snapshots (
    checkin_id BIGINT PRIMARY KEY REFERENCES play_checkins(id) ON DELETE RESTRICT,
    milestones JSONB NOT NULL CHECK (jsonb_typeof(milestones) = 'array'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS play_checkin_reconciliations (
    id BIGSERIAL PRIMARY KEY,
    checkin_id BIGINT NOT NULL REFERENCES play_checkins(id) ON DELETE RESTRICT,
    action_id TEXT NOT NULL UNIQUE,
    previous_streak INT NOT NULL CHECK (previous_streak >= 0),
    observed_streak INT NOT NULL CHECK (observed_streak > 0),
    milestone_added NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (milestone_added >= 0),
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_play_checkin_reconciliations_checkin
    ON play_checkin_reconciliations(checkin_id, id DESC);

CREATE TRIGGER play_checkin_rule_snapshots_immutable
    BEFORE UPDATE OR DELETE ON play_checkin_rule_snapshots
    FOR EACH ROW EXECUTE FUNCTION reject_play_growth_qualification_mutation();
CREATE TRIGGER play_checkin_reconciliations_immutable
    BEFORE UPDATE OR DELETE ON play_checkin_reconciliations
    FOR EACH ROW EXECUTE FUNCTION reject_play_growth_qualification_mutation();
