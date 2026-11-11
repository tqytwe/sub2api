package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

func couponAudienceQuery(a service.CouponAudience, onlyUser int64) (string, []any) {
	args := []any{}
	clauses := []string{"u.deleted_at IS NULL", "u.status = 'active'", "u.role = 'user'"}
	bind := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if onlyUser > 0 {
		clauses = append(clauses, "u.id = "+bind(onlyUser))
	}
	if a.Mode == "users" {
		clauses = append(clauses, "u.id = ANY("+bind(pq.Array(a.UserIDs))+")")
	}
	if a.Search != "" {
		p := bind("%" + strings.TrimSpace(a.Search) + "%")
		exact := bind(strings.TrimSpace(a.Search))
		clauses = append(clauses, "(u.email ILIKE "+p+" OR u.username ILIKE "+p+" OR u.id::text = "+exact+")")
	}
	if a.GroupName != "" {
		clauses = append(clauses, "EXISTS(SELECT 1 FROM user_allowed_groups ug JOIN groups g ON g.id=ug.group_id WHERE ug.user_id=u.id AND g.deleted_at IS NULL AND g.name ILIKE "+bind("%"+a.GroupName+"%")+")")
	}
	if a.APIKeyGroupID > 0 {
		p := bind(a.APIKeyGroupID)
		clauses = append(clauses, "EXISTS(SELECT 1 FROM api_keys k WHERE k.user_id=u.id AND k.deleted_at IS NULL AND (k.group_id="+p+" OR EXISTS(SELECT 1 FROM api_key_groups kg WHERE kg.api_key_id=k.id AND kg.group_id="+p+")))")
	}
	if a.VIPTier != nil {
		p := bind(*a.VIPTier)
		paid := "COALESCE((SELECT SUM(c.net_amount) FROM play_membership_verified_contributions c WHERE c.user_id=u.id AND c.qualification_state='verified'),0)"
		clauses = append(clauses, paid+" >= COALESCE((SELECT (tier->>'min_recharge')::numeric FROM settings cfg, jsonb_array_elements(cfg.value::jsonb) tier WHERE cfg.key='play_vip_tiers' AND (tier->>'tier')::int="+p+"),0) AND "+paid+" < COALESCE((SELECT MIN((tier->>'min_recharge')::numeric) FROM settings cfg, jsonb_array_elements(cfg.value::jsonb) tier WHERE cfg.key='play_vip_tiers' AND (tier->>'tier')::int>"+p+"),1e100)")
	}
	keys := make([]int64, 0, len(a.Attributes))
	for k := range a.Attributes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		clauses = append(clauses, "EXISTS(SELECT 1 FROM user_attribute_values av WHERE av.user_id=u.id AND av.attribute_id="+bind(k)+" AND av.value ILIKE "+bind("%"+a.Attributes[k]+"%")+")")
	}
	return "SELECT u.id FROM users u WHERE " + strings.Join(clauses, " AND ") + " ORDER BY u.id LIMIT 10001", args
}
func couponAudienceIDs(ctx context.Context, exec sqlQueryExecutor, a service.CouponAudience, userID int64, lock bool) ([]int64, error) {
	q, args := couponAudienceQuery(a, userID)
	if lock {
		q += " FOR SHARE OF u"
	}
	rows, err := exec.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 10000 {
		return nil, infraerrors.BadRequest("COUPON_AUDIENCE_TOO_LARGE", "select at most 10000 users")
	}
	return ids, nil
}

const couponPolicyColumns = "template_id,code,enabled,starts_at,ends_at,claim_limit,per_user_limit,claimed_count,audience,version"

func couponPolicy(ctx context.Context, exec sqlQueryExecutor, id int64, lock bool) (*service.CouponClaimPolicy, error) {
	q := "SELECT " + couponPolicyColumns + " FROM coupon_claim_codes WHERE template_id=$1"
	if lock {
		q += " FOR UPDATE"
	}
	p := &service.CouponClaimPolicy{}
	var raw []byte
	err := scanSingleRow(ctx, exec, q, []any{id}, &p.TemplateID, &p.Code, &p.Enabled, &p.StartsAt, &p.EndsAt, &p.ClaimLimit, &p.PerUserLimit, &p.ClaimedCount, &raw, &p.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "coupon template not found")
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &p.Audience); err != nil {
		return nil, err
	}
	return p, nil
}
func (r *couponRepository) GetCouponClaimPolicy(ctx context.Context, id int64) (*service.CouponClaimPolicy, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	p, err := couponPolicy(ctx, exec, id, false)
	if err != nil {
		return nil, err
	}
	err = scanSingleRow(ctx, exec, `SELECT COUNT(*),COUNT(*) FILTER(WHERE status='used'),COUNT(*) FILTER(WHERE source IN ('admin_batch','manual','compensation')) FROM user_coupons WHERE template_id=$1`, []any{id}, &p.IssuedCount, &p.UsedCount, &p.ManualCount)
	return p, err
}
func (r *couponRepository) SaveCouponClaimPolicy(ctx context.Context, id int64, input service.CouponClaimPolicy, actorID int64) (*service.CouponClaimPolicy, error) {
	if err := service.ValidateCouponClaimPolicy(input); err != nil {
		return nil, infraerrors.BadRequest("INVALID_COUPON_CLAIM_POLICY", err.Error())
	}
	if input.Audience.Mode == "" {
		input.Audience.Mode = "all"
	}
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		t, err := getCouponTemplate(ctx, exec, id, true)
		if err != nil {
			return err
		}
		if t == nil {
			return infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "template not found")
		}
		p, err := couponPolicy(ctx, exec, id, true)
		if err != nil {
			return err
		}
		if input.Version != p.Version {
			return infraerrors.Conflict("COUPON_POLICY_STALE", "settings changed; refresh before saving")
		}
		if input.Enabled && (t.Status != service.CouponTemplateStatusActive || !input.EndsAt.After(time.Now())) {
			return infraerrors.Conflict("COUPON_CLAIM_NOT_OPEN", "use an active template and future claim end time")
		}
		if input.ClaimLimit < p.ClaimedCount {
			return infraerrors.Conflict("COUPON_CLAIM_LIMIT_BELOW_ISSUED", "limit cannot be lower than claims already issued")
		}
		raw, err := json.Marshal(input.Audience)
		if err != nil {
			return err
		}
		_, err = exec.ExecContext(ctx, `UPDATE coupon_claim_codes SET enabled=$2,starts_at=$3,ends_at=$4,claim_limit=$5,per_user_limit=$6,audience=$7,version=version+1 WHERE template_id=$1`, id, input.Enabled, input.StartsAt, input.EndsAt, input.ClaimLimit, input.PerUserLimit, raw)
		if err != nil {
			return err
		}
		if err = insertCouponEvent(ctx, exec, couponEventInput{TemplateID: &id, EventType: "claim_policy_updated", ActorType: couponActorType(actorID), ActorUserID: couponActorID(actorID), Metadata: map[string]any{"enabled": input.Enabled, "version": p.Version + 1}}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetCouponClaimPolicy(ctx, id)
}
func couponCampaignEligibility(ctx context.Context, exec sqlQueryExecutor, templateID int64, ids []int64, limit int, at time.Time) ([]int64, []service.CouponRecipient, error) {
	counts := map[int64]int{}
	live := map[int64]bool{}
	rows, err := exec.QueryContext(ctx, `SELECT user_id,COUNT(*) FILTER(WHERE source IN ('admin_batch','manual','compensation','code_claim')),BOOL_OR(status IN ('available','locked') AND expires_at>$3) FROM user_coupons WHERE template_id=$1 AND user_id=ANY($2) GROUP BY user_id`, templateID, pq.Array(ids), at)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var n int
		var active bool
		if err = rows.Scan(&id, &n, &active); err != nil {
			return nil, nil, err
		}
		counts[id] = n
		live[id] = active
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	eligible := []int64{}
	skipped := []service.CouponRecipient{}
	for _, id := range ids {
		reason := ""
		if counts[id] >= limit {
			reason = "per_user_limit"
		} else if live[id] {
			reason = "already_holds_coupon"
		}
		if reason != "" {
			skipped = append(skipped, service.CouponRecipient{UserID: id, Reason: reason})
		} else {
			eligible = append(eligible, id)
		}
	}
	return eligible, skipped, nil
}
func (r *couponRepository) ClaimCouponCode(ctx context.Context, code string, userID int64, at time.Time) (*service.UserCoupon, error) {
	var result *service.UserCoupon
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		var id int64
		if err := scanSingleRow(ctx, exec, "SELECT template_id FROM coupon_claim_codes WHERE code=$1", []any{code}, &id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return infraerrors.NotFound("COUPON_CODE_NOT_FOUND", "coupon code not found")
			}
			return err
		}
		t, err := getCouponTemplate(ctx, exec, id, true)
		if err != nil {
			return err
		}
		p, err := couponPolicy(ctx, exec, id, true)
		if err != nil {
			return err
		}
		ids, err := couponAudienceIDs(ctx, exec, service.CouponAudience{Mode: "all"}, userID, true)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return infraerrors.Conflict("COUPON_AUDIENCE_MISMATCH", "account cannot claim this coupon")
		}
		eligible, _, err := couponCampaignEligibility(ctx, exec, id, ids, p.PerUserLimit, at)
		if err != nil {
			return err
		}
		if len(eligible) == 0 {
			var cid int64
			err = scanSingleRow(ctx, exec, `SELECT id FROM user_coupons WHERE template_id=$1 AND user_id=$2 ORDER BY (status IN ('available','locked') AND expires_at>$3) DESC,issued_at DESC,id DESC LIMIT 1`, []any{id, userID, at}, &cid)
			if err != nil {
				return err
			}
			result, err = getUserCoupon(ctx, exec, cid, false)
			return err
		}
		if !p.Enabled || (p.StartsAt != nil && p.StartsAt.After(at)) || p.EndsAt == nil || !p.EndsAt.After(at) {
			return infraerrors.Conflict("COUPON_CLAIM_NOT_OPEN", "coupon code is not open for claims")
		}
		ids, err = couponAudienceIDs(ctx, exec, p.Audience, userID, false)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return infraerrors.Conflict("COUPON_AUDIENCE_MISMATCH", "account does not meet claim conditions")
		}
		if p.ClaimedCount >= p.ClaimLimit {
			return infraerrors.Conflict("COUPON_CODE_EXHAUSTED", "coupon code claim limit reached")
		}
		var n int64
		if err = scanSingleRow(ctx, exec, `SELECT COUNT(*) FROM user_coupons WHERE template_id=$1 AND user_id=$2 AND source='code_claim'`, []any{id, userID}, &n); err != nil {
			return err
		}
		result, err = issueCouponWithTemplate(ctx, exec, t, service.CouponIssueInput{TemplateID: id, UserID: userID, Source: service.CouponIssueSource("code_claim"), SourceRef: fmt.Sprintf("claim:%d", id), IdempotencyKey: fmt.Sprintf("coupon-claim:%d:%d:%d", id, userID, n+1), ActorID: userID}, at)
		if err != nil {
			return err
		}
		_, err = exec.ExecContext(ctx, `UPDATE coupon_claim_codes SET claimed_count=claimed_count+1 WHERE template_id=$1`, id)
		return err
	})
	return result, err
}
func (r *couponRepository) PreviewCouponCampaign(ctx context.Context, id int64, a service.CouponAudience, actorID int64, at time.Time) (*service.CouponCampaignPreview, error) {
	if err := service.ValidateCouponAudience(a); err != nil {
		return nil, infraerrors.BadRequest("INVALID_COUPON_AUDIENCE", err.Error())
	}
	var result *service.CouponCampaignPreview
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		t, err := getCouponTemplate(ctx, exec, id, true)
		if err != nil {
			return err
		}
		if t == nil || t.Status != service.CouponTemplateStatusActive {
			return infraerrors.Conflict("COUPON_TEMPLATE_INACTIVE", "select an active template")
		}
		if _, _, err = service.CouponExpiryForIssue(*t, at); err != nil {
			return infraerrors.Conflict("COUPON_TEMPLATE_NOT_ISSUABLE", err.Error())
		}
		p, err := couponPolicy(ctx, exec, id, true)
		if err != nil {
			return err
		}
		ids, err := couponAudienceIDs(ctx, exec, a, 0, false)
		if err != nil {
			return err
		}
		eligible, skipped, err := couponCampaignEligibility(ctx, exec, id, ids, p.PerUserLimit, at)
		if err != nil {
			return err
		}
		requested := len(ids)
		if a.Mode == "users" {
			seen := map[int64]bool{}
			found := map[int64]bool{}
			for _, v := range ids {
				found[v] = true
			}
			for _, v := range a.UserIDs {
				if !found[v] && !seen[v] {
					skipped = append(skipped, service.CouponRecipient{UserID: v, Reason: "account_ineligible"})
				}
				seen[v] = true
			}
			requested = len(seen)
		}
		token := make([]byte, 24)
		if _, err = rand.Read(token); err != nil {
			return err
		}
		result = &service.CouponCampaignPreview{Token: hex.EncodeToString(token), TemplateID: id, TemplateVersion: t.Version, PolicyVersion: p.Version, ActorID: actorID, Audience: a, Recipients: eligible, Skipped: skipped, RequestedCount: requested, ExpiresAt: at.Add(15 * time.Minute)}
		if t.TotalIssueLimit != nil {
			left := *t.TotalIssueLimit - t.IssuedCount
			result.RemainingStock = &left
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err = exec.ExecContext(ctx, `DELETE FROM coupon_campaign_previews WHERE token IN (SELECT token FROM coupon_campaign_previews WHERE expires_at<$1 AND batch_id IS NULL FOR UPDATE SKIP LOCKED)`, at); err != nil {
			return err
		}
		_, err = exec.ExecContext(ctx, `INSERT INTO coupon_campaign_previews(token,template_id,actor_id,snapshot,expires_at) VALUES($1,$2,$3,$4,$5)`, result.Token, id, actorID, raw, result.ExpiresAt)
		return err
	})
	return result, err
}
func (r *couponRepository) ExecuteCouponCampaign(ctx context.Context, token string, actorID int64, at time.Time) (*service.CouponCampaignResult, error) {
	var result *service.CouponCampaignResult
	err := r.withTx(ctx, func(exec sqlQueryExecutor) error {
		var raw []byte
		var actor int64
		var batchID *int64
		err := scanSingleRow(ctx, exec, `SELECT snapshot,actor_id,batch_id FROM coupon_campaign_previews WHERE token=$1 FOR UPDATE`, []any{token}, &raw, &actor, &batchID)
		if errors.Is(err, sql.ErrNoRows) {
			return infraerrors.NotFound("COUPON_PREVIEW_NOT_FOUND", "preview expired; preview again")
		}
		if err != nil {
			return err
		}
		if actor != actorID {
			return infraerrors.NotFound("COUPON_PREVIEW_NOT_FOUND", "preview not found")
		}
		var preview service.CouponCampaignPreview
		if err = json.Unmarshal(raw, &preview); err != nil {
			return err
		}
		result = &service.CouponCampaignResult{Skipped: preview.Skipped}
		if batchID != nil {
			result.BatchID = *batchID
			var snap []byte
			err = scanSingleRow(ctx, exec, `SELECT issued_count,input_snapshot FROM coupon_issue_batches WHERE id=$1`, []any{*batchID}, &result.IssuedCount, &snap)
			if err == nil {
				var saved struct {
					Skipped []service.CouponRecipient `json:"skipped"`
				}
				if err = json.Unmarshal(snap, &saved); err == nil {
					result.Skipped = saved.Skipped
				}
			}
			return err
		}
		if !preview.ExpiresAt.After(at) {
			return infraerrors.Conflict("COUPON_PREVIEW_EXPIRED", "preview expired; preview again")
		}
		t, err := getCouponTemplate(ctx, exec, preview.TemplateID, true)
		if err != nil {
			return err
		}
		if t == nil || t.Status != service.CouponTemplateStatusActive || t.Version != preview.TemplateVersion {
			return infraerrors.Conflict("COUPON_PREVIEW_STALE", "template changed; preview again")
		}
		p, err := couponPolicy(ctx, exec, t.ID, true)
		if err != nil {
			return err
		}
		if p.Version != preview.PolicyVersion {
			return infraerrors.Conflict("COUPON_PREVIEW_STALE", "claim policy changed; preview again")
		}
		if len(preview.Recipients) == 0 {
			return infraerrors.BadRequest("COUPON_AUDIENCE_EMPTY", "no users eligible")
		}
		ids, err := couponAudienceIDs(ctx, exec, service.CouponAudience{Mode: "users", UserIDs: preview.Recipients}, 0, true)
		if err != nil {
			return err
		}
		active := map[int64]bool{}
		for _, id := range ids {
			active[id] = true
		}
		for _, id := range preview.Recipients {
			if !active[id] {
				result.Skipped = append(result.Skipped, service.CouponRecipient{UserID: id, Reason: "account_ineligible"})
			}
		}
		ids, skipped, err := couponCampaignEligibility(ctx, exec, t.ID, ids, p.PerUserLimit, at)
		if err != nil {
			return err
		}
		result.Skipped = append(result.Skipped, skipped...)
		if t.TotalIssueLimit != nil && *t.TotalIssueLimit-t.IssuedCount < int64(len(ids)) {
			return infraerrors.Conflict("COUPON_TEMPLATE_STOCK_EXHAUSTED", "not enough stock for this batch")
		}
		snapshot, err := json.Marshal(map[string]any{"audience": preview.Audience, "user_ids": ids, "skipped": result.Skipped})
		if err != nil {
			return err
		}
		err = scanSingleRow(ctx, exec, `INSERT INTO coupon_issue_batches(template_id,source,requested_count,issued_count,failed_count,status,idempotency_key,input_snapshot,created_by,completed_at) VALUES($1,'admin_batch',$2,$3,0,'completed',$4,$5,$6,$7) RETURNING id`, []any{t.ID, preview.RequestedCount, len(ids), "coupon-campaign:" + token, snapshot, actorID, at}, &result.BatchID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			_, err = issueCouponWithTemplate(ctx, exec, t, service.CouponIssueInput{TemplateID: t.ID, UserID: id, Source: service.CouponIssueSourceAdminBatch, IssueBatchID: &result.BatchID, SourceRef: fmt.Sprintf("batch:%d", result.BatchID), IdempotencyKey: fmt.Sprintf("coupon-batch:%d:user:%d", result.BatchID, id), ActorID: actorID}, at)
			if err != nil {
				return err
			}
		}
		result.IssuedCount = len(ids)
		if err = insertCouponEvent(ctx, exec, couponEventInput{
			TemplateID: &t.ID, IssueBatchID: &result.BatchID,
			EventType: "batch_completed", ActorType: "admin", ActorUserID: &actorID,
			Metadata: map[string]any{"requested_count": preview.RequestedCount, "issued_count": result.IssuedCount, "skipped_count": len(result.Skipped)},
		}); err != nil {
			return err
		}
		_, err = exec.ExecContext(ctx, `UPDATE coupon_campaign_previews SET batch_id=$2 WHERE token=$1`, token, result.BatchID)
		return err
	})
	return result, err
}
func (r *couponRepository) CouponSummary(ctx context.Context, f service.UserCouponListFilter) (*service.CouponSummary, error) {
	if err := r.ExpireAvailableUserCoupons(ctx, f.UserID, time.Now()); err != nil {
		return nil, err
	}
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	where, args := couponUserFilter(f)
	result := &service.CouponSummary{Amounts: []service.CouponCurrencySummary{}}
	err = scanSingleRow(ctx, exec, `SELECT COUNT(*),COUNT(*) FILTER(WHERE uc.status='available' AND uc.valid_from<=NOW()),COUNT(*) FILTER(WHERE uc.status='used') FROM user_coupons uc LEFT JOIN users u ON u.id=uc.user_id`+where, args, &result.Issued, &result.Available, &result.Used)
	if err != nil {
		return nil, err
	}
	rows, err := exec.QueryContext(ctx, `SELECT COALESCE(NULLIF(po.payment_currency,''),'CNY'),po.pay_amount,po.discount_amount,po.amount,po.refund_amount,po.status FROM user_coupons uc LEFT JOIN users u ON u.id=uc.user_id JOIN payment_orders po ON po.id=uc.used_order_id`+where+` AND uc.status='used' ORDER BY po.id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	type amounts struct{ paid, discount, refunded decimal.Decimal }
	totals := map[string]amounts{}
	for rows.Next() {
		var currency, status string
		var paid, discount, amount, refund float64
		if err = rows.Scan(&currency, &paid, &discount, &amount, &refund, &status); err != nil {
			return nil, err
		}
		a := totals[currency]
		a.paid = a.paid.Add(decimal.NewFromFloat(paid))
		a.discount = a.discount.Add(decimal.NewFromFloat(discount))
		a.refunded = a.refunded.Add(decimal.NewFromFloat(service.CouponOrderCashRefund(amount, paid, refund, currency, status)))
		totals[currency] = a
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	currencies := make([]string, 0, len(totals))
	for c := range totals {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	for _, c := range currencies {
		a := totals[c]
		result.Amounts = append(result.Amounts, service.CouponCurrencySummary{Currency: c, Paid: a.paid.InexactFloat64(), Discount: a.discount.InexactFloat64(), Refunded: a.refunded.InexactFloat64(), NetPaid: a.paid.Sub(a.refunded).InexactFloat64()})
	}
	return result, nil
}
func (r *couponRepository) CouponEvents(ctx context.Context, id int64) ([]service.CouponEvent, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := exec.QueryContext(ctx, `SELECT id,event_type,order_id,actor_type,created_at,metadata FROM coupon_events WHERE coupon_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.CouponEvent{}
	for rows.Next() {
		var e service.CouponEvent
		var raw []byte
		if err = rows.Scan(&e.ID, &e.EventType, &e.OrderID, &e.ActorType, &e.CreatedAt, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &e.Metadata); err != nil {
			return nil, err
		}
		delete(e.Metadata, "idempotency_key")
		out = append(out, e)
	}
	return out, rows.Err()
}

var _ service.CouponCampaignRepository = (*couponRepository)(nil)

func couponManualIssueAllowed(ctx context.Context, exec sqlQueryExecutor, templateID int64, ids []int64, source service.CouponIssueSource, at time.Time) error {
	if source != service.CouponIssueSourceAdminBatch && source != service.CouponIssueSourceManual && source != service.CouponIssueSourceCompensate {
		return nil
	}
	active, err := couponAudienceIDs(ctx, exec, service.CouponAudience{Mode: "users", UserIDs: ids}, 0, true)
	if err != nil {
		return err
	}
	if len(active) != len(ids) {
		return infraerrors.Conflict("COUPON_AUDIENCE_MISMATCH", "recipient account is inactive, missing or an administrator")
	}
	p, err := couponPolicy(ctx, exec, templateID, true)
	if err != nil {
		return err
	}
	_, skipped, err := couponCampaignEligibility(ctx, exec, templateID, ids, p.PerUserLimit, at)
	if err != nil {
		return err
	}
	if len(skipped) > 0 {
		return infraerrors.Conflict("COUPON_PER_USER_LIMIT", "recipient already holds a coupon or reached the campaign limit; use campaign preview")
	}
	return nil
}

func (r *couponRepository) ListCouponClaimPolicies(ctx context.Context, ids []int64) ([]service.CouponClaimPolicy, error) {
	exec, err := r.exec(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := exec.QueryContext(ctx, `SELECT cc.template_id,cc.code,cc.enabled,cc.claim_limit,cc.claimed_count,COUNT(uc.id),COUNT(uc.id) FILTER(WHERE uc.status='used'),COUNT(uc.id) FILTER(WHERE uc.source IN ('admin_batch','manual','compensation')) FROM coupon_claim_codes cc LEFT JOIN user_coupons uc ON uc.template_id=cc.template_id WHERE cc.template_id=ANY($1) GROUP BY cc.template_id ORDER BY cc.template_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []service.CouponClaimPolicy{}
	for rows.Next() {
		var p service.CouponClaimPolicy
		if err = rows.Scan(&p.TemplateID, &p.Code, &p.Enabled, &p.ClaimLimit, &p.ClaimedCount, &p.IssuedCount, &p.UsedCount, &p.ManualCount); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
