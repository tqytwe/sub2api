-- Unified paid membership accounting and audience targeting.
CREATE TABLE IF NOT EXISTS play_membership_order_contributions (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_type VARCHAR(32) NOT NULL,
    paid_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    refund_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    net_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    paid_at TIMESTAMPTZ,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_play_membership_amounts CHECK (paid_amount >= 0 AND refund_amount >= 0 AND net_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_play_membership_user_net
    ON play_membership_order_contributions(user_id, net_amount);
CREATE INDEX IF NOT EXISTS idx_play_membership_paid_at
    ON play_membership_order_contributions(paid_at);

ALTER TABLE play_campaigns
    ADD COLUMN IF NOT EXISTS audience_json JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_play_campaigns_audience
    ON play_campaigns USING GIN (audience_json);

-- Historical backfill is intentionally idempotent. Only real paid balance and
-- subscription orders are included; reward/manual rows never enter this table.
INSERT INTO play_membership_order_contributions
    (order_id, user_id, order_type, paid_amount, refund_amount, net_amount, paid_at, status)
SELECT id,
       user_id,
       order_type,
       GREATEST(COALESCE(pay_amount, 0), 0),
       GREATEST(COALESCE(refund_amount, 0) * CASE WHEN amount > 0 THEN pay_amount / amount ELSE 0 END, 0),
       GREATEST(COALESCE(pay_amount, 0) - COALESCE(refund_amount, 0) * CASE WHEN amount > 0 THEN pay_amount / amount ELSE 0 END, 0),
       paid_at,
       CASE WHEN status IN ('REFUNDED', 'PARTIALLY_REFUNDED') THEN status ELSE 'active' END
FROM payment_orders
WHERE order_type IN ('balance', 'subscription')
  AND status IN ('PAID', 'RECHARGING', 'COMPLETED', 'REFUNDED', 'PARTIALLY_REFUNDED')
ON CONFLICT (order_id) DO UPDATE SET
    paid_amount = EXCLUDED.paid_amount,
    refund_amount = EXCLUDED.refund_amount,
    net_amount = EXCLUDED.net_amount,
    paid_at = EXCLUDED.paid_at,
    status = EXCLUDED.status,
    updated_at = NOW();
