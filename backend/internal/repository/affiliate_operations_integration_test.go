//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateOperationsLeaderboardKeepsMyRankOutsidePublicTopTen(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	empty, err := r.GetAffiliateOperationsLeaderboard(ctx, service.AffiliateOperationsQuery{Month: "2026-01", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Empty(t, empty.Items)
	require.Nil(t, empty.MyRank)
	var current int64
	for i := 0; i < 12; i++ {
		inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("rank-%d-%d@private-company.example", time.Now().UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("invitee-%d-%d@example.com", time.Now().UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		_, err := client.ExecContext(ctx, `INSERT INTO affiliate_paid_invite_facts(invitee_id,inviter_id,bound_at,evidence_status,evidence_source,first_paid_at,first_paid_month,net_paid) VALUES($1,$2,'2026-01-01','verified','live_binding',$3,'2026-01',10)`, invitee.ID, inviter.ID, time.Date(2026, 1, 2+i, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		current = inviter.ID
		if i == 0 {
			first, readErr := r.GetAffiliateOperationsLeaderboard(ctx, service.AffiliateOperationsQuery{Month: "2026-01", UserID: current, Page: 1, PageSize: 20})
			require.NoError(t, readErr)
			require.Len(t, first.Items, 1)
		}
	}
	q := service.AffiliateOperationsQuery{Month: "2026-01", UserID: current, Page: 1, PageSize: 20}
	board, err := r.GetAffiliateOperationsLeaderboard(ctx, q)
	require.NoError(t, err)
	require.Len(t, board.Items, 10)
	require.NotNil(t, board.MyRank)
	require.EqualValues(t, 12, board.MyRank.Rank)
	for _, item := range board.Items {
		require.Zero(t, item.UserID)
		require.Empty(t, item.Email)
		require.NotContains(t, item.DisplayName, "private-company")
		require.NotContains(t, item.DisplayName, "rank-")
	}
	_, err = client.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, current)
	require.NoError(t, err)
	board, err = r.GetAffiliateOperationsLeaderboard(ctx, q)
	require.NoError(t, err)
	require.Nil(t, board.MyRank)
}

func TestAffiliateOperationsProductionAugustCohortMatchesAdminAndUserViews(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	inviter := mustCreateUser(t, client, &service.User{Email: "guolingzheng@gmail.com", PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	for i := 0; i < 4; i++ {
		invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("august-invitee-%d-%d@example.com", time.Now().UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		bound, bindErr := r.BindInviter(ctx, invitee.ID, inviter.ID)
		require.NoError(t, bindErr)
		require.True(t, bound)
		_, err := client.ExecContext(ctx, `UPDATE users SET created_at='2026-08-01' WHERE id=$1`, invitee.ID)
		require.NoError(t, err)
		if i == 0 {
			_, err := client.ExecContext(ctx, `UPDATE affiliate_paid_invite_facts SET bound_at='2026-08-01',evidence_status='verified',evidence_source='live_binding',first_paid_at='2026-08-05T00:00:00Z',first_paid_month='2026-08',net_paid=10 WHERE invitee_id=$1`, invitee.ID)
			require.NoError(t, err)
			continue
		}
		_, err = client.ExecContext(ctx, `UPDATE affiliate_paid_invite_facts SET bound_at='2026-08-01',evidence_status='verified',evidence_source='live_binding' WHERE invitee_id=$1`, invitee.ID)
		require.NoError(t, err)
	}

	adminQuery := service.AffiliateOperationsQuery{Month: "2026-08", Admin: true, Page: 1, PageSize: 20}
	adminSummary, err := r.GetAffiliateOperationsSummary(ctx, adminQuery)
	require.NoError(t, err)
	require.EqualValues(t, 4, adminSummary.InvitedCount)
	require.EqualValues(t, 1, adminSummary.EffectivePaidCount)
	require.InDelta(t, 0.25, adminSummary.ConversionRate, 0.000001)

	adminBoard, err := r.GetAffiliateOperationsLeaderboard(ctx, adminQuery)
	require.NoError(t, err)
	require.Len(t, adminBoard.Items, 1)
	require.Equal(t, "guolingzheng@gmail.com", adminBoard.Items[0].Email)
	require.EqualValues(t, 1, adminBoard.Items[0].EffectivePaidCount)

	userQuery := service.AffiliateOperationsQuery{Month: "2026-08", UserID: inviter.ID, Page: 1, PageSize: 20}
	userBoard, err := r.GetAffiliateOperationsLeaderboard(ctx, userQuery)
	require.NoError(t, err)
	require.Len(t, userBoard.Items, 1)
	require.Equal(t, "gu***@g***.com", userBoard.Items[0].DisplayName)
	require.Empty(t, userBoard.Items[0].Email)
	require.EqualValues(t, 1, userBoard.Items[0].EffectivePaidCount)
}

func TestAffiliateOperationsReconcilesRegistrationWithoutInventingHistoricalBinding(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("reconcile-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	for i := 0; i < 4; i++ {
		invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("reconcile-invite-%d-%d@example.com", time.Now().UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		bound, err := r.BindInviter(ctx, invitee.ID, inviter.ID)
		require.NoError(t, err)
		require.True(t, bound)
		registered := "2026-10-31T16:00:00Z" // November starts at 16:00 UTC.
		if i == 1 {
			registered = "2026-10-31T15:59:59Z"
		}
		_, err = client.ExecContext(ctx, `UPDATE users SET created_at=$2 WHERE id=$1`, invitee.ID, registered)
		require.NoError(t, err)
		if i == 2 {
			_, err = client.ExecContext(ctx, `UPDATE affiliate_paid_invite_facts SET bound_at='2026-11-01',evidence_status='verified',first_paid_at='2026-11-02',first_paid_month='2026-11',net_paid=10 WHERE invitee_id=$1`, invitee.ID)
		} else {
			_, err = client.ExecContext(ctx, `UPDATE user_affiliates SET bound_at=NULL WHERE user_id=$1`, invitee.ID)
			require.NoError(t, err)
			_, err = client.ExecContext(ctx, `UPDATE affiliate_paid_invite_facts SET bound_at=NULL WHERE invitee_id=$1`, invitee.ID)
		}
		require.NoError(t, err)
		if i == 0 {
			_, err = client.ExecContext(ctx, `UPDATE affiliate_paid_invite_facts SET first_paid_at='2026-11-02',first_paid_month='2026-11' WHERE invitee_id=$1`, invitee.ID)
			require.NoError(t, err)
		}
		if i == 3 {
			_, err = client.ExecContext(ctx, `DELETE FROM affiliate_paid_invite_facts WHERE invitee_id=$1`, invitee.ID)
			require.NoError(t, err)
		}
	}
	q := service.AffiliateOperationsQuery{Month: "2026-11", UserID: inviter.ID, Admin: true, Page: 1, PageSize: 20}
	summary, err := r.GetAffiliateOperationsSummary(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.InvitedCount, "registration is not evidence of the historical binding date")
	require.EqualValues(t, 1, summary.EffectivePaidCount)
	require.InDelta(t, 1.0/3.0, summary.ConversionRate, 0.000001, "conversion compares the registered cohort, including historical bindings")
	raw, err := json.Marshal(summary)
	require.NoError(t, err)
	var response map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &response))
	var reconciliation map[string]int64
	require.NoError(t, json.Unmarshal(response["reconciliation"], &reconciliation))
	require.Equal(t, map[string]int64{
		"relationship_count": 4, "registered_month_count": 3, "registered_effective_paid_count": 1,
		"known_binding_count": 1, "unknown_binding_count": 3,
		"effective_paid_count": 1, "unpaid_count": 0, "refunded_count": 0,
		"risk_excluded_count": 0, "disabled_count": 0, "pending_count": 3,
		"missing_fact_count": 1, "pending_payment_count": 1,
	}, reconciliation)
	board, err := r.GetAffiliateOperationsLeaderboard(ctx, q)
	require.NoError(t, err)
	require.Len(t, board.Items, 1, "unverified bindings cannot enter the paid leaderboard")
	var unknown int
	require.NoError(t, scanAffiliateRow(ctx, client, `SELECT COUNT(*) FROM user_affiliates WHERE inviter_id=$1 AND bound_at IS NULL`, []any{inviter.ID}, &unknown))
	require.Equal(t, 3, unknown, "read-side reconciliation must not repair production evidence")
}

func TestAffiliateOperationsPaginationAndUnknownHistoryRemainHonest(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("promoter-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	for i := 0; i < 105; i++ {
		invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("invite-%d-%d@secret.example", time.Now().UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		_, err := client.ExecContext(ctx, `INSERT INTO affiliate_paid_invite_facts(invitee_id,inviter_id,evidence_status) VALUES($1,$2,'pending')`, invitee.ID, inviter.ID)
		require.NoError(t, err)
	}
	q := service.AffiliateOperationsQuery{UserID: inviter.ID, Page: 6, PageSize: 20, Month: "2026-01"}
	page, err := r.ListAffiliateOperationsInvitees(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 105, page.Total)
	require.Len(t, page.Items, 5)
	require.Equal(t, "pending_evidence", page.Items[0].Status)
	require.Nil(t, page.Items[0].InvitedAt)
	require.Empty(t, page.Items[0].InviteeEmail)
	summary, err := r.GetAffiliateOperationsSummary(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 105, summary.InvitedCount)
	require.EqualValues(t, 0, summary.EffectivePaidCount)
	require.EqualValues(t, 105, summary.PendingCount)
}

func TestAffiliateOperationsQueriesSeparateCurrenciesAndExposeAllHistory(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("ops-money-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	_, err := r.EnsureUserAffiliate(ctx, u.ID)
	require.NoError(t, err)
	var campaign, reward int64
	now := time.Now().UTC()
	require.NoError(t, scanAffiliateRow(ctx, client, `INSERT INTO referral_campaigns(campaign_key,name,status,registration_from,registration_to,starts_at,ends_at,qualification_to,claim_deadline,risk_hold_hours,pay_threshold,usage_threshold,max_enrollments,budget_total,reward_mode,signing_secret) VALUES($1,'test','running',$2,$3,$2,$3,$3,$3::timestamptz+INTERVAL '1 hour',168,0,0,10,100,'additive',$4) RETURNING id`, []any{fmt.Sprintf("ops-money-%d", u.ID), now.Add(-time.Hour), now.Add(time.Hour), []byte("fixture")}, &campaign))
	require.NoError(t, scanAffiliateRow(ctx, client, `INSERT INTO referral_campaign_rewards(campaign_id,user_id,tier_no,reward_type,amount,currency,status,idempotency_key,claim_deadline) VALUES($1,$2,1,'invite',10,'CNY','available',$3,NOW()+INTERVAL '1 hour') RETURNING id`, []any{campaign, u.ID, fmt.Sprintf("ops-money-%d", u.ID)}, &reward))
	_, err = client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount) VALUES($1,'accrue',5),($1,'transfer',2),($1,'withdraw',1);`, u.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount,referral_reward_id) VALUES($1,'accrue',10,$2),($1,'revoke',-2,$2)`, u.ID, reward)
	require.NoError(t, err)
	q, err := service.NormalizeAffiliateOperationsQuery(service.AffiliateOperationsQuery{UserID: u.ID, Admin: true}, now)
	require.NoError(t, err)
	summary, err := r.GetAffiliateOperationsSummary(ctx, q)
	require.NoError(t, err)
	require.Equal(t, "mixed", summary.Currency)
	require.Nil(t, summary.NetEarned)
	require.Nil(t, summary.MonthNetEarned)
	require.Len(t, summary.Amounts, 2)
	require.Len(t, summary.PeriodAmounts, 2)
	require.NotEmpty(t, summary.Trend)
	earnings, err := r.ListAffiliateOperationsEarnings(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 5, earnings.Total)
	for _, e := range earnings.Items {
		if e.SourceType == "campaign_reward" {
			require.Equal(t, "CNY", e.Currency)
		}
	}
	promoters, err := r.ListAffiliateOperationsPromoters(ctx, q)
	require.NoError(t, err)
	require.Len(t, promoters.Items, 1)
	require.Equal(t, "mixed", promoters.Items[0].Currency)
	require.Nil(t, promoters.Items[0].NetEarned)
	require.Len(t, promoters.Items[0].Amounts, 2)
	audits, err := r.ListAffiliateOperationsAudits(ctx, q)
	require.NoError(t, err)
	require.Empty(t, audits.Items)
	history, err := r.ListAffiliateOperationsCampaigns(ctx, q)
	require.NoError(t, err)
	require.Empty(t, history.Items)
	_, err = client.ExecContext(ctx, `INSERT INTO referral_campaign_enrollments(campaign_id,user_id) VALUES($1,$2)`, campaign, u.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO referral_campaign_audit_logs(campaign_id,campaign_version,actor_id,action,detail) VALUES($1,1,$2,'reviewed','{}')`, campaign, u.ID)
	require.NoError(t, err)
	audits, err = r.ListAffiliateOperationsAudits(ctx, q)
	require.NoError(t, err)
	require.Len(t, audits.Items, 1)
	require.Equal(t, "campaign_relation", audits.Items[0].Detail["scope"])
	q.Scope = "joined"
	history, err = r.ListAffiliateOperationsCampaigns(ctx, q)
	require.NoError(t, err)
	require.Len(t, history.Items, 1)
	q.Scope = "ended"
	history, err = r.ListAffiliateOperationsCampaigns(ctx, q)
	require.NoError(t, err)
	require.Empty(t, history.Items)
}

func TestAffiliateOperationsRiskLinkIsAdminOnlyAndExcludedFromRanking(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("ops-risk-promoter-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("ops-risk-invitee-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	_, err := client.ExecContext(ctx, `INSERT INTO affiliate_paid_invite_facts(invitee_id,inviter_id,evidence_status,evidence_source,first_paid_at,first_paid_month,net_paid) VALUES($1,$2,'verified','live_binding','2026-01-02','2026-01',10)`, invitee.ID, inviter.ID)
	require.NoError(t, err)
	var risk int64
	require.NoError(t, scanAffiliateRow(ctx, client, `INSERT INTO ip_risk_cases(case_key,primary_ip,primary_network,level,status,first_detected_at,last_detected_at) VALUES($1,'192.0.2.1','192.0.2.0/24','high','open',NOW(),NOW()) RETURNING id`, []any{fmt.Sprintf("affiliate-risk-%d", invitee.ID)}, &risk))
	_, err = client.ExecContext(ctx, `INSERT INTO ip_risk_case_users(case_id,user_id,relation_type,first_seen_at,last_seen_at) VALUES($1,$2,'suspected_new',NOW(),NOW())`, risk, invitee.ID)
	require.NoError(t, err)
	q, err := service.NormalizeAffiliateOperationsQuery(service.AffiliateOperationsQuery{UserID: inviter.ID, Admin: true, Month: "2026-01"}, time.Now())
	require.NoError(t, err)
	invites, err := r.ListAffiliateOperationsInvitees(ctx, q)
	require.NoError(t, err)
	require.Len(t, invites.Items, 1)
	require.Equal(t, "risk_excluded", invites.Items[0].Status)
	require.Equal(t, &risk, invites.Items[0].RiskCaseID)
	q.Admin = false
	invites, err = r.ListAffiliateOperationsInvitees(ctx, q)
	require.NoError(t, err)
	require.Nil(t, invites.Items[0].RiskCaseID)
	board, err := r.GetAffiliateOperationsLeaderboard(ctx, q)
	require.NoError(t, err)
	require.Nil(t, board.MyRank)
}

func TestAffiliateOperationsCampaignRankAndIndependentQualificationRiskFilters(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	now := time.Now().UTC()
	var campaign, current int64
	require.NoError(t, scanAffiliateRow(ctx, client, `INSERT INTO referral_campaigns(campaign_key,name,status,registration_from,registration_to,starts_at,ends_at,qualification_to,claim_deadline,risk_hold_hours,pay_threshold,usage_threshold,max_enrollments,budget_total,reward_mode,signing_secret) VALUES($1,'ranks','running',$2,$3,$2,$3,$3,$3::timestamptz+INTERVAL '1 hour',168,0,0,100,100,'additive',$4) RETURNING id`, []any{fmt.Sprintf("ops-campaign-rank-%d", now.UnixNano()), now.Add(-time.Hour), now.Add(time.Hour), []byte("fixture")}, &campaign))
	var lastInvitee int64
	for i := 0; i < 12; i++ {
		inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("campaign-rank-%d-%d@secret.example", now.UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("campaign-invite-%d-%d@example.com", now.UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
		_, err := client.ExecContext(ctx, `INSERT INTO referral_campaign_attributions(campaign_id,inviter_id,invitee_id,token_nonce,registered_at,status,rules_version,legacy_rebate_policy,qualification_to_snapshot,pay_threshold_snapshot,usage_threshold_snapshot,risk_hold_hours_snapshot) VALUES($1,$2,$3,$4,$5,'approved',1,'stack',$6,0,0,168)`, campaign, inviter.ID, invitee.ID, fmt.Sprintf("n-%d", i), now, now.Add(time.Hour))
		require.NoError(t, err)
		_, err = client.ExecContext(ctx, `INSERT INTO referral_campaign_qualifications(campaign_id,inviter_id,invitee_id,net_paid,actual_cost,risk_status,status,qualified_at) VALUES($1,$2,$3,$4,0,'approved','qualified',$5)`, campaign, inviter.ID, invitee.ID, 100-i, now.Add(time.Duration(i)*time.Second))
		require.NoError(t, err)
		current = inviter.ID
		lastInvitee = invitee.ID
	}
	q, err := service.NormalizeAffiliateOperationsQuery(service.AffiliateOperationsQuery{UserID: current, CampaignID: campaign}, now)
	require.NoError(t, err)
	board, err := r.GetAffiliateOperationsLeaderboard(ctx, q)
	require.NoError(t, err)
	require.Len(t, board.Items, 10)
	require.NotNil(t, board.MyRank)
	require.EqualValues(t, 12, board.MyRank.Rank)
	progress, err := r.GetReferralCampaignProgress(ctx, campaign, current)
	require.NoError(t, err)
	require.Len(t, progress.Leaderboard, 10)
	require.NotNil(t, progress.Ranking)
	require.Equal(t, 12, progress.Ranking.Rank)
	for _, row := range progress.Leaderboard {
		require.Equal(t, "CNY", row.Currency)
		require.NotContains(t, row.EmailMasked, "secret.example")
	}
	_, err = client.ExecContext(ctx, `UPDATE referral_campaign_qualifications SET risk_status='rejected' WHERE campaign_id=$1 AND invitee_id=$2`, campaign, lastInvitee)
	require.NoError(t, err)
	page, err := r.ListReferralCampaignInvitesFiltered(ctx, campaign, 1, 20, "", "qualified", "rejected")
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, "qualified", page.Items[0].Qualification)
	require.Equal(t, "rejected", page.Items[0].RiskStatus)
	board, err = r.GetAffiliateOperationsLeaderboard(ctx, q)
	require.NoError(t, err)
	require.Nil(t, board.MyRank)
}
