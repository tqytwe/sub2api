-- Preserve already-created periods until an operator reviews the v2 delta.
-- New periods use all four token categories. Never rewrite payout evidence.
ALTER TABLE play_arena_periods
    ADD COLUMN IF NOT EXISTS token_accounting_version SMALLINT NOT NULL DEFAULT 1
        CHECK (token_accounting_version IN (1, 2));
ALTER TABLE play_arena_periods ALTER COLUMN token_accounting_version SET DEFAULT 2;

COMMENT ON COLUMN play_arena_periods.token_accounting_version IS
    '1: input/output/cache creation; 2: also cache read. Existing periods require reviewed activation.';

CREATE TABLE IF NOT EXISTS play_arena_accounting_events (
    id BIGSERIAL PRIMARY KEY,
    period_id BIGINT NOT NULL REFERENCES play_arena_periods(id),
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    from_version SMALLINT NOT NULL,
    to_version SMALLINT NOT NULL,
    preview_digest TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (period_id,to_version)
);
