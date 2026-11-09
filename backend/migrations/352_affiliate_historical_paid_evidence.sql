-- Recover only provable historical paid-invite attribution. This migration
-- updates the honor read model; it never changes balances or financial ledgers.
-- A registration timestamp is not a historical binding timestamp.
CREATE OR REPLACE FUNCTION affiliate_rebate_audit_amount_matches(p_detail TEXT,p_amount NUMERIC)
RETURNS BOOLEAN LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE v_detail JSONB;
BEGIN
    v_detail=p_detail::jsonb;
    RETURN COALESCE(jsonb_typeof(v_detail->'rebateAmount')='number'
      AND (v_detail->>'rebateAmount')::numeric=p_amount,false);
EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range THEN
    RETURN false;
END $$;

CREATE OR REPLACE FUNCTION affiliate_has_first_payment_attribution(p_inviter BIGINT,p_invitee BIGINT,p_order BIGINT)
RETURNS BOOLEAN LANGUAGE sql STABLE AS $$
 SELECT COUNT(*)=1 AND COALESCE(BOOL_AND(l.user_id=p_inviter AND l.source_user_id=p_invitee
   AND EXISTS(SELECT 1 FROM payment_orders po WHERE po.id=p_order AND po.user_id=p_invitee)
   AND EXISTS(SELECT 1 FROM payment_audit_logs a WHERE a.order_id=p_order::text
     AND a.action='AFFILIATE_REBATE_APPLIED'
     AND affiliate_rebate_audit_amount_matches(a.detail,l.amount))),false)
 FROM user_affiliate_ledger l WHERE l.source_order_id=p_order AND l.action='accrue'
   AND l.referral_reward_id IS NULL AND l.amount>0
$$;

CREATE OR REPLACE FUNCTION affiliate_refresh_paid_invite_fact(p_invitee BIGINT) RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE
    v_inviter BIGINT; v_bound TIMESTAMPTZ; v_campaign_bound TIMESTAMPTZ; v_campaign_inviter BIGINT; v_campaign_inviters BIGINT;
    v_evidence TEXT; v_source TEXT; v_conflict BOOLEAN;
    v_first_order BIGINT; v_first_paid TIMESTAMPTZ; v_net NUMERIC(20,8); v_payment_floor TIMESTAMPTZ;
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
    -- The original first-order rebate plus its matching applied audit proves
    -- this payment was attributed to this inviter. It does not prove a precise
    -- binding date: bound_at stays NULL and registration counts stay separate.
    IF v_evidence='pending' AND v_source='historical_unknown'
       AND affiliate_has_first_payment_attribution(v_inviter,p_invitee,v_first_order) THEN
        v_evidence='verified';v_source='first_payment_rebate';
    END IF;
    IF v_evidence='verified' AND v_first_paid<v_bound THEN
        v_evidence='pending';v_source='paid_before_binding';
    END IF;
    IF v_evidence='verified' AND EXISTS(SELECT 1 FROM play_membership_order_contributions c WHERE c.user_id=p_invitee AND c.qualification_state='pending_review' AND c.paid_amount>0 AND (v_first_paid IS NULL OR c.paid_at<v_first_paid)) THEN
        v_evidence='pending';v_source='historical_payment_pending';
    END IF;
    IF v_evidence='verified' AND EXISTS(SELECT 1 FROM affiliate_paid_invite_facts f WHERE f.invitee_id=p_invitee AND f.first_paid_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM play_membership_order_contributions c WHERE c.order_id=f.first_paid_order_id AND c.user_id=p_invitee AND c.qualification_state='verified' AND c.paid_amount>0)) THEN
        v_evidence='pending';v_source='payment_evidence_removed';
    END IF;
    v_payment_floor=COALESCE(v_bound,CASE WHEN v_source='first_payment_rebate' THEN v_first_paid END);
    SELECT COALESCE(SUM(c.net_amount),0) INTO v_net FROM play_membership_order_contributions c
    WHERE c.user_id=p_invitee AND c.qualification_state='verified'
      AND c.order_type IN ('balance','subscription') AND c.paid_amount>0 AND c.status IN('COMPLETED','REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REFUND_FAILED') AND c.paid_at>=v_payment_floor;
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

-- Payment contributions can be written before the rebate audit is completed.
-- Refresh only the affected person after audit/ledger mutations, never on polls.
CREATE OR REPLACE FUNCTION affiliate_refresh_paid_invite_rebate_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE v_user BIGINT;
BEGIN
    IF TG_TABLE_NAME='user_affiliate_ledger' THEN
        IF TG_OP<>'INSERT' AND OLD.source_user_id IS NOT NULL AND OLD.source_order_id IS NOT NULL THEN
            PERFORM affiliate_refresh_paid_invite_fact(OLD.source_user_id);
        END IF;
        IF TG_OP<>'DELETE' AND NEW.source_user_id IS NOT NULL AND NEW.source_order_id IS NOT NULL THEN
            PERFORM affiliate_refresh_paid_invite_fact(NEW.source_user_id);
        END IF;
    ELSE
        IF TG_OP<>'INSERT' AND OLD.action='AFFILIATE_REBATE_APPLIED' THEN
            SELECT user_id INTO v_user FROM play_membership_order_contributions WHERE order_id::text=OLD.order_id;
            IF v_user IS NOT NULL THEN PERFORM affiliate_refresh_paid_invite_fact(v_user); END IF;
        END IF;
        IF TG_OP<>'DELETE' AND NEW.action='AFFILIATE_REBATE_APPLIED' THEN
            SELECT user_id INTO v_user FROM play_membership_order_contributions WHERE order_id::text=NEW.order_id;
            IF v_user IS NOT NULL THEN PERFORM affiliate_refresh_paid_invite_fact(v_user); END IF;
        END IF;
    END IF;
    RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS affiliate_paid_invite_rebate_ledger ON user_affiliate_ledger;
CREATE TRIGGER affiliate_paid_invite_rebate_ledger AFTER INSERT OR UPDATE OR DELETE ON user_affiliate_ledger
 FOR EACH ROW EXECUTE FUNCTION affiliate_refresh_paid_invite_rebate_trigger();
DROP TRIGGER IF EXISTS affiliate_paid_invite_rebate_audit ON payment_audit_logs;
CREATE TRIGGER affiliate_paid_invite_rebate_audit AFTER INSERT OR UPDATE OR DELETE ON payment_audit_logs
 FOR EACH ROW EXECUTE FUNCTION affiliate_refresh_paid_invite_rebate_trigger();

-- Idempotent rebuild of existing facts, retaining locked first-payment months.
SELECT affiliate_refresh_paid_invite_fact(invitee_id) FROM affiliate_paid_invite_facts;
