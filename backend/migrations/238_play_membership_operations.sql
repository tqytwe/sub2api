-- Auditable VIP configuration and membership tier changes.
CREATE TABLE IF NOT EXISTS play_membership_tier_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    from_tier INT NOT NULL,
    to_tier INT NOT NULL,
    net_paid_before NUMERIC(20,8) NOT NULL DEFAULT 0,
    net_paid_after NUMERIC(20,8) NOT NULL DEFAULT 0,
    reason VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT play_membership_tier_history_tier_check CHECK (from_tier >= 0 AND to_tier >= 0 AND from_tier <> to_tier),
    CONSTRAINT play_membership_tier_history_amount_check CHECK (net_paid_before >= 0 AND net_paid_after >= 0),
    CONSTRAINT play_membership_tier_history_reason_check CHECK (reason IN ('payment','refund','tier_config'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_play_membership_tier_history_order_transition
    ON play_membership_tier_history(order_id, from_tier, to_tier)
    WHERE order_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_play_membership_tier_history_user_created
    ON play_membership_tier_history(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_play_membership_tier_history_created
    ON play_membership_tier_history(created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS play_vip_config_versions (
    version BIGSERIAL PRIMARY KEY,
    tiers_json JSONB NOT NULL,
    actor_admin_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL,
    affected_users INT NOT NULL DEFAULT 0,
    upgraded_users INT NOT NULL DEFAULT 0,
    downgraded_users INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT play_vip_config_reason_check CHECK (char_length(reason) BETWEEN 10 AND 500),
    CONSTRAINT play_vip_config_impact_check CHECK (affected_users >= 0 AND upgraded_users >= 0 AND downgraded_users >= 0)
);

CREATE INDEX IF NOT EXISTS idx_play_vip_config_versions_created
    ON play_vip_config_versions(created_at DESC, version DESC);
