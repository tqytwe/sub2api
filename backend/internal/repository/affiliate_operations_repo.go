package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var _ service.AffiliateOperationsRepository = (*affiliateRepository)(nil)

// A bounded read model; payment history is projected on mutation, never scanned
// by leaderboard polling. Current risk/disabled state is checked at read time.
const affiliateOperationsStateCTE = `WITH states AS (
 SELECT f.*,COALESCE(inviter.email,'') AS inviter_email,COALESCE(inviter.username,'') AS inviter_username,
 COALESCE(invitee.email,'') AS invitee_email,
 CASE WHEN inviter.deleted_at IS NOT NULL OR invitee.deleted_at IS NOT NULL OR inviter.status<>'active' OR invitee.status<>'active' THEN 'disabled'
 WHEN EXISTS(SELECT 1 FROM ip_risk_case_users cu JOIN ip_risk_cases rc ON rc.id=cu.case_id WHERE cu.user_id IN (f.inviter_id,f.invitee_id) AND rc.status IN ('open','observing','processing') AND rc.level IN ('high','severe','critical')) THEN 'risk_excluded'
 WHEN f.evidence_status<>'verified' THEN 'pending_evidence'
 WHEN f.first_paid_at IS NULL THEN 'unpaid'
 WHEN f.net_paid<=0 THEN 'refunded' ELSE 'effective' END AS status
 FROM affiliate_paid_invite_facts f JOIN users inviter ON inviter.id=f.inviter_id JOIN users invitee ON invitee.id=f.invitee_id
)`

type affiliateOperationsSQL struct {
	args    []any
	clauses []string
}

func (b *affiliateOperationsSQL) arg(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}
func (b *affiliateOperationsSQL) equal(column string, v any) {
	b.clauses = append(b.clauses, column+"="+b.arg(v))
}
func (b *affiliateOperationsSQL) where() string {
	if len(b.clauses) == 0 {
		return "TRUE"
	}
	return strings.Join(b.clauses, " AND ")
}
func (b *affiliateOperationsSQL) dates(q service.AffiliateOperationsQuery, column string) {
	if q.StartAt != nil {
		b.clauses = append(b.clauses, column+">="+b.arg(*q.StartAt))
	}
	if q.EndAt != nil {
		b.clauses = append(b.clauses, column+"<"+b.arg(*q.EndAt))
	}
}
func affiliateOperationsFactFilter(q service.AffiliateOperationsQuery) *affiliateOperationsSQL {
	b := &affiliateOperationsSQL{}
	if q.UserID > 0 {
		b.equal("inviter_id", q.UserID)
	}
	if q.InviteeID > 0 {
		b.equal("invitee_id", q.InviteeID)
	}
	if q.Status == "exceptions" {
		b.clauses = append(b.clauses, "status IN ('pending_evidence','refunded','risk_excluded','disabled')")
	} else if q.Status != "" {
		b.equal("status", q.Status)
	}
	if q.Search != "" {
		p := b.arg("%" + q.Search + "%")
		if q.Admin {
			b.clauses = append(b.clauses, "(inviter_email ILIKE "+p+" OR invitee_email ILIKE "+p+")")
		} else {
			b.clauses = append(b.clauses, "invitee_id::text="+b.arg(q.Search))
		}
	}
	b.dates(q, "bound_at")
	return b
}

func (r *affiliateRepository) affiliateOperationsJSON(ctx context.Context, query string, args []any, out any) error {
	var raw []byte
	if err := scanAffiliateRow(ctx, clientFromContext(ctx, r.client), query, args, &raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func affiliateOperationsPagedSQL(b *affiliateOperationsSQL, q service.AffiliateOperationsQuery, projection, order string) string {
	limit := b.arg(q.PageSize)
	offset := b.arg((q.Page - 1) * q.PageSize)
	page := b.arg(q.Page)
	return ` SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(p)) FROM (SELECT ` + projection + ` FROM filtered ORDER BY ` + order + ` LIMIT ` + limit + ` OFFSET ` + offset + `) p),'[]'::jsonb),'total',(SELECT COUNT(*) FROM filtered),'page',` + page + `::integer,'page_size',` + limit + `::integer)`
}

func (r *affiliateRepository) ListAffiliateOperationsInvitees(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsPage[service.AffiliateOperationsInvitee], error) {
	b := affiliateOperationsFactFilter(q)
	projection := `invitee_id AS user_id,inviter_id,invitee_id,inviter_email,invitee_email,invitee_email AS display_name,bound_at AS invited_at,first_paid_at,first_paid_month,status,CASE WHEN status='pending_evidence' THEN CASE WHEN evidence_status='conflict' THEN 'evidence_conflict' ELSE 'evidence_missing' END ELSE status END AS reason,COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=filtered.inviter_id AND l.source_user_id=filtered.invitee_id AND l.action IN ('accrue','revoke') AND l.referral_reward_id IS NULL),0) AS total_rebate`
	if q.Admin {
		projection += `,CASE WHEN status='risk_excluded' THEN (SELECT rc.id FROM ip_risk_case_users cu JOIN ip_risk_cases rc ON rc.id=cu.case_id WHERE cu.user_id IN(filtered.inviter_id,filtered.invitee_id) AND rc.status IN('open','observing','processing') AND rc.level IN('high','severe','critical') ORDER BY CASE rc.level WHEN 'critical' THEN 3 WHEN 'severe' THEN 2 ELSE 1 END DESC,rc.updated_at DESC,rc.id DESC LIMIT 1) END AS risk_case_id`
	}
	query := affiliateOperationsStateCTE + `,filtered AS(SELECT * FROM states WHERE ` + b.where() + `)` + affiliateOperationsPagedSQL(b, q, projection, "bound_at DESC NULLS LAST,invitee_id DESC")
	var out service.AffiliateOperationsPage[service.AffiliateOperationsInvitee]
	if err := r.affiliateOperationsJSON(ctx, query, b.args, &out); err != nil {
		return nil, err
	}
	for i := range out.Items {
		out.Items[i].DisplayName = service.MaskAffiliateOperationsName(out.Items[i].DisplayName)
		if !q.Admin {
			out.Items[i].InviterID = 0
			out.Items[i].InviteeID = 0
			out.Items[i].InviterEmail = ""
			out.Items[i].InviteeEmail = ""
			out.Items[i].RiskCaseID = nil
		}
	}
	return &out, nil
}

func (r *affiliateRepository) ListAffiliateOperationsEarnings(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsPage[service.AffiliateOperationsEarning], error) {
	b := &affiliateOperationsSQL{}
	if q.UserID > 0 {
		b.equal("l.user_id", q.UserID)
	}
	if q.InviteeID > 0 {
		b.equal("l.source_user_id", q.InviteeID)
	}
	if q.SourceOrderID > 0 {
		b.equal("l.source_order_id", q.SourceOrderID)
	}
	if q.Action != "" {
		b.equal("l.action", q.Action)
	}
	b.dates(q, "l.created_at")
	if q.Search != "" && q.Admin {
		p := b.arg("%" + q.Search + "%")
		b.clauses = append(b.clauses, "(u.email ILIKE "+p+" OR invitee.email ILIKE "+p+")")
	}
	cte := `WITH filtered AS(SELECT l.id,l.user_id,COALESCE(u.email,'') AS email,l.source_user_id AS invitee_id,COALESCE(invitee.email,'') AS invitee_email,COALESCE(invitee.email,'') AS source_user_display_name,l.source_order_id,l.action,
 CASE WHEN l.action='transfer' THEN 'balance_transfer' WHEN l.action='withdraw' THEN 'offline_withdrawal' WHEN l.referral_reward_id IS NOT NULL THEN 'campaign_reward' WHEN l.source_order_id IS NOT NULL THEN 'order_rebate' ELSE 'other_bonus' END AS source_type,
 l.amount,COALESCE(reward.currency,'USD') AS currency,
 CASE WHEN l.action='revoke' OR reward.status='revoked' THEN 'revoked' WHEN l.action<>'accrue' THEN 'completed' WHEN l.frozen_until>NOW() THEN 'frozen' ELSE 'available' END AS status,
 l.frozen_until,l.balance_after,l.created_at FROM user_affiliate_ledger l JOIN users u ON u.id=l.user_id LEFT JOIN users invitee ON invitee.id=l.source_user_id LEFT JOIN referral_campaign_rewards reward ON reward.id=l.referral_reward_id WHERE ` + b.where() + `)`
	query := cte + affiliateOperationsPagedSQL(b, q, "*", "created_at DESC,id DESC")
	var out service.AffiliateOperationsPage[service.AffiliateOperationsEarning]
	if err := r.affiliateOperationsJSON(ctx, query, b.args, &out); err != nil {
		return nil, err
	}
	for i := range out.Items {
		out.Items[i].SourceUserDisplayName = service.MaskAffiliateOperationsName(out.Items[i].SourceUserDisplayName)
		if !q.Admin {
			out.Items[i].UserID = 0
			out.Items[i].Email = ""
			out.Items[i].InviteeEmail = ""
			out.Items[i].InviteeID = nil
		}
	}
	return &out, nil
}

func (r *affiliateRepository) GetAffiliateOperationsLeaderboard(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsLeaderboard, error) {
	b := &affiliateOperationsSQL{}
	month := b.arg(q.Month)
	var cte string
	tieOrder := "reached_at ASC NULLS LAST"
	if q.CampaignID == 0 {
		cte = affiliateOperationsStateCTE + `,counts AS(SELECT inviter_id,MAX(inviter_email) AS email,MAX(inviter_username) AS username,COUNT(*) AS effective_paid_count,MAX(first_paid_at) AS reached_at FROM states WHERE evidence_status='verified' AND net_paid>0 AND status='effective' AND first_paid_month=` + month + ` GROUP BY inviter_id)`
	} else {
		tieOrder = "paid DESC"
		campaign := b.arg(q.CampaignID)
		cte = `WITH counts AS(SELECT q.inviter_id,MAX(u.email) AS email,MAX(COALESCE(u.username,'')) AS username,COUNT(DISTINCT q.invitee_id) AS effective_paid_count,SUM(q.net_paid) AS paid,MAX(q.qualified_at) AS reached_at FROM referral_campaign_qualifications q JOIN referral_campaign_attributions a ON a.campaign_id=q.campaign_id AND a.invitee_id=q.invitee_id AND a.inviter_id=q.inviter_id JOIN users u ON u.id=q.inviter_id JOIN users invitee ON invitee.id=q.invitee_id WHERE q.campaign_id=` + campaign + ` AND q.status='qualified' AND q.risk_status='approved' AND a.status='approved' AND u.status='active' AND u.deleted_at IS NULL AND invitee.status='active' AND invitee.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM ip_risk_case_users cu JOIN ip_risk_cases rc ON rc.id=cu.case_id WHERE cu.user_id IN(q.inviter_id,q.invitee_id) AND rc.status IN ('open','observing','processing') AND rc.level IN ('high','severe','critical')) GROUP BY q.inviter_id)`
		// Keep placeholder numbering stable for the common envelope below.
	}
	my := b.arg(q.UserID)
	cte += `,ranked AS(SELECT ROW_NUMBER() OVER(ORDER BY effective_paid_count DESC,` + tieOrder + `,inviter_id ASC) AS rank,inviter_id AS user_id,email,username,email AS display_name,effective_paid_count,inviter_id=` + my + ` AS is_me FROM counts)`
	query := cte + ` SELECT jsonb_build_object('month',` + month + `::text,'timezone','Asia/Shanghai','items',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY rank) FROM (SELECT * FROM ranked WHERE rank<=10)p),'[]'::jsonb),'my_rank',(SELECT to_jsonb(me) FROM ranked me WHERE user_id=` + my + `),'updated_at',NOW())`
	var out service.AffiliateOperationsLeaderboard
	if err := r.affiliateOperationsJSON(ctx, query, b.args, &out); err != nil {
		return nil, err
	}
	mask := func(row *service.AffiliateOperationsRank) {
		row.DisplayName = service.MaskLeaderboardEmail(row.Email)
		if !q.Admin {
			row.UserID = 0
			row.Email = ""
			row.Username = ""
		}
	}
	for i := range out.Items {
		mask(&out.Items[i])
	}
	if out.MyRank != nil {
		mask(out.MyRank)
	}
	return &out, nil
}

func (r *affiliateRepository) GetAffiliateOperationsSummary(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsSummary, error) {
	b := &affiliateOperationsSQL{}
	user := b.arg(q.UserID)
	month := b.arg(q.Month)
	cohort := "TRUE"
	paid := "TRUE"
	if q.Admin {
		cohort = "to_char(bound_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM')=" + month
		paid = "first_paid_month=" + month
	}
	query := affiliateOperationsStateCTE + `,mine AS(SELECT * FROM states WHERE (` + user + `::bigint=0 OR inviter_id=` + user + `)),counts AS(SELECT COUNT(*) FILTER(WHERE ` + cohort + `) AS invited_count,COUNT(*) FILTER(WHERE status='effective' AND ` + paid + `) AS effective_paid_count,COUNT(*) FILTER(WHERE status='effective' AND ` + cohort + `) AS cohort_effective_paid_count,COUNT(*) FILTER(WHERE status='pending_evidence') AS pending_count,COUNT(*) FILTER(WHERE status IN('pending_evidence','refunded','risk_excluded','disabled')) AS exceptions_count FROM mine),wallet AS(SELECT COALESCE(SUM(ua.aff_quota+COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=ua.user_id AND l.action='accrue' AND l.frozen_until<=NOW()),0)),0) AS available_quota,COALESCE(SUM(GREATEST(ua.aff_frozen_quota-COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=ua.user_id AND l.action='accrue' AND l.frozen_until<=NOW()),0),0)),0) AS frozen_quota FROM user_affiliates ua WHERE (` + user + `::bigint=0 OR ua.user_id=` + user + `)),money AS(SELECT COALESCE(SUM(l.amount) FILTER(WHERE l.action='accrue' AND l.referral_reward_id IS NULL),0) AS gross_earned,-COALESCE(SUM(l.amount) FILTER(WHERE l.action='revoke' AND l.referral_reward_id IS NULL),0) AS revoked_amount,COALESCE(SUM(l.amount) FILTER(WHERE l.action IN('accrue','revoke') AND l.referral_reward_id IS NULL),0) AS net_earned,COALESCE(SUM(l.amount) FILTER(WHERE l.action='transfer'),0) AS transferred_amount,COALESCE(SUM(l.amount) FILTER(WHERE l.action='withdraw'),0) AS withdrawn_amount FROM user_affiliate_ledger l WHERE (` + user + `::bigint=0 OR l.user_id=` + user + `)) SELECT to_jsonb(c)||to_jsonb(w)||to_jsonb(m)||jsonb_build_object('month',` + month + `::text,'currency','USD','monetary_scope','all_time','conversion_rate',CASE WHEN c.invited_count>0 THEN c.cohort_effective_paid_count::numeric/c.invited_count ELSE 0 END,'trend','[]'::jsonb,'updated_at',NOW()) FROM counts c CROSS JOIN wallet w CROSS JOIN money m`
	var out service.AffiliateOperationsSummary
	if err := r.affiliateOperationsJSON(ctx, query, b.args, &out); err != nil {
		return nil, err
	}
	if err := r.affiliateOperationsReconciliation(ctx, q, &out.Reconciliation); err != nil {
		return nil, err
	}
	if q.Admin {
		// Cohort conversion uses actual registration dates, independently of
		// whether the historical binding timestamp can be reconstructed.
		out.CohortEffectivePaidCount = out.Reconciliation.RegisteredEffectivePaidCount
		out.ConversionRate = 0
		if out.Reconciliation.RegisteredMonthCount > 0 {
			out.ConversionRate = float64(out.CohortEffectivePaidCount) / float64(out.Reconciliation.RegisteredMonthCount)
		}
	}
	if err := r.affiliateOperationsMoney(ctx, q, &out); err != nil {
		return nil, err
	}
	if q.Admin {
		trendQuery := affiliateOperationsStateCTE + `,days AS(SELECT generate_series(to_date($2,'YYYY-MM'),(to_date($2,'YYYY-MM')+INTERVAL '1 month - 1 day')::date,INTERVAL '1 day')::date AS day),mine AS(SELECT * FROM states WHERE ($1::bigint=0 OR inviter_id=$1)) SELECT COALESCE(jsonb_agg(jsonb_build_object('date',to_char(d.day,'YYYY-MM-DD'),'invited_count',(SELECT COUNT(*) FROM mine WHERE (bound_at AT TIME ZONE 'Asia/Shanghai')::date=d.day),'effective_paid_count',(SELECT COUNT(*) FROM mine WHERE status='effective' AND (first_paid_at AT TIME ZONE 'Asia/Shanghai')::date=d.day)) ORDER BY d.day),'[]'::jsonb) FROM days d`
		if err := r.affiliateOperationsJSON(ctx, trendQuery, []any{q.UserID, q.Month}, &out.Trend); err != nil {
			return nil, err
		}
	}
	return &out, nil
}

func (r *affiliateRepository) affiliateOperationsReconciliation(ctx context.Context, q service.AffiliateOperationsQuery, out *service.AffiliateOperationsReconciliation) error {
	// Start from canonical relationships so a missing read-model row is visible.
	// UNION deduplicates a person invited through several campaigns. The same
	// state precedence as the leaderboard keeps the qualification buckets disjoint.
	query := affiliateOperationsStateCTE + `,relationships AS(
 SELECT user_id AS invitee_id FROM user_affiliates WHERE inviter_id IS NOT NULL AND ($1::bigint=0 OR inviter_id=$1)
 UNION SELECT invitee_id FROM referral_campaign_attributions WHERE ($1::bigint=0 OR inviter_id=$1)
),reconciled AS(
 SELECT u.created_at,s.bound_at,s.first_paid_at,s.invitee_id IS NULL AS missing_fact,COALESCE(s.status,'pending_evidence') AS status
 FROM relationships r JOIN users u ON u.id=r.invitee_id LEFT JOIN states s ON s.invitee_id=r.invitee_id
)
 SELECT jsonb_build_object('relationship_count',COUNT(*),
 'registered_month_count',COUNT(*) FILTER(WHERE to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM')=$2),
 'registered_effective_paid_count',COUNT(*) FILTER(WHERE to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM')=$2 AND status='effective'),
 'known_binding_count',COUNT(bound_at),'unknown_binding_count',COUNT(*) FILTER(WHERE bound_at IS NULL),
 'effective_paid_count',COUNT(*) FILTER(WHERE status='effective'),'unpaid_count',COUNT(*) FILTER(WHERE status='unpaid'),
 'refunded_count',COUNT(*) FILTER(WHERE status='refunded'),'risk_excluded_count',COUNT(*) FILTER(WHERE status='risk_excluded'),
 'disabled_count',COUNT(*) FILTER(WHERE status='disabled'),'pending_count',COUNT(*) FILTER(WHERE status='pending_evidence'),
 'missing_fact_count',COUNT(*) FILTER(WHERE missing_fact),'pending_payment_count',COUNT(*) FILTER(WHERE status='pending_evidence' AND first_paid_at IS NOT NULL)) FROM reconciled`
	return r.affiliateOperationsJSON(ctx, query, []any{q.UserID, q.Month}, out)
}

func (r *affiliateRepository) affiliateOperationsMoney(ctx context.Context, q service.AffiliateOperationsQuery, out *service.AffiliateOperationsSummary) error {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	start, _ := time.ParseInLocation("2006-01", q.Month, loc)
	end := start.AddDate(0, 1, 0)
	if q.StartAt != nil {
		start = *q.StartAt
	}
	if q.EndAt != nil {
		end = *q.EndAt
	}
	query := `WITH entries AS(SELECT l.action,l.amount,l.created_at,COALESCE(reward.currency,'USD') AS currency,CASE WHEN l.referral_reward_id IS NOT NULL THEN 'campaign' WHEN l.source_order_id IS NOT NULL THEN 'regular' ELSE 'other' END AS source_type FROM user_affiliate_ledger l LEFT JOIN referral_campaign_rewards reward ON reward.id=l.referral_reward_id WHERE ($1::bigint=0 OR l.user_id=$1) AND l.action IN('accrue','revoke')),
 amounts AS(SELECT source_type,currency,COALESCE(SUM(amount) FILTER(WHERE action='accrue'),0) AS gross_earned,-COALESCE(SUM(amount) FILTER(WHERE action='revoke'),0) AS revoked_amount,COALESCE(SUM(amount),0) AS net_earned FROM entries GROUP BY source_type,currency),
 period_amounts AS(SELECT source_type,currency,COALESCE(SUM(amount) FILTER(WHERE action='accrue'),0) AS gross_earned,-COALESCE(SUM(amount) FILTER(WHERE action='revoke'),0) AS revoked_amount,COALESCE(SUM(amount),0) AS net_earned FROM entries WHERE created_at>=$2 AND created_at<$3 GROUP BY source_type,currency)
 SELECT jsonb_build_object('amounts',COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY currency,source_type) FROM amounts a),'[]'::jsonb),'period_amounts',COALESCE((SELECT jsonb_agg(to_jsonb(a) ORDER BY currency,source_type) FROM period_amounts a),'[]'::jsonb),
 'currency',CASE WHEN COUNT(DISTINCT currency)>1 THEN 'mixed' ELSE COALESCE(MIN(currency),'USD') END,
 'gross_earned',CASE WHEN COUNT(DISTINCT currency)<=1 THEN COALESCE(SUM(amount) FILTER(WHERE action='accrue'),0) END,
 'revoked_amount',CASE WHEN COUNT(DISTINCT currency)<=1 THEN -COALESCE(SUM(amount) FILTER(WHERE action='revoke'),0) END,
 'net_earned',CASE WHEN COUNT(DISTINCT currency)<=1 THEN COALESCE(SUM(amount),0) END,
 'month_net_earned',(SELECT CASE WHEN COUNT(DISTINCT currency)<=1 THEN COALESCE(SUM(net_earned),0) END FROM period_amounts)) FROM entries`
	var money struct {
		GrossEarned    *float64                           `json:"gross_earned"`
		RevokedAmount  *float64                           `json:"revoked_amount"`
		NetEarned      *float64                           `json:"net_earned"`
		MonthNetEarned *float64                           `json:"month_net_earned"`
		Currency       string                             `json:"currency"`
		Amounts        []service.AffiliateOperationsMoney `json:"amounts"`
		PeriodAmounts  []service.AffiliateOperationsMoney `json:"period_amounts"`
	}
	if err := r.affiliateOperationsJSON(ctx, query, []any{q.UserID, start, end}, &money); err != nil {
		return err
	}
	out.GrossEarned = money.GrossEarned
	out.RevokedAmount = money.RevokedAmount
	out.NetEarned = money.NetEarned
	out.MonthNetEarned = money.MonthNetEarned
	out.Currency = money.Currency
	out.Amounts = money.Amounts
	out.PeriodAmounts = money.PeriodAmounts
	return nil
}

func (r *affiliateRepository) ListAffiliateOperationsPromoters(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsPage[service.AffiliateOperationsPromoter], error) {
	b := &affiliateOperationsSQL{}
	if q.UserID > 0 {
		b.equal("u.id", q.UserID)
	} else {
		b.clauses = append(b.clauses, "(EXISTS(SELECT 1 FROM affiliate_paid_invite_facts f WHERE f.inviter_id=u.id) OR EXISTS(SELECT 1 FROM user_affiliate_ledger l WHERE l.user_id=u.id))")
	}
	if q.Search != "" {
		p := b.arg("%" + q.Search + "%")
		b.clauses = append(b.clauses, "(u.email ILIKE "+p+" OR u.username ILIKE "+p+" OR ua.aff_code ILIKE "+p+")")
	}
	cte := affiliateOperationsStateCTE + `,filtered AS(SELECT u.id AS user_id,COALESCE(u.email,'') AS email,COALESCE(u.username,'') AS username,COALESCE(ua.aff_code,'') AS aff_code,(SELECT COUNT(*) FROM states s WHERE s.inviter_id=u.id) AS invited_count,(SELECT COUNT(*) FROM states s WHERE s.inviter_id=u.id AND s.status='effective') AS effective_paid_count,COALESCE(ua.aff_quota,0)+COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=u.id AND l.action='accrue' AND l.frozen_until<=NOW()),0) AS available_quota,GREATEST(COALESCE(ua.aff_frozen_quota,0)-COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=u.id AND l.action='accrue' AND l.frozen_until<=NOW()),0),0) AS frozen_quota,COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=u.id AND l.action='accrue' AND l.referral_reward_id IS NULL),0) AS gross_earned,COALESCE((SELECT SUM(l.amount) FROM user_affiliate_ledger l WHERE l.user_id=u.id AND l.action IN('accrue','revoke') AND l.referral_reward_id IS NULL),0) AS net_earned FROM users u LEFT JOIN user_affiliates ua ON ua.user_id=u.id WHERE ` + b.where() + `)`
	query := cte + affiliateOperationsPagedSQL(b, q, "*", "effective_paid_count DESC,user_id ASC")
	var out service.AffiliateOperationsPage[service.AffiliateOperationsPromoter]
	if err := r.affiliateOperationsJSON(ctx, query, b.args, &out); err != nil {
		return nil, err
	}
	if err := r.affiliateOperationsPromoterMoney(ctx, out.Items); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *affiliateRepository) affiliateOperationsPromoterMoney(ctx context.Context, items []service.AffiliateOperationsPromoter) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].UserID
	}
	query := `WITH entries AS(SELECT l.user_id,l.action,l.amount,COALESCE(reward.currency,'USD') AS currency,CASE WHEN l.referral_reward_id IS NOT NULL THEN 'campaign' WHEN l.source_order_id IS NOT NULL THEN 'regular' ELSE 'other' END AS source_type FROM user_affiliate_ledger l LEFT JOIN referral_campaign_rewards reward ON reward.id=l.referral_reward_id WHERE l.user_id=ANY($1::bigint[]) AND l.action IN('accrue','revoke')),
 amounts AS(SELECT user_id,source_type,currency,COALESCE(SUM(amount) FILTER(WHERE action='accrue'),0) AS gross_earned,-COALESCE(SUM(amount) FILTER(WHERE action='revoke'),0) AS revoked_amount,COALESCE(SUM(amount),0) AS net_earned FROM entries GROUP BY user_id,source_type,currency),
 summaries AS(SELECT e.user_id,CASE WHEN COUNT(DISTINCT e.currency)>1 THEN 'mixed' ELSE COALESCE(MIN(e.currency),'USD') END AS currency,CASE WHEN COUNT(DISTINCT e.currency)<=1 THEN COALESCE(SUM(e.amount) FILTER(WHERE e.action='accrue'),0) END AS gross_earned,CASE WHEN COUNT(DISTINCT e.currency)<=1 THEN COALESCE(SUM(e.amount),0) END AS net_earned,(SELECT jsonb_agg(to_jsonb(a)-'user_id' ORDER BY a.currency,a.source_type) FROM amounts a WHERE a.user_id=e.user_id) AS amounts FROM entries e GROUP BY e.user_id)
 SELECT COALESCE(jsonb_agg(to_jsonb(s)),'[]'::jsonb) FROM summaries s`
	var money []service.AffiliateOperationsPromoter
	if err := r.affiliateOperationsJSON(ctx, query, []any{pq.Array(ids)}, &money); err != nil {
		return err
	}
	byID := make(map[int64]service.AffiliateOperationsPromoter, len(money))
	for _, m := range money {
		byID[m.UserID] = m
	}
	for i := range items {
		m, ok := byID[items[i].UserID]
		if !ok {
			zero := 0.0
			items[i].GrossEarned = &zero
			items[i].NetEarned = &zero
			items[i].Currency = "USD"
			items[i].Amounts = []service.AffiliateOperationsMoney{}
			continue
		}
		items[i].GrossEarned = m.GrossEarned
		items[i].NetEarned = m.NetEarned
		items[i].Currency = m.Currency
		items[i].Amounts = m.Amounts
	}
	return nil
}

func (r *affiliateRepository) ListAffiliateOperationsAudits(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsPage[service.AffiliateOperationsAudit], error) {
	b := &affiliateOperationsSQL{}
	relatedUser := b.arg(q.UserID)
	if q.UserID > 0 {
		b.equal("user_id", q.UserID)
	}
	b.dates(q, "created_at")
	cte := `WITH audits AS(SELECT id,user_id,actor_id,action,detail,created_at FROM affiliate_operations_audit_logs UNION ALL SELECT -c.id AS id,NULLIF(` + relatedUser + `::bigint,0) AS user_id,c.actor_id,c.action,c.detail||jsonb_build_object('campaign_id',c.campaign_id,'scope','campaign_relation') AS detail,c.created_at FROM referral_campaign_audit_logs c WHERE (` + relatedUser + `::bigint=0 OR EXISTS(SELECT 1 FROM referral_campaign_enrollments e WHERE e.campaign_id=c.campaign_id AND e.user_id=` + relatedUser + `) OR EXISTS(SELECT 1 FROM referral_campaign_attributions a WHERE a.campaign_id=c.campaign_id AND (a.inviter_id=` + relatedUser + ` OR a.invitee_id=` + relatedUser + `)) OR EXISTS(SELECT 1 FROM referral_campaign_rewards reward WHERE reward.campaign_id=c.campaign_id AND reward.user_id=` + relatedUser + `))),filtered AS(SELECT * FROM audits WHERE ` + b.where() + `)`
	query := cte + affiliateOperationsPagedSQL(b, q, "*", "created_at DESC,id DESC")
	var out service.AffiliateOperationsPage[service.AffiliateOperationsAudit]
	if err := r.affiliateOperationsJSON(ctx, query, b.args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *affiliateRepository) ListAffiliateOperationsCampaigns(ctx context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsPage[service.ReferralCampaignProgress], error) {
	b := &affiliateOperationsSQL{}
	b.equal("e.user_id", q.UserID)
	if q.Scope == "joined" {
		b.clauses = append(b.clauses, "c.status NOT IN('closed','cancelled')")
	}
	if q.Scope == "ended" {
		b.clauses = append(b.clauses, "c.status IN('closed','cancelled')")
	}
	client := clientFromContext(ctx, r.client)
	from := ` FROM referral_campaign_enrollments e JOIN referral_campaigns c ON c.id=e.campaign_id WHERE ` + b.where()
	total, err := scanInt64(ctx, client, `SELECT COUNT(*)`+from, b.args...)
	if err != nil {
		return nil, err
	}
	limit := b.arg(q.PageSize)
	offset := b.arg((q.Page - 1) * q.PageSize)
	rows, err := client.QueryContext(ctx, `SELECT c.id`+from+` ORDER BY c.starts_at DESC,c.id DESC LIMIT `+limit+` OFFSET `+offset, b.args...)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	out := &service.AffiliateOperationsPage[service.ReferralCampaignProgress]{Items: []service.ReferralCampaignProgress{}, Total: total, Page: q.Page, PageSize: q.PageSize}
	for _, id := range ids {
		progress, err := r.GetReferralCampaignProgress(ctx, id, q.UserID)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, *progress)
	}
	return out, nil
}
