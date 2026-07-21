-- Fund management source batches and refund workflow.
-- This layer classifies the spendable wallet balance by source without
-- rewriting immutable balance_transactions history.

CREATE TABLE IF NOT EXISTS balance_fund_batches (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    payment_order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    source_kind VARCHAR(40) NOT NULL,
    source_type VARCHAR(64) NOT NULL DEFAULT '',
    source_id VARCHAR(160) NOT NULL DEFAULT '',
    original_amount NUMERIC(20,8) NOT NULL,
    remaining_amount NUMERIC(20,8) NOT NULL,
    consumed_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    refunded_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    refund_frozen_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    refundable BOOLEAN NOT NULL DEFAULT FALSE,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(24) NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_balance_fund_batches_kind
        CHECK (source_kind IN (
            'online_recharge',
            'offline_recharge',
            'signup_gift',
            'ops_gift',
            'compensation',
            'redeem_gift',
            'promotion_gift',
            'unknown'
        )),
    CONSTRAINT chk_balance_fund_batches_status
        CHECK (status IN ('active', 'consumed', 'void')),
    CONSTRAINT chk_balance_fund_batches_amounts
        CHECK (
            original_amount >= 0
            AND remaining_amount >= 0
            AND consumed_amount >= 0
            AND refunded_amount >= 0
            AND refund_frozen_amount >= 0
            AND original_amount = remaining_amount + consumed_amount + refunded_amount + refund_frozen_amount
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_balance_fund_batches_balance_transaction
    ON balance_fund_batches(user_id, balance_transaction_id)
    WHERE balance_transaction_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_balance_fund_batches_user_priority
    ON balance_fund_batches(user_id, refundable, source_kind, available_at, id)
    WHERE status = 'active' AND remaining_amount > 0;

CREATE INDEX IF NOT EXISTS idx_balance_fund_batches_user_kind
    ON balance_fund_batches(user_id, source_kind, status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_balance_fund_batches_payment_order
    ON balance_fund_batches(payment_order_id)
    WHERE payment_order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS balance_fund_allocations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    batch_id BIGINT REFERENCES balance_fund_batches(id) ON DELETE SET NULL,
    balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    action VARCHAR(32) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    source_type VARCHAR(64) NOT NULL DEFAULT '',
    source_id VARCHAR(160) NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_balance_fund_allocations_action
        CHECK (action IN ('grant', 'consume', 'restore', 'refund_freeze', 'refund_unfreeze', 'refund_complete', 'reclassify')),
    CONSTRAINT chk_balance_fund_allocations_amount
        CHECK (amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_balance_fund_allocations_user_transaction
    ON balance_fund_allocations(user_id, balance_transaction_id, action);

CREATE INDEX IF NOT EXISTS idx_balance_fund_allocations_batch
    ON balance_fund_allocations(batch_id, created_at, id);

CREATE TABLE IF NOT EXISTS fund_refund_requests (
    id BIGSERIAL PRIMARY KEY,
    request_no VARCHAR(40) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_type VARCHAR(40) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    status VARCHAR(32) NOT NULL DEFAULT 'pending_review',
    reason TEXT NOT NULL DEFAULT '',
    admin_note TEXT NOT NULL DEFAULT '',
    payout_method VARCHAR(32) NOT NULL DEFAULT '',
    payout_currency VARCHAR(8) NOT NULL DEFAULT '',
    payout_account_mask TEXT NOT NULL DEFAULT '',
    payout_recipient_name_mask TEXT NOT NULL DEFAULT '',
    payout_account_snapshot_encrypted TEXT NOT NULL DEFAULT '',
    submit_balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    close_balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    approved_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    rejected_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    rejected_at TIMESTAMPTZ,
    rejected_reason TEXT NOT NULL DEFAULT '',
    canceled_at TIMESTAMPTZ,
    paid_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    paid_at TIMESTAMPTZ,
    paid_amount NUMERIC(20,8),
    paid_currency VARCHAR(8),
    payout_fx_rate NUMERIC(20,8),
    external_txn_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_fund_refund_requests_type
        CHECK (request_type IN ('online_recharge_refund', 'offline_recharge_refund')),
    CONSTRAINT chk_fund_refund_requests_status
        CHECK (status IN ('pending_review', 'payout_pending', 'paid', 'rejected', 'canceled')),
    CONSTRAINT chk_fund_refund_requests_amount
        CHECK (amount > 0 AND amount = TRUNC(amount)),
    CONSTRAINT chk_fund_refund_requests_currency
        CHECK (currency IN ('USD')),
    CONSTRAINT chk_fund_refund_requests_paid
        CHECK (
            status <> 'paid'
            OR (
                paid_at IS NOT NULL
                AND paid_amount IS NOT NULL
                AND paid_amount > 0
                AND paid_amount = TRUNC(paid_amount)
                AND paid_currency IS NOT NULL
                AND payout_fx_rate IS NOT NULL
                AND payout_fx_rate > 0
            )
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_fund_refund_requests_user_in_progress
    ON fund_refund_requests(user_id)
    WHERE status IN ('pending_review', 'payout_pending');

CREATE INDEX IF NOT EXISTS idx_fund_refund_requests_status_created
    ON fund_refund_requests(status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_fund_refund_requests_user_created
    ON fund_refund_requests(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS fund_refund_request_batches (
    id BIGSERIAL PRIMARY KEY,
    fund_refund_request_id BIGINT NOT NULL REFERENCES fund_refund_requests(id) ON DELETE CASCADE,
    batch_id BIGINT NOT NULL REFERENCES balance_fund_batches(id) ON DELETE RESTRICT,
    amount NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_fund_refund_request_batches_amount
        CHECK (amount > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_fund_refund_request_batches
    ON fund_refund_request_batches(fund_refund_request_id, batch_id);

CREATE TABLE IF NOT EXISTS fund_classification_runs (
    id BIGSERIAL PRIMARY KEY,
    mode VARCHAR(16) NOT NULL,
    classification_kind VARCHAR(40) NOT NULL,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    candidate_count INTEGER NOT NULL DEFAULT 0,
    affected_count INTEGER NOT NULL DEFAULT 0,
    report JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_fund_classification_runs_mode
        CHECK (mode IN ('preview', 'execute')),
    CONSTRAINT chk_fund_classification_runs_kind
        CHECK (classification_kind IN ('signup_gift_30'))
);

CREATE INDEX IF NOT EXISTS idx_fund_classification_runs_created
    ON fund_classification_runs(created_at DESC, id DESC);

COMMENT ON TABLE balance_fund_batches IS 'Per-source wallet fund batches for refundable recharge and non-refundable gift balances';
COMMENT ON TABLE fund_refund_requests IS 'User wallet refund requests for unconsumed online/offline recharge funds';
COMMENT ON TABLE fund_classification_runs IS 'Administrator-controlled preview/execute runs for historical fund classification';
