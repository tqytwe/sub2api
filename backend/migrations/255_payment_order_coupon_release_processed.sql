ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS coupon_lock_release_processed_at TIMESTAMPTZ NULL;

-- Historical terminal orders whose coupon is no longer locked by that order
-- have already reached an unreleasable state. Mark them once so the periodic
-- cleanup does not rescan them forever. Orders that still own the lock remain
-- pending and will be released by the normal transactional cleanup path.
UPDATE payment_orders AS po
SET coupon_lock_release_processed_at = NOW()
WHERE po.coupon_lock_release_processed_at IS NULL
  AND po.paid_at IS NULL
  AND po.coupon_id IS NOT NULL
  AND po.status IN ('CANCELLED', 'EXPIRED', 'FAILED')
  AND NOT EXISTS (
      SELECT 1
      FROM user_coupons AS uc
      WHERE uc.id = po.coupon_id
        AND uc.locked_order_id = po.id
  );
