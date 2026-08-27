-- Immutable execution and funding state for durable mobile video jobs.
-- Do not rewrite 259: that migration may already have been reviewed/applied
-- independently. Existing rows without a snapshot are deliberately left
-- visible for the worker to fail closed rather than being submitted blindly.

SET LOCAL lock_timeout = '5s';

ALTER TABLE mobile_video_jobs
    ADD COLUMN IF NOT EXISTS execution_snapshot JSONB,
    ADD COLUMN IF NOT EXISTS unit_price_usd NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS rate_multiplier NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS hold_amount NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS billing_state VARCHAR(24) NOT NULL DEFAULT 'funding',
    ADD COLUMN IF NOT EXISTS client_cancelled_at TIMESTAMPTZ;

ALTER TABLE mobile_video_jobs
    DROP CONSTRAINT IF EXISTS mobile_video_jobs_state_check;

ALTER TABLE mobile_video_jobs
    DROP CONSTRAINT IF EXISTS mobile_video_jobs_billing_state_check;

ALTER TABLE mobile_video_jobs
    DROP CONSTRAINT IF EXISTS mobile_video_jobs_funding_values_check;

ALTER TABLE mobile_video_jobs
    ADD CONSTRAINT mobile_video_jobs_state_check
        CHECK (state IN ('funding', 'queued', 'submitting', 'submission_unknown', 'polling', 'completed', 'failed', 'cancelled'));

ALTER TABLE mobile_video_jobs
    ADD CONSTRAINT mobile_video_jobs_billing_state_check
        CHECK (billing_state IN ('funding', 'reserved', 'releasing', 'captured', 'released', 'not_required'));

ALTER TABLE mobile_video_jobs
    ADD CONSTRAINT mobile_video_jobs_funding_values_check
        CHECK (
            -- Rows created before this migration have no immutable execution
            -- snapshot or quoted balance values. Keep that exact all-NULL
            -- legacy shape valid so this additive migration can be applied;
            -- the worker treats it as fail-closed and never dispatches it.
            (
                execution_snapshot IS NULL
                AND unit_price_usd IS NULL
                AND rate_multiplier IS NULL
                AND hold_amount IS NULL
            )
            OR (
                execution_snapshot IS NOT NULL
                AND unit_price_usd IS NOT NULL
                AND rate_multiplier IS NOT NULL
                AND hold_amount IS NOT NULL
                AND unit_price_usd >= 0
                AND rate_multiplier >= 0
                AND hold_amount >= 0
            )
        );

CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_funding_recovery
    ON mobile_video_jobs (state, updated_at, created_at)
    WHERE state = 'funding';

CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_releasing_recovery
    ON mobile_video_jobs (updated_at, created_at)
    WHERE state = 'funding' AND billing_state = 'releasing';

CREATE INDEX IF NOT EXISTS idx_mobile_video_jobs_user_active
    ON mobile_video_jobs (user_id, state, created_at)
    WHERE state IN ('queued', 'submitting', 'submission_unknown', 'polling');
