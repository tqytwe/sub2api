-- Referral growth campaigns: immutable attribution, paid/usage qualification,
-- budget-reserved reward ledger, approval/audit state, and refund reconciliation.

CREATE TABLE IF NOT EXISTS referral_campaigns (
    id BIGSERIAL PRIMARY KEY,
    campaign_key VARCHAR(96) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    registration_from TIMESTAMPTZ NOT NULL,
    registration_to TIMESTAMPTZ NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    qualification_to TIMESTAMPTZ NOT NULL,
    claim_deadline TIMESTAMPTZ NOT NULL,
    risk_hold_hours INTEGER NOT NULL DEFAULT 168,
    pay_threshold NUMERIC(20,8) NOT NULL DEFAULT 0,
    usage_threshold NUMERIC(20,8) NOT NULL DEFAULT 0,
    max_enrollments INTEGER NOT NULL DEFAULT 0,
    budget_total NUMERIC(20,8) NOT NULL DEFAULT 0,
    budget_reserved NUMERIC(20,8) NOT NULL DEFAULT 0,
    budget_paid NUMERIC(20,8) NOT NULL DEFAULT 0,
    reward_mode VARCHAR(16) NOT NULL DEFAULT 'additive',
    rank_rewards_json JSONB NOT NULL DEFAULT '{}',
    signing_secret BYTEA NOT NULL,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    approved_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT referral_campaign_status_check CHECK (status IN ('draft','review','approved','scheduled','running','paused','settling','closed','cancelled')),
    CONSTRAINT referral_campaign_reward_mode_check CHECK (reward_mode IN ('additive','replace')),
    CONSTRAINT referral_campaign_window_check CHECK (registration_to > registration_from AND ends_at > starts_at AND qualification_to >= ends_at AND claim_deadline > ends_at),
    CONSTRAINT referral_campaign_amount_check CHECK (risk_hold_hours BETWEEN 168 AND 720 AND pay_threshold >= 0 AND usage_threshold >= 0 AND max_enrollments > 0 AND budget_total > 0 AND budget_reserved >= 0 AND budget_paid >= 0 AND budget_reserved + budget_paid <= budget_total)
);

CREATE TABLE IF NOT EXISTS referral_campaign_tiers (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    tier_no INTEGER NOT NULL,
    required_invites INTEGER NOT NULL,
    reward_amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, tier_no),
    CONSTRAINT referral_campaign_tier_check CHECK (tier_no > 0 AND required_invites > 0 AND reward_amount > 0)
);

CREATE TABLE IF NOT EXISTS referral_campaign_enrollments (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, user_id)
);

CREATE TABLE IF NOT EXISTS referral_campaign_attributions (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_nonce VARCHAR(128) NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, invitee_id),
    CONSTRAINT referral_campaign_attribution_self_check CHECK (inviter_id <> invitee_id),
    CONSTRAINT referral_campaign_attribution_status_check CHECK (status IN ('pending','approved','rejected','revoked'))
);

CREATE TABLE IF NOT EXISTS referral_campaign_qualifications (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    net_paid NUMERIC(20,8) NOT NULL DEFAULT 0,
    actual_cost NUMERIC(20,8) NOT NULL DEFAULT 0,
    source_order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    risk_status VARCHAR(24) NOT NULL DEFAULT 'pending',
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    qualified_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revocation_reason VARCHAR(500) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, invitee_id),
    CONSTRAINT referral_campaign_qualification_amount_check CHECK (net_paid >= 0 AND actual_cost >= 0),
    CONSTRAINT referral_campaign_qualification_status_check CHECK (risk_status IN ('pending','approved','rejected') AND status IN ('pending','qualified','revoked'))
);

CREATE TABLE IF NOT EXISTS referral_campaign_rewards (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier_no INTEGER NOT NULL DEFAULT 0,
    reward_type VARCHAR(24) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    status VARCHAR(24) NOT NULL DEFAULT 'claimable',
    version BIGINT NOT NULL DEFAULT 1,
    generation INTEGER NOT NULL DEFAULT 1,
    qualification_id BIGINT REFERENCES referral_campaign_qualifications(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    unlock_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claim_deadline TIMESTAMPTZ NOT NULL,
    frozen_until TIMESTAMPTZ,
    claimed_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, user_id, reward_type, tier_no, generation),
    CONSTRAINT referral_campaign_reward_amount_check CHECK (amount > 0),
    CONSTRAINT referral_campaign_reward_status_check CHECK (status IN ('claimable','claimed_frozen','available','expired','revoked','debt_review','resolved'))
);

CREATE TABLE IF NOT EXISTS referral_campaign_budget_ledger (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    reward_id BIGINT REFERENCES referral_campaign_rewards(id) ON DELETE SET NULL,
    action VARCHAR(24) NOT NULL,
    amount NUMERIC(20,8) NOT NULL,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT referral_campaign_budget_action_check CHECK (action IN ('reserve','release','pay','reverse'))
);

CREATE TABLE IF NOT EXISTS referral_campaign_approvals (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL,
    review_type VARCHAR(24) NOT NULL,
    decision VARCHAR(24) NOT NULL,
    reviewer_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    note VARCHAR(2000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, version, review_type),
    UNIQUE (campaign_id, version, reviewer_id),
    CONSTRAINT referral_campaign_approval_check CHECK (review_type IN ('ops','finance','risk','ux') AND decision IN ('approved','rejected'))
);

CREATE TABLE IF NOT EXISTS referral_campaign_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT REFERENCES referral_campaigns(id) ON DELETE SET NULL,
    campaign_version BIGINT,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(48) NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS referral_campaign_reconcile_queue (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT REFERENCES referral_campaigns(id) ON DELETE SET NULL,
    invitee_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    source_order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    action VARCHAR(24) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, invitee_id, source_order_id, action),
    CONSTRAINT referral_campaign_queue_check CHECK (action IN ('qualify','refund_revoke') AND status IN ('pending','processing','done','failed'))
);

CREATE INDEX IF NOT EXISTS idx_referral_campaign_status_window ON referral_campaigns(status, starts_at, ends_at);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_attribution_inviter ON referral_campaign_attributions(campaign_id, inviter_id, registered_at DESC);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_attribution_nonce ON referral_campaign_attributions(campaign_id, token_nonce);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_qualification_inviter ON referral_campaign_qualifications(campaign_id, inviter_id, status);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_reward_user ON referral_campaign_rewards(campaign_id, user_id, status, claim_deadline);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_queue_pending ON referral_campaign_reconcile_queue(status, available_at, id);

-- Campaign rewards use the existing affiliate wallet, but retain a direct,
-- unique source pointer so retries and refund reversals cannot credit twice.
ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS referral_reward_id BIGINT REFERENCES referral_campaign_rewards(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_referral_reward_accrue
    ON user_affiliate_ledger(referral_reward_id)
    WHERE referral_reward_id IS NOT NULL AND action = 'accrue';
