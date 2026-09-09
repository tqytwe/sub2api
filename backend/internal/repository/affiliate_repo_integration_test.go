//go:build integration

package repository

import (
	"context"
	"fmt"
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
