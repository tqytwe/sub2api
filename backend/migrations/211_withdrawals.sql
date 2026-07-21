-- CP5 user withdrawal workflow.
-- Keeps withdrawal freezes separate from task/image frozen_balance and stores
-- payout account details only as AES-encrypted snapshots plus masks.

CREATE TABLE IF NOT EXISTS withdrawal_system_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    global_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    minimum_amount NUMERIC(20,8) NOT NULL DEFAULT 10.00000000,
    daily_limit_amount NUMERIC(20,8) NOT NULL DEFAULT 500.00000000,
    double_review_threshold NUMERIC(20,8) NOT NULL DEFAULT 100.00000000,
    reward_maturity_hours INTEGER NOT NULL DEFAULT 72,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawal_system_amounts
        CHECK (
            minimum_amount > 0
            AND daily_limit_amount > 0
            AND double_review_threshold > 0
            AND reward_maturity_hours = 72
        )
);

INSERT INTO withdrawal_system_settings (id, global_enabled, minimum_amount, daily_limit_amount, double_review_threshold, reward_maturity_hours)
VALUES (1, FALSE, 10.00000000, 500.00000000, 100.00000000, 72)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS user_withdrawal_settings (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    minimum_amount_override NUMERIC(20,8),
    daily_limit_amount_override NUMERIC(20,8),
    disabled_reason TEXT NOT NULL DEFAULT '',
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_withdrawal_overrides
        CHECK (
            (minimum_amount_override IS NULL OR minimum_amount_override > 0)
            AND (daily_limit_amount_override IS NULL OR daily_limit_amount_override > 0)
        )
);

CREATE OR REPLACE FUNCTION ensure_user_withdrawal_ready()
RETURNS trigger AS $$
BEGIN
    IF NEW.enabled THEN
        IF NOT EXISTS (
            SELECT 1
            FROM users
            WHERE id = NEW.user_id
              AND deleted_at IS NULL
              AND withdrawal_recalc_status = 'ready'
        ) THEN
            RAISE EXCEPTION 'withdrawal_recalc_status = ready required before enabling withdrawals';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_user_withdrawal_ready ON user_withdrawal_settings;
CREATE TRIGGER trg_user_withdrawal_ready
BEFORE INSERT OR UPDATE OF enabled ON user_withdrawal_settings
FOR EACH ROW EXECUTE FUNCTION ensure_user_withdrawal_ready();

CREATE TABLE IF NOT EXISTS withdrawal_payout_accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method VARCHAR(32) NOT NULL,
    currency VARCHAR(8) NOT NULL,
    recipient_name_mask TEXT NOT NULL DEFAULT '',
    account_mask TEXT NOT NULL,
    account_encrypted TEXT NOT NULL,
    is_current BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawal_payout_method
        CHECK (method IN ('alipay', 'bank_transfer', 'other')),
    CONSTRAINT chk_withdrawal_payout_currency
        CHECK (currency IN ('CNY', 'USD'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_withdrawal_payout_current
    ON withdrawal_payout_accounts(user_id)
    WHERE is_current;

CREATE INDEX IF NOT EXISTS idx_withdrawal_payout_accounts_user_created
    ON withdrawal_payout_accounts(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    request_no VARCHAR(40) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    status VARCHAR(32) NOT NULL DEFAULT 'pending_review',
    payout_method VARCHAR(32) NOT NULL,
    payout_currency VARCHAR(8) NOT NULL,
    payout_account_mask TEXT NOT NULL,
    payout_recipient_name_mask TEXT NOT NULL DEFAULT '',
    account_snapshot_encrypted TEXT NOT NULL,
    submit_balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    close_balance_transaction_id BIGINT REFERENCES balance_transactions(id) ON DELETE SET NULL,
    first_approved_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    first_approved_at TIMESTAMPTZ,
    second_approved_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    second_approved_at TIMESTAMPTZ,
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
    external_fee_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    payout_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawal_request_status
        CHECK (status IN ('pending_review', 'second_review', 'payout_pending', 'paid', 'rejected', 'canceled')),
    CONSTRAINT chk_withdrawal_request_currency
        CHECK (currency IN ('USD')),
    CONSTRAINT chk_withdrawal_request_payout_currency
        CHECK (payout_currency IN ('CNY', 'USD')),
    CONSTRAINT chk_withdrawal_request_method
        CHECK (payout_method IN ('alipay', 'bank_transfer', 'other')),
    CONSTRAINT amount_scale_two_decimals
        CHECK (amount > 0 AND amount = ROUND(amount, 2)),
    CONSTRAINT chk_withdrawal_paid_snapshot
        CHECK (
            status <> 'paid'
            OR (
                paid_at IS NOT NULL
                AND paid_amount IS NOT NULL
                AND paid_amount > 0
                AND paid_currency IS NOT NULL
                AND payout_fx_rate IS NOT NULL
                AND payout_fx_rate > 0
            )
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_withdrawal_requests_user_in_progress
    ON withdrawal_requests(user_id)
    WHERE status IN ('pending_review', 'second_review', 'payout_pending');

CREATE INDEX IF NOT EXISTS idx_withdrawal_requests_status_created
    ON withdrawal_requests(status, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_withdrawal_requests_user_created
    ON withdrawal_requests(user_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS withdrawal_status_events (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_request_id BIGINT NOT NULL REFERENCES withdrawal_requests(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL,
    actor_type VARCHAR(16) NOT NULL DEFAULT 'system',
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    note TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawal_status_events_status
        CHECK (status IN ('pending_review', 'second_review', 'payout_pending', 'paid', 'rejected', 'canceled')),
    CONSTRAINT chk_withdrawal_status_actor
        CHECK (actor_type IN ('user', 'admin', 'system'))
);

CREATE INDEX IF NOT EXISTS idx_withdrawal_status_events_request
    ON withdrawal_status_events(withdrawal_request_id, created_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS withdrawal_request_entitlements (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_request_id BIGINT NOT NULL REFERENCES withdrawal_requests(id) ON DELETE CASCADE,
    entitlement_id BIGINT NOT NULL REFERENCES withdrawable_entitlements(id) ON DELETE RESTRICT,
    amount NUMERIC(20,8) NOT NULL,
    available_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_withdrawal_request_entitlement_amount
        CHECK (amount > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_withdrawal_request_entitlements
    ON withdrawal_request_entitlements(withdrawal_request_id, entitlement_id);

COMMENT ON TABLE withdrawal_system_settings IS 'Global withdrawal switch and default limits; disabled by default';
COMMENT ON TABLE user_withdrawal_settings IS 'Per-user withdrawal allowlist and limit overrides; enabled users must have withdrawal_recalc_status = ready';
COMMENT ON TABLE withdrawal_payout_accounts IS 'Current user payout account with AES-encrypted full details and safe masks';
COMMENT ON TABLE withdrawal_requests IS 'User withdrawal requests reviewed by admins and paid offline';
COMMENT ON TABLE withdrawal_request_entitlements IS 'Exact mature withdrawable entitlement batches locked by each withdrawal request';
