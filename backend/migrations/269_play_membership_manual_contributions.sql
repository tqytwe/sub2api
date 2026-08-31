-- Audited manual/offline payments are membership contributions without a
-- payment_orders foreign key.  They are kept separate from gifts and rewards.
CREATE TABLE IF NOT EXISTS play_membership_manual_contributions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance_transaction_id BIGINT NOT NULL UNIQUE REFERENCES balance_transactions(id) ON DELETE RESTRICT,
    source_type VARCHAR(64) NOT NULL,
    external_ref VARCHAR(160) NOT NULL,
    paid_amount NUMERIC(20,8) NOT NULL,
    refund_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    net_amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
    qualification_state VARCHAR(24) NOT NULL DEFAULT 'verified',
    qualification_source VARCHAR(64) NOT NULL DEFAULT 'manual_review',
    qualification_reason TEXT,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rule_version VARCHAR(32) NOT NULL DEFAULT 'v1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_membership_manual_source_ref UNIQUE (source_type, external_ref),
    CONSTRAINT chk_play_membership_manual_amounts CHECK (
        paid_amount > 0 AND refund_amount >= 0 AND refund_amount <= paid_amount
        AND net_amount = GREATEST(paid_amount - refund_amount, 0)
    ),
    CONSTRAINT chk_play_membership_manual_state CHECK (qualification_state IN ('pending_review', 'verified', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_play_membership_manual_user_state
    ON play_membership_manual_contributions(user_id, qualification_state, paid_at DESC);

CREATE OR REPLACE VIEW play_membership_verified_contributions AS
SELECT order_id, user_id, order_type, paid_amount, refund_amount, net_amount,
       paid_at, status, qualification_state, qualification_source, qualification_reason,
       updated_at
FROM play_membership_order_contributions
WHERE qualification_state = 'verified'
UNION ALL
SELECT NULL::bigint AS order_id, user_id, 'offline_recharge' AS order_type,
       paid_amount, refund_amount, net_amount, paid_at, 'verified' AS status,
       qualification_state, qualification_source, qualification_reason, updated_at
FROM play_membership_manual_contributions
WHERE qualification_state = 'verified';

ALTER TABLE play_membership_tier_history
    DROP CONSTRAINT IF EXISTS play_membership_tier_history_reason_check;
ALTER TABLE play_membership_tier_history
    ADD CONSTRAINT play_membership_tier_history_reason_check
    CHECK (reason IN ('payment','refund','tier_config','offline_recharge','offline_recharge_backfill'));
