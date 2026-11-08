-- The honor leaderboard is an independent read model. It never credits,
-- reverses, freezes or withdraws wallet funds.
ALTER TABLE user_affiliates ADD COLUMN IF NOT EXISTS bound_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS affiliate_paid_invite_facts (
    invitee_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bound_at TIMESTAMPTZ,
    evidence_status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK(evidence_status IN ('pending','verified','conflict')),
    evidence_source VARCHAR(40) NOT NULL DEFAULT 'historical_unknown',
    rule_version VARCHAR(40) NOT NULL DEFAULT 'paid-invite-v1',
    first_paid_order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    first_paid_at TIMESTAMPTZ,
    first_paid_month VARCHAR(7),
    first_paid_locked BOOLEAN NOT NULL DEFAULT FALSE,
    net_paid NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(net_paid>=0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK(inviter_id<>invitee_id),
    CHECK((first_paid_at IS NULL AND first_paid_month IS NULL) OR (first_paid_at IS NOT NULL AND first_paid_month IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_affiliate_paid_invite_month ON affiliate_paid_invite_facts(first_paid_month,inviter_id) WHERE evidence_status='verified' AND net_paid>0;
CREATE INDEX IF NOT EXISTS idx_affiliate_paid_invite_inviter ON affiliate_paid_invite_facts(inviter_id,invitee_id);
CREATE INDEX IF NOT EXISTS idx_affiliate_paid_invite_pending ON affiliate_paid_invite_facts(evidence_status) WHERE evidence_status<>'verified';

CREATE TABLE IF NOT EXISTS affiliate_operations_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_affiliate_operations_audit_user ON affiliate_operations_audit_logs(user_id,created_at DESC,id DESC);

-- Future bindings retain their actual observation time. Existing historical
-- rows are deliberately not assigned their profile creation/update timestamp.
CREATE OR REPLACE FUNCTION affiliate_record_binding_time() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.inviter_id IS NOT NULL AND NEW.bound_at IS NULL THEN
        IF TG_OP='INSERT' THEN NEW.bound_at=clock_timestamp();
        ELSIF OLD.inviter_id IS NULL THEN NEW.bound_at=clock_timestamp();
        END IF;
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS affiliate_binding_time ON user_affiliates;
CREATE TRIGGER affiliate_binding_time BEFORE INSERT OR UPDATE OF inviter_id ON user_affiliates FOR EACH ROW EXECUTE FUNCTION affiliate_record_binding_time();

CREATE OR REPLACE FUNCTION affiliate_refresh_paid_invite_fact(p_invitee BIGINT) RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE
    v_inviter BIGINT; v_bound TIMESTAMPTZ; v_campaign_bound TIMESTAMPTZ; v_campaign_inviter BIGINT; v_campaign_inviters BIGINT;
    v_evidence TEXT; v_source TEXT; v_conflict BOOLEAN;
    v_first_order BIGINT; v_first_paid TIMESTAMPTZ; v_net NUMERIC(20,8);
BEGIN
    -- Different payment orders for one person cannot race and overwrite a
    -- net-paid sum with an earlier snapshot. No global lock or periodic scan.
    PERFORM pg_advisory_xact_lock(hashtextextended('affiliate_paid:'||p_invitee::text,0));
    SELECT inviter_id,bound_at INTO v_inviter,v_bound FROM user_affiliates WHERE user_id=p_invitee;
    SELECT MIN(inviter_id),COUNT(DISTINCT inviter_id) INTO v_campaign_inviter,v_campaign_inviters FROM referral_campaign_attributions WHERE invitee_id=p_invitee;
    v_inviter=COALESCE(v_inviter,v_campaign_inviter);
    IF v_inviter IS NULL OR v_inviter=p_invitee THEN RETURN; END IF;
    SELECT MIN(registered_at) FILTER(WHERE inviter_id=v_inviter),
           COALESCE(BOOL_OR(inviter_id<>v_inviter),false)
    INTO v_campaign_bound,v_conflict FROM referral_campaign_attributions WHERE invitee_id=p_invitee;
    v_conflict=v_conflict OR v_campaign_inviters>1;
    v_source='live_binding'; v_evidence='verified';
    IF v_conflict THEN v_evidence='conflict'; v_source='campaign_conflict';
    ELSIF v_campaign_bound IS NOT NULL THEN
        v_bound=CASE WHEN v_bound IS NULL THEN v_campaign_bound ELSE LEAST(v_bound,v_campaign_bound) END;
        v_source='campaign_attribution';
    ELSIF v_bound IS NULL THEN v_evidence='pending'; v_source='historical_unknown';
    END IF;
    SELECT c.order_id,c.paid_at INTO v_first_order,v_first_paid
    FROM play_membership_order_contributions c
    WHERE c.user_id=p_invitee AND c.qualification_state='verified'
      AND c.order_type IN ('balance','subscription') AND c.paid_amount>0 AND c.status IN('COMPLETED','REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REFUND_FAILED')
    ORDER BY c.paid_at,c.order_id LIMIT 1;
    -- The first real payment is lifetime-first, including fully refunded
    -- orders. Filtering before MIN would falsely turn a second purchase into
    -- a new paying invite after a later attribution.
    IF v_evidence='verified' AND v_first_paid<v_bound THEN
        v_evidence='pending';v_source='paid_before_binding';
    END IF;
    IF v_evidence='verified' AND EXISTS(SELECT 1 FROM play_membership_order_contributions c WHERE c.user_id=p_invitee AND c.qualification_state='pending_review' AND c.paid_amount>0 AND (v_first_paid IS NULL OR c.paid_at<v_first_paid)) THEN
        v_evidence='pending';v_source='historical_payment_pending';
    END IF;
    IF v_evidence='verified' AND EXISTS(SELECT 1 FROM affiliate_paid_invite_facts f WHERE f.invitee_id=p_invitee AND f.first_paid_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM play_membership_order_contributions c WHERE c.order_id=f.first_paid_order_id AND c.user_id=p_invitee AND c.qualification_state='verified' AND c.paid_amount>0)) THEN
        v_evidence='pending';v_source='payment_evidence_removed';
    END IF;
    SELECT COALESCE(SUM(c.net_amount),0) INTO v_net FROM play_membership_order_contributions c
    WHERE c.user_id=p_invitee AND c.qualification_state='verified'
      AND c.order_type IN ('balance','subscription') AND c.paid_amount>0 AND c.status IN('COMPLETED','REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REFUND_FAILED') AND c.paid_at>=v_bound;
    INSERT INTO affiliate_paid_invite_facts(invitee_id,inviter_id,bound_at,evidence_status,evidence_source,first_paid_order_id,first_paid_at,first_paid_month,first_paid_locked,net_paid)
    VALUES(p_invitee,v_inviter,v_bound,v_evidence,v_source,v_first_order,v_first_paid,to_char(v_first_paid AT TIME ZONE 'Asia/Shanghai','YYYY-MM'),v_evidence='verified' AND v_first_paid IS NOT NULL,v_net)
    ON CONFLICT(invitee_id) DO UPDATE SET
      bound_at=COALESCE(affiliate_paid_invite_facts.bound_at,EXCLUDED.bound_at),
      evidence_status=CASE WHEN affiliate_paid_invite_facts.inviter_id<>EXCLUDED.inviter_id THEN 'conflict' ELSE EXCLUDED.evidence_status END,
      evidence_source=CASE WHEN affiliate_paid_invite_facts.inviter_id<>EXCLUDED.inviter_id THEN 'inviter_conflict' ELSE EXCLUDED.evidence_source END,
      first_paid_order_id=CASE WHEN affiliate_paid_invite_facts.first_paid_locked AND NOT COALESCE(EXCLUDED.first_paid_at<affiliate_paid_invite_facts.first_paid_at,false) THEN affiliate_paid_invite_facts.first_paid_order_id ELSE EXCLUDED.first_paid_order_id END,
      first_paid_at=CASE WHEN affiliate_paid_invite_facts.first_paid_locked AND NOT COALESCE(EXCLUDED.first_paid_at<affiliate_paid_invite_facts.first_paid_at,false) THEN affiliate_paid_invite_facts.first_paid_at ELSE EXCLUDED.first_paid_at END,
      first_paid_month=CASE WHEN affiliate_paid_invite_facts.first_paid_locked AND NOT COALESCE(EXCLUDED.first_paid_at<affiliate_paid_invite_facts.first_paid_at,false) THEN affiliate_paid_invite_facts.first_paid_month ELSE EXCLUDED.first_paid_month END,
      first_paid_locked=affiliate_paid_invite_facts.first_paid_locked OR EXCLUDED.first_paid_locked,
      net_paid=EXCLUDED.net_paid,updated_at=NOW();
END $$;

CREATE OR REPLACE FUNCTION affiliate_refresh_paid_invite_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_TABLE_NAME='user_affiliates' THEN
        PERFORM affiliate_refresh_paid_invite_fact(NEW.user_id);
    ELSIF TG_TABLE_NAME='referral_campaign_attributions' THEN
        IF TG_OP='DELETE' THEN PERFORM affiliate_refresh_paid_invite_fact(OLD.invitee_id);
        ELSE PERFORM affiliate_refresh_paid_invite_fact(NEW.invitee_id); END IF;
    ELSE
        IF TG_OP='DELETE' THEN PERFORM affiliate_refresh_paid_invite_fact(OLD.user_id);
        ELSE
            PERFORM affiliate_refresh_paid_invite_fact(NEW.user_id);
            IF TG_OP='UPDATE' AND OLD.user_id<>NEW.user_id THEN PERFORM affiliate_refresh_paid_invite_fact(OLD.user_id); END IF;
        END IF;
    END IF;
    RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS affiliate_paid_invite_binding ON user_affiliates;
CREATE TRIGGER affiliate_paid_invite_binding AFTER INSERT OR UPDATE OF inviter_id,bound_at ON user_affiliates FOR EACH ROW EXECUTE FUNCTION affiliate_refresh_paid_invite_trigger();
DROP TRIGGER IF EXISTS affiliate_paid_invite_order ON play_membership_order_contributions;
CREATE TRIGGER affiliate_paid_invite_order AFTER INSERT OR UPDATE OR DELETE ON play_membership_order_contributions FOR EACH ROW EXECUTE FUNCTION affiliate_refresh_paid_invite_trigger();
DROP TRIGGER IF EXISTS affiliate_paid_invite_attribution ON referral_campaign_attributions;
CREATE TRIGGER affiliate_paid_invite_attribution AFTER INSERT OR UPDATE OR DELETE ON referral_campaign_attributions FOR EACH ROW EXECUTE FUNCTION affiliate_refresh_paid_invite_trigger();

-- One-time evidence backfill only. Replaying does not move an earned month.
SELECT affiliate_refresh_paid_invite_fact(invitee_id) FROM (
    SELECT user_id AS invitee_id FROM user_affiliates WHERE inviter_id IS NOT NULL
    UNION SELECT invitee_id FROM referral_campaign_attributions
) invitations;
