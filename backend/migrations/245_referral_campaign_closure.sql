-- Referral campaign closure: public rules, explicit legacy-rebate policy,
-- version history, immutable attribution policy snapshots, and user read state.
-- Existing attributions retain the legacy stacking behaviour for compatibility.

ALTER TABLE referral_campaigns
    ADD COLUMN IF NOT EXISTS public_rules_md TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS invitee_notice_md TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS legacy_rebate_policy VARCHAR(16) NOT NULL DEFAULT 'stack',
    ADD COLUMN IF NOT EXISTS rules_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS rules_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE referral_campaigns
    DROP CONSTRAINT IF EXISTS referral_campaign_legacy_rebate_policy_check;
ALTER TABLE referral_campaigns
    ADD CONSTRAINT referral_campaign_legacy_rebate_policy_check
    CHECK (legacy_rebate_policy IN ('exclude', 'stack'));

CREATE TABLE IF NOT EXISTS referral_campaign_rule_versions (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    rules_version BIGINT NOT NULL,
    change_kind VARCHAR(24) NOT NULL DEFAULT 'created',
    snapshot JSONB NOT NULL,
    changed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, rules_version),
    CONSTRAINT referral_campaign_rule_version_kind_check
      CHECK (change_kind IN ('created', 'content', 'financial', 'legacy_import'))
);

-- Funds-affecting amendments are staged separately. They never overwrite a
-- running campaign until Operations, Finance, Risk, and UX all approve the
-- proposed snapshot. Rejected snapshots remain review evidence.
CREATE TABLE IF NOT EXISTS referral_campaign_financial_versions (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    rules_version BIGINT NOT NULL,
    base_rules_version BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'review',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    UNIQUE (campaign_id, rules_version),
    CONSTRAINT referral_campaign_financial_version_status_check
      CHECK (status IN ('review', 'approved', 'rejected'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_referral_campaign_financial_version_active
    ON referral_campaign_financial_versions(campaign_id)
    WHERE status = 'review';

ALTER TABLE referral_campaign_attributions
    ADD COLUMN IF NOT EXISTS rules_version BIGINT,
    ADD COLUMN IF NOT EXISTS legacy_rebate_policy VARCHAR(16),
    ADD COLUMN IF NOT EXISTS qualification_to_snapshot TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS pay_threshold_snapshot NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS usage_threshold_snapshot NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS risk_hold_hours_snapshot INTEGER;

-- Existing attributions were created before the policy existed. Pin them to
-- stack so a migration never changes a user-visible historical promise.
UPDATE referral_campaign_attributions a
SET rules_version = COALESCE(a.rules_version, 1),
    legacy_rebate_policy = COALESCE(a.legacy_rebate_policy, 'stack'),
    qualification_to_snapshot = COALESCE(a.qualification_to_snapshot, c.qualification_to),
    pay_threshold_snapshot = COALESCE(a.pay_threshold_snapshot, c.pay_threshold),
    usage_threshold_snapshot = COALESCE(a.usage_threshold_snapshot, c.usage_threshold),
    risk_hold_hours_snapshot = COALESCE(a.risk_hold_hours_snapshot, c.risk_hold_hours)
FROM referral_campaigns c
WHERE c.id = a.campaign_id;

ALTER TABLE referral_campaign_attributions
    ALTER COLUMN rules_version SET NOT NULL,
    ALTER COLUMN legacy_rebate_policy SET NOT NULL,
    ALTER COLUMN qualification_to_snapshot SET NOT NULL,
    ALTER COLUMN pay_threshold_snapshot SET NOT NULL,
    ALTER COLUMN usage_threshold_snapshot SET NOT NULL,
    ALTER COLUMN risk_hold_hours_snapshot SET NOT NULL;

ALTER TABLE referral_campaign_attributions
    DROP CONSTRAINT IF EXISTS referral_campaign_attribution_legacy_rebate_policy_check;
ALTER TABLE referral_campaign_attributions
    ADD CONSTRAINT referral_campaign_attribution_legacy_rebate_policy_check
    CHECK (legacy_rebate_policy IN ('exclude', 'stack'));

CREATE TABLE IF NOT EXISTS referral_campaign_user_views (
    campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rules_version BIGINT NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_referral_campaign_attribution_policy
    ON referral_campaign_attributions(invitee_id, legacy_rebate_policy, status);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_rule_versions_campaign
    ON referral_campaign_rule_versions(campaign_id, rules_version DESC);
