-- Operational projection for administrator fund actions.  The balance ledger
-- remains the financial source of truth; this table only gives operators a
-- stable public operation reference and an immutable audit chain.
CREATE TABLE IF NOT EXISTS fund_operation_records (
    id BIGSERIAL PRIMARY KEY,
    operation_no VARCHAR(48) NOT NULL UNIQUE,
    operation_kind VARCHAR(40) NOT NULL,
    status VARCHAR(40) NOT NULL DEFAULT 'completed',
    target_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    reason TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    external_ref VARCHAR(160) NOT NULL DEFAULT '',
    root_external_ref VARCHAR(160) NOT NULL DEFAULT '',
    balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    original_operation_id BIGINT REFERENCES fund_operation_records(id) ON DELETE RESTRICT,
    related_operation_id BIGINT REFERENCES fund_operation_records(id) ON DELETE RESTRICT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_fund_operation_kind CHECK (operation_kind IN ('offline_recharge', 'ops_gift', 'compensation', 'refund', 'reversal', 'account_correction')),
    CONSTRAINT chk_fund_operation_status CHECK (status IN ('completed', 'pending', 'canceled', 'pending_insufficient_balance')),
    CONSTRAINT chk_fund_operation_amount CHECK (amount > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_fund_operation_balance_transaction
    ON fund_operation_records(balance_transaction_id) WHERE balance_transaction_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fund_operation_target_created
    ON fund_operation_records(target_user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_fund_operation_actor_created
    ON fund_operation_records(actor_user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_fund_operation_lookup
    ON fund_operation_records(operation_kind, status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_fund_operation_external_ref
    ON fund_operation_records(external_ref) WHERE external_ref <> '';

-- A gateway reference can settle only one offline recharge, even when an
-- operator accidentally selects a different account on a retry.
CREATE UNIQUE INDEX IF NOT EXISTS uq_fund_operation_offline_root_external_ref
    ON fund_operation_records(root_external_ref)
    WHERE operation_kind = 'offline_recharge' AND root_external_ref <> '';

CREATE TABLE IF NOT EXISTS fund_operation_corrections (
    id BIGSERIAL PRIMARY KEY,
    correction_no VARCHAR(48) NOT NULL UNIQUE,
    original_operation_id BIGINT NOT NULL REFERENCES fund_operation_records(id) ON DELETE RESTRICT,
    corrected_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    amount NUMERIC(20,8) NOT NULL,
    status VARCHAR(40) NOT NULL DEFAULT 'pending_insufficient_balance',
    reason TEXT NOT NULL,
    debit_balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    credit_balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    operation_record_id BIGINT REFERENCES fund_operation_records(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_fund_operation_correction_status CHECK (status IN ('completed', 'pending_insufficient_balance', 'canceled')),
    CONSTRAINT chk_fund_operation_correction_amount CHECK (amount > 0)
);
-- A source operation can have only one correction that may still move funds.
-- Canceled corrections remain audit history and do not block a new review.
CREATE UNIQUE INDEX IF NOT EXISTS uq_fund_operation_active_correction
    ON fund_operation_corrections(original_operation_id)
    WHERE status IN ('pending_insufficient_balance', 'completed');
CREATE INDEX IF NOT EXISTS idx_fund_operation_correction_original
    ON fund_operation_corrections(original_operation_id, created_at DESC, id DESC);

-- Backfill only operational credits that were created before this projection.
-- Legacy signup-gift classification records intentionally stay outside this UI.
INSERT INTO fund_operation_records (
    operation_no, operation_kind, status, target_user_id, actor_user_id, amount,
    currency, reason, external_ref, root_external_ref, balance_transaction_id,
    metadata, created_at, completed_at, updated_at
)
SELECT
    'FM-HIST-' || bt.id::text,
    CASE bt.source_type
        WHEN 'offline_recharge' THEN 'offline_recharge'
        WHEN 'compensation' THEN 'compensation'
        ELSE 'ops_gift'
    END,
    'completed', bt.user_id, bt.actor_user_id, bt.balance_delta, 'USD',
    COALESCE(bt.metadata->>'reason', bt.description, ''),
    COALESCE(bt.metadata->>'external_ref', ''), COALESCE(bt.metadata->>'external_ref', ''),
    bt.id, COALESCE(bt.metadata, '{}'::jsonb), bt.created_at, bt.created_at, bt.created_at
FROM balance_transactions bt
WHERE bt.balance_delta > 0
  AND bt.source_type IN ('offline_recharge', 'ops_gift', 'compensation')
ON CONFLICT (balance_transaction_id) WHERE balance_transaction_id IS NOT NULL DO NOTHING;

COMMENT ON TABLE fund_operation_records IS 'Administrator fund-operation projection linked to immutable balance_transactions';
COMMENT ON TABLE fund_operation_corrections IS 'Immutable wrong-account correction audit records; original credits are never overwritten';
