-- Additive payment-coupon campaign support. Existing coupons/orders are untouched.
CREATE TABLE IF NOT EXISTS coupon_claim_codes (
 template_id BIGINT PRIMARY KEY REFERENCES coupon_templates(id) ON DELETE CASCADE,
 code VARCHAR(64) NOT NULL UNIQUE DEFAULT ('SAVE-' || UPPER(REPLACE(gen_random_uuid()::text,'-',''))),
 enabled BOOLEAN NOT NULL DEFAULT FALSE,
 starts_at TIMESTAMPTZ, ends_at TIMESTAMPTZ,
 claim_limit BIGINT NOT NULL DEFAULT 0 CHECK(claim_limit >= 0),
 per_user_limit INTEGER NOT NULL DEFAULT 1 CHECK(per_user_limit BETWEEN 1 AND 10),
 claimed_count BIGINT NOT NULL DEFAULT 0 CHECK(claimed_count >= 0),
 audience JSONB NOT NULL DEFAULT '{"mode":"all"}',
 version INTEGER NOT NULL DEFAULT 1,
 CHECK(NOT enabled OR (claim_limit > 0 AND ends_at IS NOT NULL)),
 CHECK(starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at),
 CHECK(claimed_count <= claim_limit)
);
INSERT INTO coupon_claim_codes(template_id) SELECT id FROM coupon_templates ON CONFLICT(template_id) DO NOTHING;
CREATE OR REPLACE FUNCTION coupon_template_create_claim_code() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO coupon_claim_codes(template_id) VALUES(NEW.id) ON CONFLICT(template_id) DO NOTHING;
 RETURN NEW;
END; $$;
DROP TRIGGER IF EXISTS coupon_template_claim_code ON coupon_templates;
CREATE TRIGGER coupon_template_claim_code AFTER INSERT ON coupon_templates FOR EACH ROW EXECUTE FUNCTION coupon_template_create_claim_code();
ALTER TABLE user_coupons DROP CONSTRAINT IF EXISTS user_coupons_source_check;
ALTER TABLE user_coupons ADD CONSTRAINT user_coupons_source_check CHECK(source IN ('blindbox','quiz','checkin','admin_batch','manual','compensation','code_claim'));
CREATE INDEX IF NOT EXISTS idx_coupon_campaign_user ON user_coupons(template_id,user_id,source);
CREATE TABLE IF NOT EXISTS coupon_campaign_previews (
 token VARCHAR(64) PRIMARY KEY, template_id BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE CASCADE,
 actor_id BIGINT NOT NULL REFERENCES users(id), snapshot JSONB NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
 batch_id BIGINT REFERENCES coupon_issue_batches(id), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_coupon_campaign_preview_expiry ON coupon_campaign_previews(expires_at) WHERE batch_id IS NULL;
