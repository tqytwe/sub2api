-- Separate verified VIP accounting from the legacy migration 234 estimate.
ALTER TABLE IF EXISTS play_membership_order_contributions
    ADD COLUMN IF NOT EXISTS qualification_state VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    ADD COLUMN IF NOT EXISTS qualification_source VARCHAR(32) NOT NULL DEFAULT 'legacy_backfill',
    ADD COLUMN IF NOT EXISTS qualification_reason TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE IF EXISTS payment_orders
    ADD COLUMN IF NOT EXISTS subscription_snapshot JSONB;

ALTER TABLE IF EXISTS play_membership_order_contributions
    DROP CONSTRAINT IF EXISTS play_membership_qualification_state_check;

ALTER TABLE IF EXISTS play_membership_order_contributions
    ADD CONSTRAINT play_membership_qualification_state_check
    CHECK (qualification_state IN ('verified', 'pending_review', 'rejected'));

CREATE INDEX IF NOT EXISTS idx_play_membership_verified_user_net
    ON play_membership_order_contributions(user_id, net_amount)
    WHERE qualification_state = 'verified';

CREATE INDEX IF NOT EXISTS idx_play_membership_pending_review
    ON play_membership_order_contributions(order_id)
    WHERE qualification_state = 'pending_review';
