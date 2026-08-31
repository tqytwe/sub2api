-- 268_play_growth_governance.sql
-- Operational approval and budget gate for daily participation rewards:
-- check-in (including makeup), quiz, and blindbox. Arena, team, and referral
-- rewards are intentionally outside this migration because they have separate
-- settlement, budget, and audit owners.
--
-- This migration is intentionally append-only.  An approval is never edited
-- or deleted; a later `revoked` row supersedes it.  Historical rewards remain
-- valid and are not re-evaluated when governance rules change.

CREATE TABLE IF NOT EXISTS play_growth_governance_approvals (
    id              BIGSERIAL PRIMARY KEY,
    decision        VARCHAR(16) NOT NULL,
    budget_amount   DECIMAL(20, 8) NOT NULL DEFAULT 0,
    rollout_percent SMALLINT NOT NULL DEFAULT 0,
    cohort_start    TIMESTAMPTZ,
    cohort_end      TIMESTAMPTZ,
    cohort_metrics  JSONB NOT NULL DEFAULT '{}'::jsonb,
    rule_version    VARCHAR(32) NOT NULL,
    reason          TEXT NOT NULL,
    actor_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT play_growth_governance_decision_check
        CHECK (decision IN ('approved', 'revoked')),
    CONSTRAINT play_growth_governance_budget_check
        CHECK (decision = 'revoked' OR budget_amount > 0),
    CONSTRAINT play_growth_governance_rollout_check
        CHECK (decision = 'revoked' OR rollout_percent BETWEEN 10 AND 20),
    CONSTRAINT play_growth_governance_cohort_check
        CHECK (
            decision = 'revoked'
            OR (
                cohort_start IS NOT NULL
                AND cohort_end IS NOT NULL
                AND cohort_end >= cohort_start + INTERVAL '14 days'
            )
        ),
    CONSTRAINT play_growth_governance_metrics_check
        CHECK (jsonb_typeof(cohort_metrics) = 'object'),
    CONSTRAINT play_growth_governance_reason_check
        CHECK (char_length(btrim(reason)) BETWEEN 10 AND 500)
);

CREATE INDEX IF NOT EXISTS idx_play_growth_governance_latest
    ON play_growth_governance_approvals (id DESC);

-- A separate immutable ledger records the budget unit reserved by each new
-- reward.  It makes the spending decision auditable and gives the service a
-- transaction-safe place to enforce the approval's upper bound.
CREATE TABLE IF NOT EXISTS play_growth_reward_budget_ledger (
    id            BIGSERIAL PRIMARY KEY,
    approval_id   BIGINT NOT NULL REFERENCES play_growth_governance_approvals(id) ON DELETE RESTRICT,
    -- The immutable audit ledger must survive a user-domain hard-delete
    -- attempt. User deletion is therefore rejected until a separately audited
    -- retention process is designed; CASCADE would conflict with the immutable
    -- DELETE trigger and could otherwise erase historical budget spend.
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source        VARCHAR(32) NOT NULL CHECK (source IN ('checkin', 'quiz', 'blindbox')),
    action_id     VARCHAR(128) NOT NULL CHECK (btrim(action_id) <> ''),
    amount        DECIMAL(20, 8) NOT NULL CHECK (amount > 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- An action must not reserve again merely because an approval was later
    -- superseded. The source/action pair is the durable idempotency boundary.
    CONSTRAINT uq_play_growth_reward_budget_action UNIQUE (source, action_id)
);

CREATE INDEX IF NOT EXISTS idx_play_growth_reward_budget_approval_created
    ON play_growth_reward_budget_ledger (approval_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_play_growth_reward_budget_user_created
    ON play_growth_reward_budget_ledger (user_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_play_growth_governance_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% rows are immutable after insertion', TG_TABLE_NAME;
END;
$$;

DROP TRIGGER IF EXISTS play_growth_governance_approvals_immutable
    ON play_growth_governance_approvals;
CREATE TRIGGER play_growth_governance_approvals_immutable
    BEFORE UPDATE OR DELETE ON play_growth_governance_approvals
    FOR EACH ROW
    EXECUTE FUNCTION reject_play_growth_governance_mutation();

DROP TRIGGER IF EXISTS play_growth_reward_budget_ledger_immutable
    ON play_growth_reward_budget_ledger;
CREATE TRIGGER play_growth_reward_budget_ledger_immutable
    BEFORE UPDATE OR DELETE ON play_growth_reward_budget_ledger
    FOR EACH ROW
    EXECUTE FUNCTION reject_play_growth_governance_mutation();

COMMENT ON TABLE play_growth_governance_approvals IS
    'Append-only operations approvals for check-in, makeup, quiz, and blindbox rewards';
COMMENT ON TABLE play_growth_reward_budget_ledger IS
    'Immutable budget reservations for check-in, makeup, quiz, and blindbox rewards';
