-- 266_play_growth_reward_snapshot_links.sql
-- Strengthen the audit relationship for new redeemable Play rewards. Existing
-- history stays untouched: these nullable columns only describe actions issued
-- after the v0.1.182 qualification rule is active.

ALTER TABLE play_blindbox_opens
    ADD COLUMN IF NOT EXISTS growth_eligibility_snapshot_id BIGINT NULL
    REFERENCES play_growth_eligibility_snapshots(id) ON DELETE RESTRICT;

ALTER TABLE play_reward_ledger
    ADD COLUMN IF NOT EXISTS growth_eligibility_snapshot_id BIGINT NULL
    REFERENCES play_growth_eligibility_snapshots(id) ON DELETE RESTRICT;

ALTER TABLE play_reward_ledger
    ADD COLUMN IF NOT EXISTS growth_rule_version VARCHAR(32) NULL;

CREATE INDEX IF NOT EXISTS idx_play_blindbox_opens_growth_snapshot
    ON play_blindbox_opens (growth_eligibility_snapshot_id)
    WHERE growth_eligibility_snapshot_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_play_reward_ledger_growth_snapshot
    ON play_reward_ledger (growth_eligibility_snapshot_id)
    WHERE growth_eligibility_snapshot_id IS NOT NULL;

-- A qualifying new reward must retain both the foreign-key evidence and the
-- exact rule version. Null remains valid for historical balances and unrelated
-- Play ledger events, which this policy must never reinterpret or claw back.
ALTER TABLE play_reward_ledger
    DROP CONSTRAINT IF EXISTS chk_play_reward_ledger_growth_snapshot_rule_version;

ALTER TABLE play_reward_ledger
    ADD CONSTRAINT chk_play_reward_ledger_growth_snapshot_rule_version
    CHECK (
        (growth_eligibility_snapshot_id IS NULL AND growth_rule_version IS NULL)
        OR
        (growth_eligibility_snapshot_id IS NOT NULL AND growth_rule_version IS NOT NULL AND BTRIM(growth_rule_version) <> '')
    );
