-- Add storefront merchandising fields for subscription plan shelves.
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS storefront_platform VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS storefront_category VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS storefront_featured BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS storefront_badge VARCHAR(64) NOT NULL DEFAULT '';

UPDATE subscription_plans
SET storefront_category = CASE
    WHEN validity_days = 1 OR name ILIKE '%日卡%' OR product_name ILIKE '%日卡%' OR name ILIKE '%daily%' OR product_name ILIKE '%daily%' THEN 'daily'
    WHEN name ILIKE '%团队%' OR product_name ILIKE '%团队%' OR name ILIKE '%team%' OR product_name ILIKE '%team%' THEN 'team'
    WHEN name ILIKE '%企业%' OR product_name ILIKE '%企业%' OR name ILIKE '%enterprise%' OR product_name ILIKE '%enterprise%' THEN 'enterprise'
    WHEN name ILIKE '%额度%' OR product_name ILIKE '%额度%' OR name ILIKE '%credit%' OR product_name ILIKE '%credit%' THEN 'credit'
    WHEN name ILIKE '%图片%' OR product_name ILIKE '%图片%' OR name ILIKE '%image%' OR product_name ILIKE '%image%' THEN 'image'
    ELSE 'pro'
END
WHERE storefront_category = '';

CREATE INDEX IF NOT EXISTS idx_subscription_plans_storefront_platform ON subscription_plans(storefront_platform);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_storefront_category ON subscription_plans(storefront_category);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_storefront_featured ON subscription_plans(storefront_featured);
