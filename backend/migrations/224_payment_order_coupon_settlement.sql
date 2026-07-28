-- Persist coupon settlement inputs on payment orders. These values are kept
-- independently of the mutable coupon template so finance, refunds and
-- affiliate attribution remain auditable after an operator changes a template.

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS coupon_id BIGINT REFERENCES user_coupons(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS coupon_template_id BIGINT REFERENCES coupon_templates(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS coupon_snapshot JSONB,
    ADD COLUMN IF NOT EXISTS list_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gateway_base_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS discount_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fee_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS qualifying_recharge_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payment_currency VARCHAR(3) NOT NULL DEFAULT 'CNY';

-- Existing orders predate list_amount and therefore cannot trust the default
-- above. Preserve their gateway currency for order display and refunds.
UPDATE payment_orders
SET payment_currency = UPPER(provider_snapshot ->> 'currency')
WHERE list_amount = 0
  AND payment_currency = 'CNY'
  AND COALESCE(provider_snapshot ->> 'currency', '') <> '';
