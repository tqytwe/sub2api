-- New-user growth campaigns share the referral campaign reward ledger. New
-- referral campaigns must opt in to normal 10% referral stacking explicitly;
-- existing attribution snapshots remain untouched by this migration.
ALTER TABLE referral_campaigns
    ALTER COLUMN legacy_rebate_policy SET DEFAULT 'exclude';

-- Each tier reward keeps the exact campaign rule snapshot that produced it.
-- This is deliberately separate from the mutable play_campaigns.rules_json.
CREATE TABLE IF NOT EXISTS play_campaign_reward_snapshots (
    reward_id BIGINT PRIMARY KEY REFERENCES referral_campaign_rewards(id) ON DELETE RESTRICT,
    play_campaign_id BIGINT NOT NULL REFERENCES play_campaigns(id) ON DELETE RESTRICT,
    referral_campaign_id BIGINT NOT NULL REFERENCES referral_campaigns(id) ON DELETE RESTRICT,
    referral_rules_version BIGINT NOT NULL,
    qualification_metric VARCHAR(32) NOT NULL,
    qualified_amount NUMERIC(20,8) NOT NULL,
    rules_snapshot JSONB NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT play_campaign_reward_snapshot_metric_check
        CHECK (qualification_metric IN ('net_recharge', 'actual_consumption')),
    CONSTRAINT play_campaign_reward_snapshot_amount_check
        CHECK (qualified_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_play_campaign_reward_snapshots_campaign_user
    ON play_campaign_reward_snapshots(play_campaign_id, referral_campaign_id);
CREATE INDEX IF NOT EXISTS idx_referral_campaign_rewards_new_user_growth
    ON referral_campaign_rewards(campaign_id, user_id, tier_no)
    WHERE reward_type = 'new_user_tier';
