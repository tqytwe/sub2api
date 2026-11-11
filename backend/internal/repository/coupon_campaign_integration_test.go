//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func couponCampaignFixture(t *testing.T) (*couponRepository, *service.CouponTemplate, *service.User, []int64) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	actor := mustCreateUser(t, client, &service.User{Role: service.RoleAdmin})
	ids := []int64{}
	for i := 0; i < 3; i++ {
		ids = append(ids, mustCreateUser(t, client, &service.User{}).ID)
	}
	repo := &couponRepository{db: integrationDB}
	limit := int64(10)
	template, err := service.NewCouponService(repo).CreateTemplate(ctx, service.CouponTemplateInput{Name: "Campaign test", Status: service.CouponTemplateStatusActive, BenefitType: service.CouponBenefitTypePercentage, BenefitValue: 30, Currency: "CNY", ApplicableScopes: []service.CouponScope{service.CouponScopeBalance}, ValidityMode: service.CouponValidityModeRelativeDays, ValidityDays: 2, TotalIssueLimit: &limit}, actor.ID)
	require.NoError(t, err)
	return repo, template, actor, ids
}
func TestCouponCampaignSnapshotReplayAndSharedClaimLimit(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p, err := repo.GetCouponClaimPolicy(ctx, template.ID)
	require.NoError(t, err)
	require.NotEmpty(t, p.Code)
	require.False(t, p.Enabled)
	preview, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "users", UserIDs: append(append([]int64{}, ids[:2]...), actor.ID)}, actor.ID, now)
	require.NoError(t, err)
	require.Equal(t, ids[:2], preview.Recipients)
	require.Len(t, preview.Skipped, 1)
	_, err = repo.ExecuteCouponCampaign(ctx, preview.Token, ids[0], now)
	require.Equal(t, "COUPON_PREVIEW_NOT_FOUND", infraerrors.Reason(err))
	result, err := repo.ExecuteCouponCampaign(ctx, preview.Token, actor.ID, now)
	require.NoError(t, err)
	require.Equal(t, 2, result.IssuedCount)
	replay, err := repo.ExecuteCouponCampaign(ctx, preview.Token, actor.ID, now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, result, replay)
	held, err := repo.ClaimCouponCode(ctx, p.Code, ids[0], now)
	require.NoError(t, err)
	require.Equal(t, service.CouponIssueSourceAdminBatch, held.Source)
	second, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "users", UserIDs: ids}, actor.ID, now)
	require.NoError(t, err)
	require.Equal(t, ids[2:], second.Recipients)
	require.Len(t, second.Skipped, 2)
	end := now.Add(time.Hour)
	p.Enabled = true
	p.ClaimLimit = 1
	p.EndsAt = &end
	p, err = repo.SaveCouponClaimPolicy(ctx, template.ID, *p, actor.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), p.IssuedCount)
	_, err = repo.ExecuteCouponCampaign(ctx, second.Token, actor.ID, now)
	require.Equal(t, "COUPON_PREVIEW_STALE", infraerrors.Reason(err))
	claimed, err := repo.ClaimCouponCode(ctx, p.Code, ids[2], now)
	require.NoError(t, err)
	require.Equal(t, service.CouponIssueSourceCodeClaim, claimed.Source)
	quote, err := service.QuoteUserCoupon(*claimed, service.CouponOrderContext{UserID: ids[2], Scope: service.CouponScopeBalance, OrderAmount: 100, Currency: "CNY", At: now.Add(time.Second)})
	require.NoError(t, err)
	require.Equal(t, 30.0, quote.DiscountAmount)
	require.Equal(t, 70.0, quote.PayableAmount)
	again, err := repo.ClaimCouponCode(ctx, p.Code, ids[2], now)
	require.NoError(t, err)
	require.Equal(t, claimed.ID, again.ID)
	summary, err := repo.CouponSummary(ctx, service.UserCouponListFilter{TemplateID: template.ID, Page: 2, PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.Issued)
	events, err := repo.CouponEvents(ctx, claimed.ID)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	require.Equal(t, "user", events[0].ActorType)
}

func TestCouponCampaignExpiredPreviewDisabledRecipientAndConcurrentReplay(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "users", UserIDs: ids}, actor.ID, now)
	require.NoError(t, err)
	_, err = repo.ExecuteCouponCampaign(ctx, p.Token, actor.ID, now.Add(16*time.Minute))
	require.Equal(t, "COUPON_PREVIEW_EXPIRED", infraerrors.Reason(err))
	_, err = integrationDB.Exec(`UPDATE users SET status='disabled' WHERE id=$1`, ids[1])
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan *service.CouponCampaignResult, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := repo.ExecuteCouponCampaign(ctx, p.Token, actor.ID, now)
			results <- r
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	var batchID int64
	for r := range results {
		require.Equal(t, 2, r.IssuedCount)
		require.Len(t, r.Skipped, 1)
		if batchID != 0 {
			require.Equal(t, batchID, r.BatchID)
		}
		batchID = r.BatchID
	}
}
func TestCouponCampaignLastClaimConcurrency(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p, err := repo.GetCouponClaimPolicy(ctx, template.ID)
	require.NoError(t, err)
	end := now.Add(time.Hour)
	p.Enabled = true
	p.ClaimLimit = 1
	p.EndsAt = &end
	p, err = repo.SaveCouponClaimPolicy(ctx, template.ID, *p, actor.ID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	out := make(chan error, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); _, e := repo.ClaimCouponCode(ctx, p.Code, id, now); out <- e }(id)
	}
	wg.Wait()
	close(out)
	success := 0
	for e := range out {
		if e == nil {
			success++
		} else {
			require.Equal(t, "COUPON_CODE_EXHAUSTED", infraerrors.Reason(e))
		}
	}
	require.Equal(t, 1, success)
	p, err = repo.GetCouponClaimPolicy(ctx, template.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), p.ClaimedCount)
	require.Equal(t, int64(1), p.IssuedCount)
}
func TestCouponCampaignFixedAudienceExcludesLaterMatches(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	prefix := fmt.Sprintf("coupon-audience-%d", template.ID)
	_, err := integrationDB.Exec("UPDATE users SET username=$1 WHERE id=$2", prefix, ids[0])
	require.NoError(t, err)
	preview, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "filtered", Search: prefix}, actor.ID, now)
	require.NoError(t, err)
	require.Equal(t, ids[:1], preview.Recipients)
	_, err = integrationDB.Exec("UPDATE users SET username=$1 WHERE id=$2", prefix, ids[1])
	require.NoError(t, err)
	result, err := repo.ExecuteCouponCampaign(ctx, preview.Token, actor.ID, now)
	require.NoError(t, err)
	require.Equal(t, 1, result.IssuedCount)
}

func TestCouponCampaignSummaryRefundCurrenciesAndPaging(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	preview, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "users", UserIDs: ids}, actor.ID, at)
	require.NoError(t, err)
	_, err = repo.ExecuteCouponCampaign(ctx, preview.Token, actor.ID, at)
	require.NoError(t, err)
	for i, currency := range []string{"CNY", "JPY", "CNY"} {
		var orderID int64
		require.NoError(t, integrationDB.QueryRow(`INSERT INTO payment_orders(user_id,amount,pay_amount,discount_amount,payment_currency,status,refund_amount,expires_at) VALUES($1,100,103,10,$2,'PARTIALLY_REFUNDED',50,NOW()+INTERVAL '1 hour') RETURNING id`, ids[i], currency).Scan(&orderID))
		_, err = integrationDB.Exec(`UPDATE user_coupons SET status='used',used_order_id=$1,used_at=NOW() WHERE template_id=$2 AND user_id=$3`, orderID, template.ID, ids[i])
		require.NoError(t, err)
	}
	summary, err := repo.CouponSummary(ctx, service.UserCouponListFilter{TemplateID: template.ID, Page: 2, PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.Used)
	require.Equal(t, []service.CouponCurrencySummary{{Currency: "CNY", Paid: 206, Discount: 20, Refunded: 103, NetPaid: 103}, {Currency: "JPY", Paid: 103, Discount: 10, Refunded: 52, NetPaid: 51}}, summary.Amounts)
}

func TestCouponCampaignPreviewDoesNotPreventDeletingUnissuedTemplate(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	_, err := repo.PreviewCouponCampaign(context.Background(), template.ID, service.CouponAudience{Mode: "users", UserIDs: ids}, actor.ID, time.Now().UTC())
	require.NoError(t, err)
	require.NoError(t, service.NewCouponService(repo).DeleteTemplate(context.Background(), template.ID))
}

func TestCouponCampaignStockRollbackAndManualCap(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := integrationDB.Exec(`UPDATE coupon_templates SET total_issue_limit=1 WHERE id=$1`, template.ID)
	require.NoError(t, err)
	preview, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "users", UserIDs: ids}, actor.ID, now)
	require.NoError(t, err)
	_, err = repo.ExecuteCouponCampaign(ctx, preview.Token, actor.ID, now)
	require.Equal(t, "COUPON_TEMPLATE_STOCK_EXHAUSTED", infraerrors.Reason(err))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM user_coupons WHERE template_id=$1`, template.ID).Scan(&count))
	require.Zero(t, count)
	input := service.CouponIssueInput{TemplateID: template.ID, UserID: ids[0], Source: service.CouponIssueSourceManual, IdempotencyKey: fmt.Sprintf("manual-cap:%d", template.ID), ActorID: actor.ID}
	_, err = service.NewCouponService(repo).IssueCoupon(ctx, input)
	require.NoError(t, err)
	input.IdempotencyKey += "-repeat"
	_, err = service.NewCouponService(repo).IssueCoupon(ctx, input)
	require.Equal(t, "COUPON_PER_USER_LIMIT", infraerrors.Reason(err))
}

func TestCouponCampaignCombinedRecipientFilters(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	var groupID, attributeID int64
	name := fmt.Sprintf("coupon-filter-%d", template.ID)
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO groups(name) VALUES($1) RETURNING id`, name).Scan(&groupID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO user_attribute_definitions(key,name,type) VALUES($1,'Campaign audience','text') RETURNING id`, name).Scan(&attributeID))
	for _, id := range ids[:2] {
		_, err := integrationDB.Exec(`INSERT INTO user_allowed_groups(user_id,group_id) VALUES($1,$2)`, id, groupID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO api_keys(user_id,key,name,group_id) VALUES($1,$2,'campaign test',$3)`, id, fmt.Sprintf("fixture-coupon-%d", id), groupID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO user_attribute_values(user_id,attribute_id,value) VALUES($1,$2,'selected')`, id, attributeID)
		require.NoError(t, err)
	}
	_, err := integrationDB.Exec(`UPDATE api_keys SET deleted_at=NOW() WHERE user_id=$1`, ids[1])
	require.NoError(t, err)
	tier := 0
	p, err := repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "filtered", GroupName: name, APIKeyGroupID: groupID, VIPTier: &tier, Attributes: map[int64]string{attributeID: "selected"}}, actor.ID, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, ids[:1], p.Recipients)
	// The selected group can be an additional binding, not the legacy default.
	_, err = integrationDB.Exec(`INSERT INTO api_key_groups(api_key_id,group_id) SELECT id,group_id FROM api_keys WHERE user_id=ANY($1)`, pq.Array(ids[:2]))
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE api_keys SET group_id=NULL WHERE user_id=ANY($1)`, pq.Array(ids[:2]))
	require.NoError(t, err)
	p, err = repo.PreviewCouponCampaign(ctx, template.ID, service.CouponAudience{Mode: "filtered", APIKeyGroupID: groupID}, actor.ID, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, ids[:1], p.Recipients)
}

func TestCouponCampaignMigrationBackfillAndRepeatPreserveIssuedCoupon(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	coupon, err := service.NewCouponService(repo).IssueCoupon(ctx, service.CouponIssueInput{TemplateID: template.ID, UserID: ids[0], Source: service.CouponIssueSourceManual, ActorID: actor.ID, IdempotencyKey: fmt.Sprintf("upgrade:%d", template.ID)})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`DELETE FROM coupon_claim_codes WHERE template_id=$1`, template.ID)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("353_coupon_claim_campaign.sql")
	require.NoError(t, err)
	_, err = integrationDB.Exec(string(raw))
	require.NoError(t, err)
	policy, err := repo.GetCouponClaimPolicy(ctx, template.ID)
	require.NoError(t, err)
	require.False(t, policy.Enabled)
	_, err = integrationDB.Exec(string(raw))
	require.NoError(t, err)
	repeated, err := repo.GetCouponClaimPolicy(ctx, template.ID)
	require.NoError(t, err)
	require.Equal(t, policy.Code, repeated.Code)
	after, err := repo.GetUserCoupon(ctx, coupon.ID)
	require.NoError(t, err)
	require.Equal(t, coupon.TermsSnapshot, after.TermsSnapshot)
	require.Equal(t, coupon.Status, after.Status)
	require.NoError(t, ApplyMigrations(ctx, integrationDB))
}

func TestCouponCampaignExpiryIsRecordedOnceAndIncludedInSummary(t *testing.T) {
	repo, template, actor, ids := couponCampaignFixture(t)
	ctx := context.Background()
	coupon, err := service.NewCouponService(repo).IssueCoupon(ctx, service.CouponIssueInput{TemplateID: template.ID, UserID: ids[0], Source: service.CouponIssueSourceManual, ActorID: actor.ID, IdempotencyKey: fmt.Sprintf("expiry:%d", template.ID)})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE user_coupons SET valid_from=NOW()-INTERVAL '2 days',expires_at=NOW()-INTERVAL '1 day' WHERE id=$1`, coupon.ID)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		summary, err := repo.CouponSummary(ctx, service.UserCouponListFilter{TemplateID: template.ID})
		require.NoError(t, err)
		require.Equal(t, int64(1), summary.Issued)
		require.Zero(t, summary.Available)
	}
	events, err := repo.CouponEvents(ctx, coupon.ID)
	require.NoError(t, err)
	expired := 0
	for _, event := range events {
		if event.EventType == "expired" {
			expired++
		}
	}
	require.Equal(t, 1, expired)
}
