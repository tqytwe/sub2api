-- Internal platform surcharge layer.
-- Surcharge is used only for actual deductions/quota accounting. Existing
-- rate_multiplier and user-facing usage actual_cost remain the pre-surcharge
-- display/history values.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS billing_surcharge_override_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS billing_surcharge_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS billing_surcharge_mode VARCHAR(32) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS billing_surcharge_value DECIMAL(12,6) NOT NULL DEFAULT 0;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS billing_surcharge_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billed_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billing_surcharge_mode VARCHAR(32) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS billing_surcharge_value DECIMAL(12,6) NOT NULL DEFAULT 0;

INSERT INTO settings (key, value, updated_at)
VALUES
    ('billing_surcharge_enabled', 'false', NOW()),
    ('billing_surcharge_mode', 'none', NOW()),
    ('billing_surcharge_value', '0', NOW())
ON CONFLICT (key) DO NOTHING;

COMMENT ON COLUMN groups.billing_surcharge_override_enabled IS '是否覆盖全局内部手续费配置';
COMMENT ON COLUMN groups.billing_surcharge_enabled IS '是否启用分组内部手续费；仅影响实际扣款，不改变展示倍率';
COMMENT ON COLUMN groups.billing_surcharge_mode IS '内部手续费模式：none/percent_on_charged_cost/additive_multiplier';
COMMENT ON COLUMN groups.billing_surcharge_value IS '内部手续费值；按模式解释';
COMMENT ON COLUMN usage_logs.billing_surcharge_cost IS '内部手续费金额；用户展示历史不使用';
COMMENT ON COLUMN usage_logs.billed_cost IS '实际扣款金额，等于 actual_cost + billing_surcharge_cost；0 表示历史记录未快照';
COMMENT ON COLUMN usage_logs.billing_surcharge_mode IS '内部手续费模式快照';
COMMENT ON COLUMN usage_logs.billing_surcharge_value IS '内部手续费值快照';
