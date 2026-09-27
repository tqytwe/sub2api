ALTER TABLE subscription_package_entitlements
    ALTER COLUMN payment_order_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(20) NOT NULL DEFAULT 'payment',
    ADD COLUMN IF NOT EXISTS granted_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS grant_key VARCHAR(128);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_package_entitlements_grant_key
    ON subscription_package_entitlements (grant_key)
    WHERE grant_key IS NOT NULL;

ALTER TABLE subscription_package_entitlements
    DROP CONSTRAINT IF EXISTS subscription_package_entitlements_source_check;
ALTER TABLE subscription_package_entitlements
    ADD CONSTRAINT subscription_package_entitlements_source_check CHECK (
        (source_type = 'payment' AND payment_order_id IS NOT NULL AND granted_by IS NULL AND grant_key IS NULL)
        OR
        (source_type = 'admin_grant' AND payment_order_id IS NULL AND grant_key IS NOT NULL)
    );
