CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_orders_coupon_lock_release_due
    ON payment_orders (updated_at, id)
    WHERE coupon_lock_release_processed_at IS NULL
      AND paid_at IS NULL
      AND coupon_id IS NOT NULL
      AND status IN ('CANCELLED', 'EXPIRED', 'FAILED');
