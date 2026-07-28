-- 通用营销奖励奖池：支持签到、兑换码分支、奖池配置 JSON 与兑换码发放追踪。

ALTER TABLE coupon_reward_pool_versions
    ADD COLUMN IF NOT EXISTS redeem_code_weight_bp INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reward_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE coupon_reward_pool_versions
    DROP CONSTRAINT IF EXISTS coupon_reward_pool_activity_check,
    DROP CONSTRAINT IF EXISTS coupon_reward_pool_outer_weights_check;

ALTER TABLE coupon_reward_pool_versions
    ADD CONSTRAINT coupon_reward_pool_activity_check CHECK (activity IN ('blindbox', 'quiz', 'checkin')),
    ADD CONSTRAINT coupon_reward_pool_outer_weights_check CHECK (
        coupon_weight_bp >= 0
        AND redeem_code_weight_bp >= 0
        AND balance_weight_bp >= 0
        AND coupon_weight_bp + redeem_code_weight_bp + balance_weight_bp = 10000
    );

ALTER TABLE coupon_reward_draws
    DROP CONSTRAINT IF EXISTS coupon_reward_draws_activity_check;

ALTER TABLE coupon_reward_draws
    ADD CONSTRAINT coupon_reward_draws_activity_check CHECK (activity IN ('blindbox', 'quiz', 'checkin'));

ALTER TABLE coupon_issue_batches
    DROP CONSTRAINT IF EXISTS coupon_issue_batches_source_check;

ALTER TABLE coupon_issue_batches
    ADD CONSTRAINT coupon_issue_batches_source_check CHECK (source IN ('blindbox', 'quiz', 'checkin', 'admin_batch', 'manual', 'compensation'));

ALTER TABLE user_coupons
    DROP CONSTRAINT IF EXISTS user_coupons_source_check;

ALTER TABLE user_coupons
    ADD CONSTRAINT user_coupons_source_check CHECK (source IN ('blindbox', 'quiz', 'checkin', 'admin_batch', 'manual', 'compensation'));

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS batch_name TEXT,
    ADD COLUMN IF NOT EXISTS batch_tag TEXT,
    ADD COLUMN IF NOT EXISTS issued_to BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS issued_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS issue_source TEXT,
    ADD COLUMN IF NOT EXISTS issue_ref TEXT,
    ADD COLUMN IF NOT EXISTS reward_pool_version TEXT;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_batch_status
    ON redeem_codes (batch_name, status, expires_at);

CREATE INDEX IF NOT EXISTS idx_redeem_codes_issued_to
    ON redeem_codes (issued_to, issued_at DESC);
