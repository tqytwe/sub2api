-- Online indexes for coupon settlement lookups. payment_orders is a hot table,
-- so these must run outside the default migration transaction.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_orders_coupon_id
    ON payment_orders (coupon_id)
    WHERE coupon_id IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_orders_coupon_template_id
    ON payment_orders (coupon_template_id)
    WHERE coupon_template_id IS NOT NULL;
