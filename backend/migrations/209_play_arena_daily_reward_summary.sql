-- 209_play_arena_daily_reward_summary.sql
-- Track explicit daily arena settlement time for public reward summaries.

ALTER TABLE play_arena_periods
    ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ;

UPDATE play_arena_periods
SET settled_at = updated_at
WHERE status = 'settled' AND settled_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_play_arena_periods_daily_settled
    ON play_arena_periods(settled_at DESC, end_at DESC, id DESC)
    WHERE period_type = 'daily' AND status = 'settled';

COMMENT ON COLUMN play_arena_periods.settled_at IS 'Time when a ranking period was marked settled; backfilled from updated_at for historical settled rows.';
