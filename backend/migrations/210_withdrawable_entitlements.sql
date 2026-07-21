-- Withdrawable entitlement accounting foundation.
-- CP4 keeps task/image reserves in users.frozen_balance and introduces a
-- separate withdrawable ledger for mature rights and withdrawal freezes.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS withdrawable_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS withdrawal_frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS withdrawal_recalc_status VARCHAR(24) NOT NULL DEFAULT 'needs_review',
    ADD COLUMN IF NOT EXISTS withdrawal_recalc_checked_at TIMESTAMPTZ;

ALTER TABLE balance_transactions
    ADD COLUMN IF NOT EXISTS withdrawable_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS withdrawable_before NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS withdrawable_after NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS withdrawal_frozen_delta NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS withdrawal_frozen_before NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS withdrawal_frozen_after NUMERIC(20,8);

CREATE TABLE IF NOT EXISTS withdrawable_entitlements (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    source_type VARCHAR(64) NOT NULL,
    source_id VARCHAR(160) NOT NULL DEFAULT '',
    original_amount NUMERIC(20,8) NOT NULL,
    remaining_amount NUMERIC(20,8) NOT NULL,
    consumed_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    withdrawal_frozen_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawable_entitlements_amounts
        CHECK (
            original_amount >= 0
            AND remaining_amount >= 0
            AND consumed_amount >= 0
            AND withdrawal_frozen_amount >= 0
            AND original_amount = remaining_amount + consumed_amount + withdrawal_frozen_amount
        ),
    CONSTRAINT chk_withdrawable_entitlements_status
        CHECK (status IN ('active', 'consumed', 'void'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_withdrawable_entitlements_balance_transaction
    ON withdrawable_entitlements(user_id, balance_transaction_id)
    WHERE balance_transaction_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_withdrawable_entitlements_user_available
    ON withdrawable_entitlements(user_id, status, available_at, id)
    WHERE remaining_amount > 0;

CREATE INDEX IF NOT EXISTS idx_withdrawable_entitlements_user_source
    ON withdrawable_entitlements(user_id, source_type, source_id);

CREATE TABLE IF NOT EXISTS withdrawable_entitlement_allocations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entitlement_id BIGINT REFERENCES withdrawable_entitlements(id) ON DELETE SET NULL,
    balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    action VARCHAR(32) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    available_at TIMESTAMPTZ,
    source_type VARCHAR(64) NOT NULL DEFAULT '',
    source_id VARCHAR(160) NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawable_allocations_action
        CHECK (action IN ('grant', 'consume', 'restore', 'freeze', 'unfreeze', 'recompute_adjustment')),
    CONSTRAINT chk_withdrawable_allocations_amount
        CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_withdrawable_allocations_user_transaction
    ON withdrawable_entitlement_allocations(user_id, balance_transaction_id, action);

CREATE INDEX IF NOT EXISTS idx_withdrawable_allocations_entitlement
    ON withdrawable_entitlement_allocations(entitlement_id, created_at, id);

CREATE TABLE IF NOT EXISTS withdrawable_recalculation_runs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(16) NOT NULL DEFAULT 'dry_run',
    status VARCHAR(24) NOT NULL,
    ledger_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    computed_withdrawable_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    computed_pending_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    anomaly_count INTEGER NOT NULL DEFAULT 0,
    report JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawable_recalc_mode
        CHECK (mode IN ('dry_run', 'execute')),
    CONSTRAINT chk_withdrawable_recalc_status
        CHECK (status IN ('ready', 'needs_review'))
);

CREATE INDEX IF NOT EXISTS idx_withdrawable_recalc_user_created
    ON withdrawable_recalculation_runs(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_users_withdrawal_recalc_status
    ON users(withdrawal_recalc_status)
    WHERE deleted_at IS NULL;

COMMENT ON COLUMN users.frozen_balance IS 'Task and image reservation balance only; not withdrawal frozen funds';
COMMENT ON COLUMN users.withdrawable_balance IS 'Mature withdrawable entitlement balance available for future withdrawal requests';
COMMENT ON COLUMN users.withdrawal_frozen_balance IS 'Funds frozen by withdrawal applications; independent from task/image frozen_balance';
COMMENT ON TABLE withdrawable_entitlements IS 'Per-source withdrawable entitlement batches with availability time and remaining amount';
COMMENT ON TABLE withdrawable_entitlement_allocations IS 'Immutable allocation ledger for grant, consume, restore and withdrawal-freeze actions';
