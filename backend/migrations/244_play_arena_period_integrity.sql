-- A period is the settlement boundary, so concurrent first reads must never
-- create two active rows for the same day or month. Without this constraint,
-- duplicate period IDs could produce duplicate idempotency keys and payouts.

DO $$
BEGIN
    -- Never silently alter a duplicate period which already has payout
    -- evidence. Stop the rollout so finance can reconcile that period first.
    IF EXISTS (
        WITH ranked_active_periods AS (
            SELECT
                p.id,
                ROW_NUMBER() OVER (
                    PARTITION BY p.period_type, p.start_at
                    ORDER BY p.created_at ASC, p.id ASC
                ) AS row_number
            FROM play_arena_periods p
            WHERE p.status = 'active'
        )
        SELECT 1
        FROM ranked_active_periods duplicate_period
        JOIN play_reward_ledger ledger
          ON ledger.source IN ('arena_settlement', 'arena_daily_settlement')
         AND ledger.detail ->> 'period_id' = duplicate_period.id::text
        WHERE duplicate_period.row_number > 1
    ) THEN
        RAISE EXCEPTION
            'duplicate active play arena periods have payout evidence; reconcile them before applying migration 244';
    END IF;
END
$$;

-- Old accidental duplicates without payout evidence are retained as drafts for
-- audit rather than deleted or marked as settled. The earliest row remains the
-- canonical live period.
WITH ranked_active_periods AS (
    SELECT
        p.id,
        ROW_NUMBER() OVER (
            PARTITION BY p.period_type, p.start_at
            ORDER BY p.created_at ASC, p.id ASC
        ) AS row_number
    FROM play_arena_periods p
    WHERE p.status = 'active'
)
UPDATE play_arena_periods period
SET status = 'draft', updated_at = NOW()
FROM ranked_active_periods duplicate_period
WHERE period.id = duplicate_period.id
  AND duplicate_period.row_number > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_play_arena_periods_active_type_start
    ON play_arena_periods (period_type, start_at)
    WHERE status = 'active';

COMMENT ON INDEX uq_play_arena_periods_active_type_start IS
    'Exactly one active arena period may exist for each type and Shanghai start time.';
