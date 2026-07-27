-- Payment coupon templates are intentionally separate from promo_codes, which
-- remain registration-time balance grants for backwards compatibility.

CREATE TABLE IF NOT EXISTS coupon_templates (
    id BIGSERIAL PRIMARY KEY,
    template_key VARCHAR(64) NOT NULL UNIQUE,
    version INTEGER NOT NULL DEFAULT 1,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    benefit_type VARCHAR(20) NOT NULL,
    benefit_value DECIMAL(20, 4) NOT NULL,
    max_discount_amount DECIMAL(20, 4),
    currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    applicable_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    minimum_order_amount DECIMAL(20, 4) NOT NULL DEFAULT 0,
    eligible_plan_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    validity_mode VARCHAR(20) NOT NULL,
    validity_days INTEGER NOT NULL DEFAULT 0,
    fixed_expires_at TIMESTAMPTZ,
    valid_from TIMESTAMPTZ,
    total_issue_limit BIGINT,
    issued_count BIGINT NOT NULL DEFAULT 0,
    rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupon_templates_status_check CHECK (status IN ('draft', 'active', 'paused', 'archived')),
    CONSTRAINT coupon_templates_benefit_type_check CHECK (benefit_type IN ('fixed_amount', 'percentage')),
    CONSTRAINT coupon_templates_benefit_value_check CHECK (benefit_value > 0),
    CONSTRAINT coupon_templates_max_discount_check CHECK (max_discount_amount IS NULL OR max_discount_amount > 0),
    CONSTRAINT coupon_templates_minimum_amount_check CHECK (minimum_order_amount >= 0),
    CONSTRAINT coupon_templates_validity_mode_check CHECK (validity_mode IN ('relative_days', 'end_of_day', 'end_of_month', 'fixed')),
    CONSTRAINT coupon_templates_validity_check CHECK (
        (validity_mode = 'relative_days' AND validity_days > 0 AND fixed_expires_at IS NULL)
        OR (validity_mode IN ('end_of_day', 'end_of_month') AND validity_days = 0 AND fixed_expires_at IS NULL)
        OR (validity_mode = 'fixed' AND validity_days = 0 AND fixed_expires_at IS NOT NULL)
    ),
    CONSTRAINT coupon_templates_total_issue_limit_check CHECK (total_issue_limit IS NULL OR total_issue_limit > 0),
    CONSTRAINT coupon_templates_issued_count_check CHECK (issued_count >= 0),
    CONSTRAINT coupon_templates_issue_limit_check CHECK (total_issue_limit IS NULL OR issued_count <= total_issue_limit)
);

CREATE INDEX IF NOT EXISTS idx_coupon_templates_status_updated_at
    ON coupon_templates (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS coupon_issue_batches (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE RESTRICT,
    source VARCHAR(20) NOT NULL DEFAULT 'admin_batch',
    requested_count INTEGER NOT NULL,
    issued_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'completed',
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupon_issue_batches_source_check CHECK (source IN ('blindbox', 'quiz', 'admin_batch', 'manual', 'compensation')),
    CONSTRAINT coupon_issue_batches_status_check CHECK (status IN ('completed', 'failed')),
    CONSTRAINT coupon_issue_batches_counts_check CHECK (
        requested_count > 0 AND issued_count >= 0 AND failed_count >= 0 AND issued_count + failed_count <= requested_count
    )
);

CREATE INDEX IF NOT EXISTS idx_coupon_issue_batches_template_created_at
    ON coupon_issue_batches (template_id, created_at DESC);

CREATE TABLE IF NOT EXISTS user_coupons (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'available',
    terms_snapshot JSONB NOT NULL,
    source VARCHAR(20) NOT NULL,
    source_ref VARCHAR(256) NOT NULL DEFAULT '',
    issue_batch_id BIGINT REFERENCES coupon_issue_batches(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_from TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    locked_order_id BIGINT REFERENCES payment_orders(id) ON DELETE RESTRICT,
    locked_at TIMESTAMPTZ,
    used_order_id BIGINT REFERENCES payment_orders(id) ON DELETE RESTRICT,
    used_at TIMESTAMPTZ,
    voided_at TIMESTAMPTZ,
    void_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_coupons_status_check CHECK (status IN ('available', 'locked', 'used', 'expired', 'voided')),
    CONSTRAINT user_coupons_source_check CHECK (source IN ('blindbox', 'quiz', 'admin_batch', 'manual', 'compensation')),
    CONSTRAINT user_coupons_validity_check CHECK (expires_at > valid_from),
    CONSTRAINT user_coupons_lock_check CHECK (
        (status = 'locked' AND locked_order_id IS NOT NULL AND locked_at IS NOT NULL)
        OR (status <> 'locked')
    ),
    CONSTRAINT user_coupons_used_check CHECK (
        (status = 'used' AND used_order_id IS NOT NULL AND used_at IS NOT NULL)
        OR (status <> 'used')
    )
);

CREATE INDEX IF NOT EXISTS idx_user_coupons_user_status_expires_at
    ON user_coupons (user_id, status, expires_at ASC);
CREATE INDEX IF NOT EXISTS idx_user_coupons_template_created_at
    ON user_coupons (template_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_coupons_locked_order
    ON user_coupons (locked_order_id) WHERE locked_order_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_coupons_used_order
    ON user_coupons (used_order_id) WHERE used_order_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS coupon_reward_pool_versions (
    id BIGSERIAL PRIMARY KEY,
    activity VARCHAR(16) NOT NULL,
    version VARCHAR(80) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    coupon_weight_bp INTEGER NOT NULL,
    balance_weight_bp INTEGER NOT NULL,
    fallback_template_id BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE RESTRICT,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupon_reward_pool_activity_check CHECK (activity IN ('blindbox', 'quiz')),
    CONSTRAINT coupon_reward_pool_status_check CHECK (status IN ('draft', 'published', 'retired')),
    CONSTRAINT coupon_reward_pool_outer_weights_check CHECK (coupon_weight_bp >= 0 AND balance_weight_bp >= 0 AND coupon_weight_bp + balance_weight_bp = 10000),
    UNIQUE (activity, version)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_coupon_reward_pool_one_published_per_activity
    ON coupon_reward_pool_versions (activity) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_coupon_reward_pool_versions_activity_status
    ON coupon_reward_pool_versions (activity, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS coupon_reward_pool_entries (
    id BIGSERIAL PRIMARY KEY,
    pool_version_id BIGINT NOT NULL REFERENCES coupon_reward_pool_versions(id) ON DELETE CASCADE,
    template_id BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE RESTRICT,
    weight_bp INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    stock_cap BIGINT,
    issued_count BIGINT NOT NULL DEFAULT 0,
    per_user_issue_limit INTEGER,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupon_reward_pool_entries_weight_check CHECK (weight_bp > 0 AND weight_bp <= 10000),
    CONSTRAINT coupon_reward_pool_entries_stock_check CHECK (stock_cap IS NULL OR stock_cap > 0),
    CONSTRAINT coupon_reward_pool_entries_stock_issued_check CHECK (stock_cap IS NULL OR issued_count <= stock_cap),
    CONSTRAINT coupon_reward_pool_entries_issued_check CHECK (issued_count >= 0),
    CONSTRAINT coupon_reward_pool_entries_per_user_check CHECK (per_user_issue_limit IS NULL OR per_user_issue_limit > 0),
    CONSTRAINT coupon_reward_pool_entries_window_check CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at),
    UNIQUE (pool_version_id, template_id)
);

CREATE INDEX IF NOT EXISTS idx_coupon_reward_pool_entries_pool_order
    ON coupon_reward_pool_entries (pool_version_id, sort_order, id);

CREATE TABLE IF NOT EXISTS coupon_reward_draws (
    id BIGSERIAL PRIMARY KEY,
    activity VARCHAR(16) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pool_version_id BIGINT NOT NULL REFERENCES coupon_reward_pool_versions(id) ON DELETE RESTRICT,
    pool_entry_id BIGINT NOT NULL REFERENCES coupon_reward_pool_entries(id) ON DELETE RESTRICT,
    template_id BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE RESTRICT,
    user_coupon_id BIGINT NOT NULL REFERENCES user_coupons(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,
    source_ref VARCHAR(256) NOT NULL DEFAULT '',
    fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupon_reward_draws_activity_check CHECK (activity IN ('blindbox', 'quiz'))
);

CREATE INDEX IF NOT EXISTS idx_coupon_reward_draws_user_created_at
    ON coupon_reward_draws (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_coupon_reward_draws_pool_created_at
    ON coupon_reward_draws (pool_version_id, created_at DESC);

CREATE TABLE IF NOT EXISTS coupon_events (
    id BIGSERIAL PRIMARY KEY,
    coupon_id BIGINT REFERENCES user_coupons(id) ON DELETE SET NULL,
    template_id BIGINT REFERENCES coupon_templates(id) ON DELETE SET NULL,
    issue_batch_id BIGINT REFERENCES coupon_issue_batches(id) ON DELETE SET NULL,
    event_type VARCHAR(48) NOT NULL,
    actor_type VARCHAR(24) NOT NULL DEFAULT 'system',
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coupon_events_coupon_created_at
    ON coupon_events (coupon_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_coupon_events_template_created_at
    ON coupon_events (template_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_coupon_events_order_created_at
    ON coupon_events (order_id, created_at DESC) WHERE order_id IS NOT NULL;
