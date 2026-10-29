-- Quality operations core. This migration is additive and keeps legacy plans
-- usable while introducing immutable run snapshots and ownership state.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS auto_recover BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS pelican_config JSONB;
ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS running_until TIMESTAMPTZ;

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS pelican_config JSONB;
ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS quality_round_id TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS quality_action TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS quality_judgment JSONB;

CREATE INDEX IF NOT EXISTS idx_scheduled_test_results_quality_round
    ON scheduled_test_results(plan_id, quality_round_id, created_at DESC);

CREATE TABLE IF NOT EXISTS account_quality_states (
    plan_id BIGINT PRIMARY KEY REFERENCES scheduled_test_plans(id) ON DELETE CASCADE,
    state JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One editable quality rule per account and action scope. Legacy plans have a
-- NULL config and are unaffected by this partial unique index.
CREATE UNIQUE INDEX IF NOT EXISTS scheduled_test_quality_account_scope_unique
    ON scheduled_test_plans (
        account_id,
        (CASE pelican_config->'quality'->>'action'
            WHEN 'enable_bps' THEN 'bps'
            WHEN 'observe_only' THEN 'observation'
            ELSE 'quarantine'
        END)
    )
    WHERE pelican_config->'quality' IS NOT NULL;

COMMENT ON TABLE account_quality_states IS
    'Opaque quality ownership snapshots. Mutations require a preview fence and explicit confirmation.';
