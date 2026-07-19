-- Unified user balance/frozen-balance ledger.
-- All future changes to users.balance or users.frozen_balance should write one row here.

CREATE TABLE IF NOT EXISTS balance_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    balance_before NUMERIC(20,8),
    balance_after NUMERIC(20,8),
    frozen_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    frozen_before NUMERIC(20,8),
    frozen_after NUMERIC(20,8),
    source_type VARCHAR(64) NOT NULL,
    source_id VARCHAR(128) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(160) NOT NULL,
    actor_type VARCHAR(32) NOT NULL DEFAULT 'system',
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    description TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_backfilled BOOLEAN NOT NULL DEFAULT FALSE,
    confidence VARCHAR(24) NOT NULL DEFAULT 'high',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_balance_transactions_confidence
        CHECK (confidence IN ('high', 'medium', 'low', 'estimated', 'needs_review'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_balance_transactions_user_idempotency
    ON balance_transactions(user_id, idempotency_key);

CREATE UNIQUE INDEX IF NOT EXISTS uq_balance_transactions_source
    ON balance_transactions(user_id, source_type, source_id)
    WHERE source_id <> '';

CREATE INDEX IF NOT EXISTS idx_balance_transactions_user_created
    ON balance_transactions(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_balance_transactions_source_type
    ON balance_transactions(source_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_balance_transactions_backfill_confidence
    ON balance_transactions(is_backfilled, confidence, created_at DESC);

COMMENT ON TABLE balance_transactions IS 'Unified immutable ledger for users.balance and users.frozen_balance changes';
COMMENT ON COLUMN balance_transactions.idempotency_key IS 'Stable operation key; unique per user to make awards/charges retry-safe';
COMMENT ON COLUMN balance_transactions.confidence IS 'Backfill confidence: high/medium/low/estimated/needs_review';
