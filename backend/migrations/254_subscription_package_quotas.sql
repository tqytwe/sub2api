-- Package quota plans are immutable at purchase time. Existing plans remain
-- legacy plans until an administrator explicitly configures at least one cap.
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS request_limit BIGINT,
    ADD COLUMN IF NOT EXISTS amount_limit_usd DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS token_limit BIGINT;

ALTER TABLE subscription_plans
    DROP CONSTRAINT IF EXISTS subscription_plans_package_quota_check;
ALTER TABLE subscription_plans
    ADD CONSTRAINT subscription_plans_package_quota_check CHECK (
        (request_limit IS NULL OR request_limit > 0)
        AND (amount_limit_usd IS NULL OR amount_limit_usd > 0)
        AND (token_limit IS NULL OR token_limit > 0)
    );

CREATE TABLE IF NOT EXISTS subscription_package_entitlements (
    id BIGSERIAL PRIMARY KEY,
    payment_order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    exhausted_reason VARCHAR(20),
    request_limit BIGINT,
    request_used BIGINT NOT NULL DEFAULT 0,
    amount_limit_usd DECIMAL(20,10),
    amount_used_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    token_limit BIGINT,
    token_used BIGINT NOT NULL DEFAULT 0,
    plan_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at > starts_at),
    CHECK (status IN ('active', 'exhausted', 'expired', 'revoked')),
    CHECK (exhausted_reason IS NULL OR exhausted_reason IN ('request', 'amount', 'token')),
    CHECK (request_limit IS NULL OR request_limit > 0),
    CHECK (amount_limit_usd IS NULL OR amount_limit_usd > 0),
    CHECK (token_limit IS NULL OR token_limit > 0),
    CHECK (request_used >= 0 AND amount_used_usd >= 0 AND token_used >= 0)
);

CREATE INDEX IF NOT EXISTS idx_subscription_package_entitlements_active
    ON subscription_package_entitlements (user_id, group_id, expires_at, id)
    WHERE status = 'active';
