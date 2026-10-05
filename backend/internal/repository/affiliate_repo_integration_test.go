//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func querySingleFloat(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) float64 {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value float64
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func querySingleInt(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) int {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value int
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func TestAffiliateRepository_TransferQuotaToBalance_UsesClaimedQuotaBeforeClear(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-transfer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      5.5,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $3, NOW(), NOW())`, u.ID, affCode, 12.34)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.34, transferred, 1e-9)
	require.InDelta(t, 17.84, balance, 1e-9)

	affQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID)
	require.InDelta(t, 0.0, affQuota, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 17.84, persistedBalance, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'transfer'", u.ID)
	require.Equal(t, 1, ledgerCount)

	rows, err := client.QueryContext(txCtx, `
SELECT amount::double precision,
       balance_after::double precision,
       aff_quota_after::double precision,
       aff_frozen_quota_after::double precision,
       aff_history_quota_after::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND action = 'transfer'
LIMIT 1`, u.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "expected transfer ledger")
	var amount, balanceAfter, quotaAfter, frozenAfter, historyAfter float64
	require.NoError(t, rows.Scan(&amount, &balanceAfter, &quotaAfter, &frozenAfter, &historyAfter))
	require.InDelta(t, 12.34, amount, 1e-9)
	require.InDelta(t, 17.84, balanceAfter, 1e-9)
	require.InDelta(t, 0.0, quotaAfter, 1e-9)
	require.InDelta(t, 0.0, frozenAfter, 1e-9)
	require.InDelta(t, 12.34, historyAfter, 1e-9)
}

func TestAffiliateRepository_TransferQuotaToBalanceWithLedger_WritesUnifiedLedger(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)
	ledgerRepo, ok := repo.(service.AffiliateBalanceLedgerTransferRepository)
	require.True(t, ok)
	ledgerSvc := service.NewBalanceLedgerService(integrationDB, nil, nil)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-ledger-transfer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      2.0,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFL%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $3, NOW(), NOW())`, u.ID, affCode, 6.25)
	require.NoError(t, err)

	transferred, balance, err := ledgerRepo.TransferQuotaToBalanceWithLedger(txCtx, u.ID, ledgerSvc)
	require.NoError(t, err)
	require.InDelta(t, 6.25, transferred, 1e-9)
	require.InDelta(t, 8.25, balance, 1e-9)

	var (
		balanceDelta   float64
		balanceBefore  float64
		balanceAfter   float64
		sourceID       string
		idempotencyKey string
		legacyAfter    float64
	)
	rows, err := client.QueryContext(txCtx, `
SELECT bt.balance_delta::double precision,
       bt.balance_before::double precision,
       bt.balance_after::double precision,
       bt.source_id,
       bt.idempotency_key,
       ual.balance_after::double precision
FROM balance_transactions bt
JOIN user_affiliate_ledger ual ON ual.id::text = bt.source_id
WHERE bt.user_id = $1
  AND bt.source_type = 'affiliate_balance'
  AND ual.action = 'transfer'
	LIMIT 1`, u.ID)
	require.NoError(t, err)
	require.True(t, rows.Next(), "expected affiliate balance transaction")
	require.NoError(t, rows.Scan(&balanceDelta, &balanceBefore, &balanceAfter, &sourceID, &idempotencyKey, &legacyAfter))
	require.InDelta(t, 6.25, balanceDelta, 1e-9)
	require.InDelta(t, 2.0, balanceBefore, 1e-9)
	require.InDelta(t, 8.25, balanceAfter, 1e-9)
	require.Equal(t, "affiliate_transfer:"+sourceID, idempotencyKey)
	require.InDelta(t, 8.25, legacyAfter, 1e-9)
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	affQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID)
	require.InDelta(t, 0.0, affQuota, 1e-9)
	totalRecharged := querySingleFloat(t, txCtx, client,
		"SELECT total_recharged::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 6.25, totalRecharged, 1e-9)
}

// TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction guards the
// cross-layer tx propagation invariant: when AccrueQuota is called with a ctx
// that already carries a transaction (via dbent.NewTxContext), repo.withTx
// must reuse that tx rather than opening a nested one. If this invariant
// breaks, AccrueQuota would commit independently and survive a rollback of
// the outer tx, which would violate payment_fulfillment's all-or-nothing
// semantics.
func TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction(t *testing.T) {
	ctx := context.Background()

	outerTx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err, "begin outer tx")
	// Defensive cleanup: if any require.* below fires before the explicit
	// Rollback, this prevents the tx from leaking until container teardown.
	// Rollback is idempotent at the driver level (extra rollback returns an
	// error we ignore).
	t.Cleanup(func() { _ = outerTx.Rollback() })
	client := outerTx.Client()
	txCtx := dbent.NewTxContext(ctx, outerTx)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	repo := NewAffiliateRepository(client, integrationDB)
	_, err = repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound, "invitee must bind to inviter")

	applied, err := repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 3.5, 0, nil)
	require.NoError(t, err)
	require.True(t, applied, "AccrueQuota must report applied=true")

	// Visible inside the outer tx.
	innerQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 3.5, innerQuota, 1e-9)

	// Roll back the outer tx; if AccrueQuota had opened its own inner tx and
	// committed it, the rows would still be visible to the global client.
	require.NoError(t, outerTx.Rollback())

	rows, err := integrationEntClient.QueryContext(ctx,
		"SELECT COUNT(*) FROM user_affiliates WHERE user_id IN ($1, $2)",
		inviter.ID, invitee.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var postRollbackCount int
	require.NoError(t, rows.Scan(&postRollbackCount))
	require.Equal(t, 0, postRollbackCount,
		"AccrueQuota must propagate the outer tx — found persisted rows after rollback")
}

func TestAffiliateRepository_TransferQuotaToBalance_EmptyQuota(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-empty-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      3.21,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, 0, 0, NOW(), NOW())`, u.ID, affCode)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaEmpty)
	require.InDelta(t, 0.0, transferred, 1e-9)
	require.InDelta(t, 0.0, balance, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 3.21, persistedBalance, 1e-9)
}

// TestAffiliateRepository_AdminCustomCode covers the success path of admin
// invite-code rewrite + reset within a shared test transaction:
// - UpdateUserAffCode replaces aff_code, sets aff_code_custom=true, lookup works
// - the old code can no longer be found
// - ResetUserAffCode reverts aff_code_custom and assigns a new system-format code
//
// The conflict path (duplicate code → ErrAffiliateCodeTaken) lives in its own
// test because a unique-violation aborts the surrounding Postgres tx, which
// would poison subsequent assertions in the same transaction.
func TestAffiliateRepository_AdminCustomCode(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-custom-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	original, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.False(t, original.AffCodeCustom, "system-generated codes start as non-custom")
	originalCode := original.AffCode

	// Rewrite to a custom code
	customCode := fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, u.ID, customCode))

	updated, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, customCode, updated.AffCode)
	require.True(t, updated.AffCodeCustom)

	// Lookup by new custom code finds the user
	byCode, err := repo.GetAffiliateByCode(txCtx, customCode)
	require.NoError(t, err)
	require.Equal(t, u.ID, byCode.UserID)

	// Old system code should no longer match
	_, err = repo.GetAffiliateByCode(txCtx, originalCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)

	// Reset back to a fresh system code, clears custom flag
	newSysCode, err := repo.ResetUserAffCode(txCtx, u.ID)
	require.NoError(t, err)
	require.NotEqual(t, customCode, newSysCode)

	reset, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, newSysCode, reset.AffCode)
	require.False(t, reset.AffCodeCustom)

	// The old custom code is now free again
	_, err = repo.GetAffiliateByCode(txCtx, customCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)
}

// TestAffiliateRepository_AdminCustomCode_Conflict isolates the unique-violation
// path. PostgreSQL aborts the enclosing tx when a unique constraint fires, so
// this test must be the only assertion and run in its own tx — production
// callers each have their own outer tx, so this matches real behavior.
func TestAffiliateRepository_AdminCustomCode_Conflict(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	taker := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-taker-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	requester := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-req-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})

	takenCode := fmt.Sprintf("HOT%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, taker.ID, takenCode))

	// Now requester tries to grab the same code → conflict.
	err := repo.UpdateUserAffCode(txCtx, requester.ID, takenCode)
	require.ErrorIs(t, err, service.ErrAffiliateCodeTaken)
}

// TestAffiliateRepository_AdminRebateRate covers per-user exclusive rate
// set/clear and the Batch variant including NULL semantics.
func TestAffiliateRepository_AdminRebateRate(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u1 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-a@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})
	u2 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-b@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	// Set exclusive rate for u1
	rate := 42.5
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, &rate))

	got, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRatePercent)
	require.InDelta(t, 42.5, *got.AffRebateRatePercent, 1e-9)

	// Clear exclusive rate
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, nil))
	cleared, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.AffRebateRatePercent)

	// Batch set both users
	batchRate := 15.0
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, &batchRate))

	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.NotNil(t, v.AffRebateRatePercent)
		require.InDelta(t, 15.0, *v.AffRebateRatePercent, 1e-9)
	}

	// Batch clear
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, nil))
	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.Nil(t, v.AffRebateRatePercent)
	}
}

// TestAffiliateRepository_ListUsersWithCustomSettings verifies the admin list
// only includes users with at least one override applied.
func TestAffiliateRepository_ListUsersWithCustomSettings(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	// User without any custom config — should NOT appear in the list.
	plainEmail := fmt.Sprintf("affiliate-plain-%d@example.com", time.Now().UnixNano())
	uPlain := mustCreateUser(t, client, &service.User{
		Email: plainEmail, PasswordHash: "hash",
		Role: service.RoleUser, Status: service.StatusActive,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, uPlain.ID)
	require.NoError(t, err)

	// User with a custom code — should appear.
	uCode := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-codeonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	require.NoError(t, repo.UpdateUserAffCode(txCtx, uCode.ID, fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)))

	// User with only an exclusive rate — should appear.
	uRate := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rateonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	r := 33.3
	require.NoError(t, repo.SetUserRebateRate(txCtx, uRate.ID, &r))

	entries, total, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{
		Page: 1, PageSize: 100,
	})
	require.NoError(t, err)

	// Build a quick lookup to assert per-user attributes (other tests may have
	// inserted custom rows in the same DB; we only care about our 3).
	byUserID := make(map[int64]service.AffiliateAdminEntry, len(entries))
	for _, e := range entries {
		byUserID[e.UserID] = e
	}

	require.NotContains(t, byUserID, uPlain.ID, "users without overrides must not appear")

	codeEntry, ok := byUserID[uCode.ID]
	require.True(t, ok, "custom-code user missing from list")
	require.True(t, codeEntry.AffCodeCustom)
	require.Nil(t, codeEntry.AffRebateRatePercent)

	rateEntry, ok := byUserID[uRate.ID]
	require.True(t, ok, "custom-rate user missing from list")
	require.False(t, rateEntry.AffCodeCustom)
	require.NotNil(t, rateEntry.AffRebateRatePercent)
	require.InDelta(t, 33.3, *rateEntry.AffRebateRatePercent, 1e-9)

	require.GreaterOrEqual(t, total, int64(2), "total must include at least our 2 custom rows")
}

func TestAffiliateRepository_CreateReferralCampaignWritesCreatedAuditLog(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)
	campaignRepo, ok := repo.(service.ReferralCampaignRepository)
	require.True(t, ok, "affiliate repository must implement referral campaign persistence")
	creator := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("campaign-creator-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
	})

	startsAt := time.Now().UTC().Add(24 * time.Hour)
	campaign, err := campaignRepo.CreateReferralCampaign(ctx, service.ReferralCampaign{
		Key:                fmt.Sprintf("audit-campaign-%d", time.Now().UnixNano()),
		Name:               "Audit parameter type regression",
		RegistrationFrom:   startsAt,
		RegistrationTo:     startsAt.Add(24 * time.Hour),
		StartsAt:           startsAt,
		EndsAt:             startsAt.Add(7 * 24 * time.Hour),
		QualificationTo:    startsAt.Add(8 * 24 * time.Hour),
		ClaimDeadline:      startsAt.Add(14 * 24 * time.Hour),
		RiskHoldHours:      168,
		PayThreshold:       10,
		UsageThreshold:     1,
		MaxEnrollments:     100,
		BudgetTotal:        1000,
		RewardMode:         "additive",
		LegacyRebatePolicy: "exclude",
		SigningSecret:      []byte("test-signing-secret"),
		CreatedBy:          creator.ID,
	}, []service.ReferralCampaignTier{{
		Tier: 1, RequiredInvites: 1, RewardAmount: 10, Currency: "CNY",
	}})
	require.NoError(t, err)
	require.NotZero(t, campaign.ID)

	var auditCampaignKey string
	rows, err := client.QueryContext(ctx, `
SELECT detail->>'campaign_key'
FROM referral_campaign_audit_logs
WHERE campaign_id=$1 AND action='created'`, campaign.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&auditCampaignKey))
	require.NoError(t, rows.Err())
	require.Equal(t, campaign.Key, auditCampaignKey)
}

func TestAffiliateRepository_EarlyCloseReferralCampaignExecutesTypedAuditAndPreservesProcessedRewards(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)
	campaignRepo, ok := repo.(service.ReferralCampaignRepository)
	require.True(t, ok)

	actor := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("early-close-actor-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleAdmin, Status: service.StatusActive})
	recipient := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("early-close-recipient-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive})
	now := time.Now().UTC()
	var campaignID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO referral_campaigns (
			campaign_key,name,status,version,registration_from,registration_to,starts_at,ends_at,qualification_to,claim_deadline,
			risk_hold_hours,pay_threshold,usage_threshold,max_enrollments,budget_total,budget_reserved,budget_paid,reward_mode,signing_secret,created_by
		) VALUES ($1,$2,'settling',7,$3,$4,$5,$6,$7,$8,168,0,0,1,100,15,0,'additive',$9,$10)
		RETURNING id`,
		fmt.Sprintf("early-close-%d", time.Now().UnixNano()), "Typed early close", now.Add(-96*time.Hour), now.Add(-72*time.Hour), now.Add(-72*time.Hour), now.Add(-48*time.Hour), now.Add(-24*time.Hour), now.Add(time.Hour), []byte("test-secret"), actor.ID,
	).Scan(&campaignID))

	var claimableID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO referral_campaign_rewards (campaign_id,user_id,tier_no,reward_type,amount,status,idempotency_key,claim_deadline)
		VALUES ($1,$2,1,'invite',5,'claimable',$3,$4) RETURNING id`, campaignID, recipient.ID, fmt.Sprintf("early-close-claimable-%d", time.Now().UnixNano()), now.Add(time.Hour),
	).Scan(&claimableID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO referral_campaign_rewards (campaign_id,user_id,tier_no,reward_type,amount,status,idempotency_key,claim_deadline,frozen_until)
		VALUES ($1,$2,2,'invite',10,'claimed_frozen',$3,$4,$5) RETURNING id`, campaignID, recipient.ID, fmt.Sprintf("early-close-preserved-%d", time.Now().UnixNano()), now.Add(time.Hour), now.Add(24*time.Hour),
	).Scan(new(int64)))

	closed, err := campaignRepo.EarlyCloseReferralCampaign(ctx, campaignID, 7, actor.ID, "operations reviewed")
	require.NoError(t, err)
	require.Equal(t, service.ReferralCampaignStatusClosed, closed.Status)
	require.Equal(t, int64(8), closed.Version)

	var claimableStatus, processedStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM referral_campaign_rewards WHERE id=$1`, claimableID).Scan(&claimableStatus))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM referral_campaign_rewards WHERE campaign_id=$1 AND status='claimed_frozen'`, campaignID).Scan(&processedStatus))
	require.Equal(t, service.ReferralRewardStatusExpired, claimableStatus)
	require.Equal(t, service.ReferralRewardStatusClaimedFrozen, processedStatus)

	var reserved, released float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT budget_reserved::double precision FROM referral_campaigns WHERE id=$1`, campaignID).Scan(&reserved))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount),0)::double precision FROM referral_campaign_budget_ledger WHERE campaign_id=$1 AND action='release'`, campaignID).Scan(&released))
	require.InDelta(t, 10.0, reserved, 1e-9)
	require.InDelta(t, 5.0, released, 1e-9)

	var auditCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM referral_campaign_audit_logs WHERE campaign_id=$1 AND action='early_closed' AND detail->>'expired_reward_count'='1'`, campaignID).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
}

func TestAffiliateRepository_EarlyCloseReferralCampaignRollsBackWhenAuditWriteFails(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)
	campaignRepo, ok := repo.(service.ReferralCampaignRepository)
	require.True(t, ok)

	// The failure occurs after rewards, budget, and campaign rows would have
	// changed, so it verifies that the repository transaction is all-or-nothing.
	_, execErr := integrationDB.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION test_referral_early_close_audit_failure()
		RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.action = 'early_closed' THEN
				RAISE EXCEPTION 'forced early-close audit failure';
			END IF;
			RETURN NEW;
		END;
		$$`)
	require.NoError(t, execErr)
	_, execErr = integrationDB.ExecContext(ctx, `
		CREATE TRIGGER test_referral_early_close_audit_failure
		BEFORE INSERT ON referral_campaign_audit_logs
		FOR EACH ROW EXECUTE FUNCTION test_referral_early_close_audit_failure()`)
	require.NoError(t, execErr)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS test_referral_early_close_audit_failure ON referral_campaign_audit_logs`)
		_, _ = integrationDB.ExecContext(context.Background(), `DROP FUNCTION IF EXISTS test_referral_early_close_audit_failure()`)
	})

	actor := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("early-close-rollback-actor-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleAdmin, Status: service.StatusActive})
	recipient := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("early-close-rollback-recipient-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive})
	now := time.Now().UTC()
	var campaignID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO referral_campaigns (
			campaign_key,name,status,version,registration_from,registration_to,starts_at,ends_at,qualification_to,claim_deadline,
			risk_hold_hours,pay_threshold,usage_threshold,max_enrollments,budget_total,budget_reserved,budget_paid,reward_mode,signing_secret,created_by
		) VALUES ($1,$2,'settling',7,$3,$4,$5,$6,$7,$8,168,0,0,1,100,5,0,'additive',$9,$10)
		RETURNING id`,
		fmt.Sprintf("early-close-rollback-%d", time.Now().UnixNano()), "Early close rollback", now.Add(-96*time.Hour), now.Add(-72*time.Hour), now.Add(-72*time.Hour), now.Add(-48*time.Hour), now.Add(-24*time.Hour), now.Add(time.Hour), []byte("test-secret"), actor.ID,
	).Scan(&campaignID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO referral_campaign_rewards (campaign_id,user_id,tier_no,reward_type,amount,status,idempotency_key,claim_deadline)
		VALUES ($1,$2,1,'invite',5,'claimable',$3,$4) RETURNING id`, campaignID, recipient.ID, fmt.Sprintf("early-close-rollback-reward-%d", time.Now().UnixNano()), now.Add(time.Hour),
	).Scan(new(int64)))

	_, err := campaignRepo.EarlyCloseReferralCampaign(ctx, campaignID, 7, actor.ID, "operations reviewed")
	require.Error(t, err)
	require.ErrorContains(t, err, "forced early-close audit failure")

	var status string
	var version int64
	var reserved float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status,version,budget_reserved::double precision FROM referral_campaigns WHERE id=$1`, campaignID).Scan(&status, &version, &reserved))
	require.Equal(t, service.ReferralCampaignStatusSettling, status)
	require.Equal(t, int64(7), version)
	require.InDelta(t, 5.0, reserved, 1e-9)
	var rewardStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM referral_campaign_rewards WHERE campaign_id=$1`, campaignID).Scan(&rewardStatus))
	require.Equal(t, service.ReferralRewardStatusClaimable, rewardStatus)
	var releases, audits int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM referral_campaign_budget_ledger WHERE campaign_id=$1 AND action='release'`, campaignID).Scan(&releases))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM referral_campaign_audit_logs WHERE campaign_id=$1 AND action='early_closed'`, campaignID).Scan(&audits))
	require.Zero(t, releases)
	require.Zero(t, audits)
}

func mustCreateAffiliateWithdrawUser(t *testing.T, client *dbent.Client, label string) *service.User {
	t.Helper()
	return mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-withdraw-%s-%d@example.com", label, time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      5.5,
		Concurrency:  5,
	})
}

func mustSeedAffiliateQuota(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, quota, frozen, history float64) {
	t.Helper()
	affCode := fmt.Sprintf("AFW%d-%06d", userID, time.Now().UnixNano()%1_000_000)
	_, err := client.ExecContext(ctx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_frozen_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())`, userID, affCode, quota, frozen, history)
	require.NoError(t, err)
}

// mustCreateCommittedAffiliateWithdrawUser 在测试事务之外建用户与返利档案，
// 测试结束时删除（级联返利档案与流水）。WithdrawQuota 在这些用例里自行开启
// 并提交事务，与生产调用一致。
func mustCreateCommittedAffiliateWithdrawUser(t *testing.T, label string, quota, frozen, history float64) *service.User {
	t.Helper()
	client := testEntClient(t)
	u := mustCreateAffiliateWithdrawUser(t, client, label)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
	})
	mustSeedAffiliateQuota(t, context.Background(), client, u.ID, quota, frozen, history)
	return u
}

func newAffiliateWithdrawOperationID(label string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", label, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])
}

// TestAffiliateRepository_WithdrawQuota_DeductsAndRecordsLedger 覆盖线下提现主路径：
// 只扣可提取额度，余额与累计返利不变，流水 action=withdraw 带额度快照，并出现在提取记录中。
func TestAffiliateRepository_WithdrawQuota_DeductsAndRecordsLedger(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateAffiliateWithdrawUser(t, client, "user")
	mustSeedAffiliateQuota(t, txCtx, client, u.ID, 20, 0, 30)
	operationID := newAffiliateWithdrawOperationID("deduct")

	result, err := repo.WithdrawQuota(txCtx, u.ID, 12.5, operationID)
	require.NoError(t, err)
	require.Positive(t, result.LedgerID)
	require.False(t, result.Replayed)
	require.InDelta(t, 12.5, result.Amount, 1e-9)
	require.InDelta(t, 7.5, result.AvailableQuotaAfter, 1e-9)
	require.InDelta(t, 0.0, result.FrozenQuotaAfter, 1e-9)
	require.InDelta(t, 30.0, result.HistoryQuotaAfter, 1e-9)

	require.InDelta(t, 7.5, querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.InDelta(t, 30.0, querySingleFloat(t, txCtx, client,
		"SELECT aff_history_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.InDelta(t, 5.5, querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID), 1e-9)

	records, total, err := repo.ListAffiliateTransferRecords(txCtx, service.AffiliateRecordFilter{
		Search:   u.Email,
		Page:     1,
		PageSize: 20,
		SortDesc: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	record := records[0]
	require.Equal(t, result.LedgerID, record.LedgerID)
	require.Equal(t, "withdraw", record.Action)
	require.Equal(t, u.ID, record.UserID)
	require.InDelta(t, 12.5, record.Amount, 1e-9)
	require.True(t, record.SnapshotAvailable)
	require.InDelta(t, 5.5, *record.BalanceAfter, 1e-9)
	require.InDelta(t, 7.5, *record.AvailableQuotaAfter, 1e-9)
	require.InDelta(t, 30.0, *record.HistoryQuotaAfter, 1e-9)

	rows, err := client.QueryContext(txCtx, "SELECT operation_id FROM user_affiliate_ledger WHERE id = $1", result.LedgerID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var storedOperationID string
	require.NoError(t, rows.Scan(&storedOperationID))
	require.Equal(t, operationID, storedOperationID)
}

// TestAffiliateRepository_WithdrawQuota_ThawsMaturedQuotaFirst 验证已过冻结期
// 但尚未解冻的额度在扣减前解冻并可被提现。
func TestAffiliateRepository_WithdrawQuota_ThawsMaturedQuotaFirst(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateAffiliateWithdrawUser(t, client, "thaw")
	mustSeedAffiliateQuota(t, txCtx, client, u.ID, 0, 10, 10)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, frozen_until, created_at, updated_at)
VALUES ($1, 'accrue', 10, NOW() - INTERVAL '1 hour', NOW(), NOW())`, u.ID)
	require.NoError(t, err)

	result, err := repo.WithdrawQuota(txCtx, u.ID, 10, newAffiliateWithdrawOperationID("thaw"))
	require.NoError(t, err)
	require.InDelta(t, 0.0, result.AvailableQuotaAfter, 1e-9)
	require.InDelta(t, 0.0, result.FrozenQuotaAfter, 1e-9)
	require.Equal(t, 1, querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'withdraw'", u.ID))
}

// TestAffiliateRepository_WithdrawQuota_InsufficientLeavesQuotaUntouched 验证未到期的
// 冻结额度不可提现：额度不足时不扣减、不写流水，占位随事务回滚、不占用该
// operation_id，同一标识改成可提取范围内的金额仍可登记；没有返利档案的用户
// 同样按额度不足处理。
func TestAffiliateRepository_WithdrawQuota_InsufficientLeavesQuotaUntouched(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateCommittedAffiliateWithdrawUser(t, "insufficient", 5, 10, 15)
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, frozen_until, created_at, updated_at)
VALUES ($1, 'accrue', 10, NOW() + INTERVAL '1 hour', NOW(), NOW())`, u.ID)
	require.NoError(t, err)
	operationID := newAffiliateWithdrawOperationID("insufficient")

	_, err = repo.WithdrawQuota(ctx, u.ID, 6, operationID)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaInsufficient)

	require.InDelta(t, 5.0, querySingleFloat(t, ctx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.InDelta(t, 10.0, querySingleFloat(t, ctx, client,
		"SELECT aff_frozen_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.Equal(t, 0, querySingleInt(t, ctx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'withdraw'", u.ID))

	result, err := repo.WithdrawQuota(ctx, u.ID, 5, operationID)
	require.NoError(t, err)
	require.False(t, result.Replayed)
	require.InDelta(t, 0.0, result.AvailableQuotaAfter, 1e-9)

	noProfile := mustCreateAffiliateWithdrawUser(t, client, "no-profile")
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", noProfile.ID)
	})
	_, err = repo.WithdrawQuota(ctx, noProfile.ID, 1, newAffiliateWithdrawOperationID("no-profile"))
	require.ErrorIs(t, err, service.ErrAffiliateQuotaInsufficient)
	require.Equal(t, 0, querySingleInt(t, ctx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1", noProfile.ID))
}

// TestAffiliateRepository_WithdrawQuota_RetryAfterCommitReplaysFirstResult 复现
// 「提交成功但响应丢失后重试」：首次登记已在独立事务中提交，以同一 operation_id
// 重试不重复扣减、不重复写流水，返回首次登记的流水与额度快照。两笔合法的等额打款
// 使用不同的 operation_id，各扣一次。
func TestAffiliateRepository_WithdrawQuota_RetryAfterCommitReplaysFirstResult(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateCommittedAffiliateWithdrawUser(t, "retry", 100, 0, 100)
	operationID := newAffiliateWithdrawOperationID("retry")

	first, err := repo.WithdrawQuota(ctx, u.ID, 10, operationID)
	require.NoError(t, err)
	require.False(t, first.Replayed)

	retry, err := repo.WithdrawQuota(ctx, u.ID, 10, operationID)
	require.NoError(t, err)
	require.True(t, retry.Replayed)
	require.Equal(t, first.LedgerID, retry.LedgerID)
	require.Equal(t, u.ID, retry.UserID)
	require.InDelta(t, 10.0, retry.Amount, 1e-9)
	require.InDelta(t, 90.0, retry.AvailableQuotaAfter, 1e-9)
	require.InDelta(t, first.FrozenQuotaAfter, retry.FrozenQuotaAfter, 1e-9)
	require.InDelta(t, 100.0, retry.HistoryQuotaAfter, 1e-9)

	require.InDelta(t, 90.0, querySingleFloat(t, ctx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.Equal(t, 1, querySingleInt(t, ctx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'withdraw'", u.ID))

	second, err := repo.WithdrawQuota(ctx, u.ID, 10, newAffiliateWithdrawOperationID("retry-second"))
	require.NoError(t, err)
	require.False(t, second.Replayed)
	require.NotEqual(t, first.LedgerID, second.LedgerID)
	require.InDelta(t, 80.0, querySingleFloat(t, ctx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.Equal(t, 2, querySingleInt(t, ctx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'withdraw'", u.ID))
}

// TestAffiliateRepository_WithdrawQuota_SameOperationWaitsForInFlightRegistration
// 固定「并发同键」的交错：首个登记已占位并扣减但未提交时，同一 operation_id 的
// 请求在唯一索引上等待。首个登记提交后，等待的请求不重复扣减，返回同一条流水；
// 首个登记回滚后，等待的请求自行完成登记。两种结局都只扣一次。
func TestAffiliateRepository_WithdrawQuota_SameOperationWaitsForInFlightRegistration(t *testing.T) {
	for _, commitFirst := range []bool{true, false} {
		name := "first rolls back"
		if commitFirst {
			name = "first commits"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			client := testEntClient(t)
			repo := NewAffiliateRepository(client, integrationDB)

			u := mustCreateCommittedAffiliateWithdrawUser(t, "inflight", 100, 0, 100)
			operationID := newAffiliateWithdrawOperationID("inflight")

			tx, err := client.Tx(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { _ = tx.Rollback() })
			first, err := repo.WithdrawQuota(dbent.NewTxContext(ctx, tx), u.ID, 10, operationID)
			require.NoError(t, err)

			type outcome struct {
				result *service.AffiliateWithdrawResult
				err    error
			}
			waiting := make(chan outcome, 1)
			go func() {
				result, err := repo.WithdrawQuota(ctx, u.ID, 10, operationID)
				waiting <- outcome{result: result, err: err}
			}()

			select {
			case got := <-waiting:
				t.Fatalf("same operation must wait for the in-flight registration, got result=%+v err=%v", got.result, got.err)
			case <-time.After(300 * time.Millisecond):
			}

			if commitFirst {
				require.NoError(t, tx.Commit())
			} else {
				require.NoError(t, tx.Rollback())
			}

			var got outcome
			select {
			case got = <-waiting:
			case <-time.After(10 * time.Second):
				t.Fatal("waiting registration did not finish after the in-flight one ended")
			}
			require.NoError(t, got.err)
			if commitFirst {
				require.True(t, got.result.Replayed)
				require.Equal(t, first.LedgerID, got.result.LedgerID)
			} else {
				require.False(t, got.result.Replayed)
				require.NotEqual(t, first.LedgerID, got.result.LedgerID)
			}
			require.InDelta(t, 90.0, got.result.AvailableQuotaAfter, 1e-9)

			require.InDelta(t, 90.0, querySingleFloat(t, ctx, client,
				"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
			require.Equal(t, 1, querySingleInt(t, ctx, client,
				"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'withdraw'", u.ID))
		})
	}
}

// TestAffiliateRepository_WithdrawQuota_ConcurrentSameOperationDeductsOnce 让多个请求
// 同时以同一 operation_id 登记：只有一个执行扣减，其余返回同一条流水。
func TestAffiliateRepository_WithdrawQuota_ConcurrentSameOperationDeductsOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateCommittedAffiliateWithdrawUser(t, "concurrent", 100, 0, 100)
	operationID := newAffiliateWithdrawOperationID("concurrent")

	const workers = 8
	results := make([]*service.AffiliateWithdrawResult, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = repo.WithdrawQuota(ctx, u.ID, 10, operationID)
		}(i)
	}
	close(start)
	wg.Wait()

	executed := 0
	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, results[0].LedgerID, results[i].LedgerID)
		require.InDelta(t, 90.0, results[i].AvailableQuotaAfter, 1e-9)
		if !results[i].Replayed {
			executed++
		}
	}
	require.Equal(t, 1, executed)
	require.InDelta(t, 90.0, querySingleFloat(t, ctx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.Equal(t, 1, querySingleInt(t, ctx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'withdraw'", u.ID))
}

// TestAffiliateRepository_WithdrawQuota_RejectsOperationReuseWithDifferentRequest 验证
// 同一 operation_id 换了金额或用户时以 ErrIdempotencyKeyConflict 拒绝，
// 不扣减、不写流水。
func TestAffiliateRepository_WithdrawQuota_RejectsOperationReuseWithDifferentRequest(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateCommittedAffiliateWithdrawUser(t, "conflict", 100, 0, 100)
	other := mustCreateCommittedAffiliateWithdrawUser(t, "conflict-other", 100, 0, 100)
	operationID := newAffiliateWithdrawOperationID("conflict")

	_, err := repo.WithdrawQuota(ctx, u.ID, 10, operationID)
	require.NoError(t, err)

	_, err = repo.WithdrawQuota(ctx, u.ID, 11, operationID)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	_, err = repo.WithdrawQuota(ctx, other.ID, 10, operationID)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)

	require.InDelta(t, 90.0, querySingleFloat(t, ctx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID), 1e-9)
	require.InDelta(t, 100.0, querySingleFloat(t, ctx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", other.ID), 1e-9)
	require.Equal(t, 1, querySingleInt(t, ctx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id IN ($1, $2) AND action = 'withdraw'", u.ID, other.ID))
}

// TestAffiliateRepository_ListAffiliateRebateRecords_IncludesNonOrderAccruals 验证兑换码
// 等非订单来源的返利出现在返利记录中，订单字段为空。
func TestAffiliateRepository_ListAffiliateRebateRecords_IncludesNonOrderAccruals(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateAffiliateWithdrawUser(t, client, "inviter")
	invitee := mustCreateAffiliateWithdrawUser(t, client, "invitee")
	mustSeedAffiliateQuota(t, txCtx, client, inviter.ID, 0, 0, 0)

	applied, err := repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 2, 0, nil)
	require.NoError(t, err)
	require.True(t, applied)

	records, total, err := repo.ListAffiliateRebateRecords(txCtx, service.AffiliateRecordFilter{
		Search:   invitee.Email,
		Page:     1,
		PageSize: 20,
		SortDesc: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	record := records[0]
	require.Nil(t, record.OrderID)
	require.Nil(t, record.OrderAmount)
	require.Nil(t, record.PayAmount)
	require.Empty(t, record.OutTradeNo)
	require.Equal(t, inviter.ID, record.InviterID)
	require.NotNil(t, record.InviteeID)
	require.Equal(t, invitee.ID, *record.InviteeID)
	require.InDelta(t, 2.0, record.RebateAmount, 1e-9)
}
