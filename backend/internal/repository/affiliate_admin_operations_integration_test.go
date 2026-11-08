//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAffiliateAdminAtomicSettingsRollbackOnDuplicateCode(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	r := &affiliateRepository{client: client}
	owner := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("atomic-owner-%d@example.com", time.Now().UnixNano())})
	other := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("atomic-other-%d@example.com", time.Now().UnixNano())})
	t.Cleanup(func() { _, _ = client.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, owner.ID, other.ID) })
	_, err := r.EnsureUserAffiliate(ctx, owner.ID)
	require.NoError(t, err)
	_, err = r.EnsureUserAffiliate(ctx, other.ID)
	require.NoError(t, err)
	code := fmt.Sprintf("OWNER%d", owner.ID)
	rate := 12.0
	require.NoError(t, r.UpdateAffiliateSettings(ctx, owner.ID, service.AffiliateSettingsChange{Code: &code, Rate: &rate, ActorID: owner.ID}))
	rate = 55
	require.ErrorIs(t, r.UpdateAffiliateSettings(ctx, other.ID, service.AffiliateSettingsChange{Code: &code, Rate: &rate, ActorID: owner.ID}), service.ErrAffiliateCodeTaken)
	var custom bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT aff_rebate_rate_percent IS NOT NULL FROM user_affiliates WHERE user_id=$1`, other.ID).Scan(&custom))
	require.False(t, custom, "duplicate code must not leave a new rate applied")
	require.Equal(t, 1, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM affiliate_operations_audit_logs WHERE user_id=$1 AND action='settings_changed'`, owner.ID))
	require.NoError(t, r.UpdateAffiliateSettings(ctx, owner.ID, service.AffiliateSettingsChange{Reset: true, ActorID: owner.ID}))
	var reset bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT NOT aff_code_custom AND aff_rebate_rate_percent IS NULL AND aff_code<>$2 FROM user_affiliates WHERE user_id=$1`, owner.ID, code).Scan(&reset))
	require.True(t, reset)
}

func TestAffiliateAdminWithdrawalEvidenceIsAtomicAndReplayBound(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	r := &affiliateRepository{client: client}
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("withdraw-proof-%d@example.com", time.Now().UnixNano())})
	t.Cleanup(func() { _, _ = client.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, u.ID) })
	_, err := r.EnsureUserAffiliate(ctx, u.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `UPDATE user_affiliates SET aff_quota=50,aff_history_quota=50 WHERE user_id=$1`, u.ID)
	require.NoError(t, err)
	e := service.AffiliateWithdrawalEvidence{ExternalReference: "bank-ref", PaidAt: "2026-01-01T00:00:00Z", Note: "Paid outside site", ActorID: u.ID}
	op := fmt.Sprintf("proof-%d", u.ID)
	first, err := r.WithdrawQuotaWithEvidence(ctx, u.ID, 10, op, e)
	require.NoError(t, err)
	require.Equal(t, 40.0, first.AvailableQuotaAfter)
	replay, err := r.WithdrawQuotaWithEvidence(ctx, u.ID, 10, op, e)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.LedgerID, replay.LedgerID)
	e.Note = "changed proof"
	_, err = r.WithdrawQuotaWithEvidence(ctx, u.ID, 10, op, e)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	e.ActorID = 9223372036854775807
	_, err = r.WithdrawQuotaWithEvidence(ctx, u.ID, 10, op+"-audit-failure", e)
	require.Error(t, err, "invalid audit actor must roll back debit and proof")
	require.Equal(t, 40.0, querySingleFloat(t, ctx, client, `SELECT aff_quota::float8 FROM user_affiliates WHERE user_id=$1`, u.ID))
	require.Equal(t, 1, querySingleInt(t, ctx, client, `SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id=$1 AND action='withdraw'`, u.ID))
	items, total, err := r.ListAffiliateTransferRecords(ctx, service.AffiliateRecordFilter{UserID: u.ID, Page: 1, PageSize: 20, Action: "withdraw"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "bank-ref", items[0].ExternalReference)
}

func TestAffiliateAdminReportsReuseExactScopeAndGroupCurrencies(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	client := tx.Client()
	r := &affiliateRepository{client: client}
	u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("report-%d@example.com", time.Now().UnixNano())})
	_, err := r.EnsureUserAffiliate(ctx, u.ID)
	require.NoError(t, err)
	now := time.Now()
	var campaign int64
	require.NoError(t, scanAffiliateRow(ctx, client, `INSERT INTO referral_campaigns(campaign_key,name,status,registration_from,registration_to,starts_at,ends_at,qualification_to,claim_deadline,risk_hold_hours,pay_threshold,usage_threshold,max_enrollments,budget_total,reward_mode,signing_secret) VALUES($1,'test','running',$2,$3,$2,$3,$3,$3::timestamptz+INTERVAL '1 hour',168,0,0,10,100,'additive',$4) RETURNING id`, []any{fmt.Sprintf("report-%d", u.ID), now.Add(-time.Hour), now.Add(time.Hour), []byte("fixture")}, &campaign))
	var reward int64
	require.NoError(t, scanAffiliateRow(ctx, client, `INSERT INTO referral_campaign_rewards(campaign_id,user_id,tier_no,reward_type,amount,currency,status,idempotency_key,claim_deadline) VALUES($1,$2,1,'tier',8,'CNY','available',$3,$4) RETURNING id`, []any{campaign, u.ID, fmt.Sprintf("report-reward-%d", u.ID), now.Add(time.Hour)}, &reward))
	_, err = client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(id,user_id,action,amount,referral_reward_id) VALUES(9007199254740993,$1,'accrue',8,$2),(DEFAULT,$1,'accrue',2,NULL)`, u.ID, reward)
	require.NoError(t, err)
	filter := service.AffiliateRecordFilter{UserID: u.ID, Page: 1, PageSize: 1}
	report, err := r.AffiliateRecordReport(ctx, "rebates", filter, false, u.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, report.Summary.RecordCount)
	require.Nil(t, report.Summary.TotalAmount)
	require.Equal(t, "mixed", report.Summary.Currency)
	require.Len(t, report.Summary.Amounts, 2)
	filter.SourceType = "campaign"
	filter.CampaignID = campaign
	items, total, err := r.ListAffiliateRebateRecords(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "CNY", items[0].Currency)
	export, err := r.AffiliateRecordReport(ctx, "rebates", filter, true, u.ID)
	require.NoError(t, err)
	require.Len(t, export.Rows, 1)
	require.Equal(t, json.Number("9007199254740993"), export.Rows[0]["ledger_id"])
	require.EqualValues(t, 1, export.Summary.RecordCount)
	require.Equal(t, 8.0, *export.Summary.TotalAmount)
	_, err = client.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='revoked' WHERE id=$1`, reward)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount,referral_reward_id) VALUES($1,'revoke',-8,$2)`, u.ID, reward)
	require.NoError(t, err)
	report, err = r.AffiliateRecordReport(ctx, "rebates", filter, false, u.ID)
	require.NoError(t, err)
	require.Equal(t, 0.0, *report.Summary.TotalAmount)
	require.Zero(t, report.Summary.Amounts[0].AvailableAmount)
	filter.Status = "available"
	_, total, err = r.ListAffiliateRebateRecords(ctx, filter)
	require.NoError(t, err)
	require.Zero(t, total, "revoked campaign accrual is not available")
}
