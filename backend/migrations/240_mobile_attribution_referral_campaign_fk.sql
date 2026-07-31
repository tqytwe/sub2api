-- Keep normal Play campaign attribution separate from referral campaign attribution.
-- Invite tokens resolve to referral_campaigns; ordinary acquisition tokens may still
-- reference play_campaigns through campaign_id.
ALTER TABLE IF EXISTS mobile_installations
    ADD COLUMN IF NOT EXISTS referral_campaign_id BIGINT NULL REFERENCES referral_campaigns(id) ON DELETE SET NULL;

ALTER TABLE IF EXISTS mobile_attribution_events
    ADD COLUMN IF NOT EXISTS referral_campaign_id BIGINT NULL REFERENCES referral_campaigns(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_mobile_installations_referral_campaign_seen
    ON mobile_installations(referral_campaign_id, first_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_attribution_events_referral_campaign_time
    ON mobile_attribution_events(referral_campaign_id, occurred_at DESC);
