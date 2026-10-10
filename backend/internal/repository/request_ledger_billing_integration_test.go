//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRequestLedgerBillingTransactionDedupAndFailure(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("ledger-%d@example.invalid", time.Now().UnixNano()), PasswordHash: "fixture", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "fixture-ledger-" + uuid.NewString(), Name: "fixture"})
	account := mustCreateAccount(t, client, &service.Account{Name: "ledger-fixture-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	l := requestledger.New(integrationDB)
	repo := NewUsageBillingRepositoryWithLedger(client, integrationDB, service.NewBalanceLedgerService(integrationDB, nil, nil))
	admit := func() (context.Context, *requestledger.Handle) {
		h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
		require.NoError(t, err)
		requestCtx := requestledger.WithHandle(ctx, h)
		require.NoError(t, requestledger.BindIdentity(requestCtx, user.ID, key.ID))
		a, err := requestledger.BeginAttempt(requestCtx, account.ID, account.ID)
		require.NoError(t, err)
		require.NoError(t, a.Finish(ctx, 200, nil))
		require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
		return requestCtx, h
	}
	billed, h := admit()
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), APIKeyID: key.ID, UserID: user.ID, AccountID: account.ID, AccountType: service.AccountTypeAPIKey, BalanceCost: 1.25}
	first, err := repo.Apply(billed, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	again, err := repo.Apply(billed, cmd)
	require.NoError(t, err)
	require.False(t, again.Applied)
	replay, replayHandle := admit()
	duplicate, err := repo.Apply(replay, cmd)
	require.NoError(t, err)
	require.False(t, duplicate.Applied)
	var balance float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, .000001)
	refs, err := l.BillingReferences(ctx, requestledger.Viewer{UserID: user.ID}, h.ID)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.True(t, refs[0].Settled)
	require.True(t, refs[0].Applied)
	require.NotNil(t, refs[0].WalletTransactionID)
	require.Nil(t, refs[0].BilledCost, "unknown usage must not display zero or invented cost")
	refs, err = l.BillingReferences(ctx, requestledger.Viewer{UserID: user.ID}, replayHandle.ID)
	require.NoError(t, err)
	require.False(t, refs[0].Applied)
	failed, failedHandle := admit()
	bad := *cmd
	bad.RequestID = uuid.NewString()
	bad.RequestFingerprint = ""
	bad.APIKeyID = key.ID + 100000000
	_, err = repo.Apply(failed, &bad)
	require.Error(t, err)
	var settlement string
	require.NoError(t, integrationDB.QueryRow(`SELECT settlement_state FROM gateway_requests WHERE id=$1`, failedHandle.ID).Scan(&settlement))
	require.Equal(t, "settlement_pending", settlement)
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, .000001)
	// Inject failure after monetary effects, in the same transaction as verification.
	faultCtx, faultHandle := admit()
	fault := *cmd
	fault.RequestID = uuid.NewString()
	fault.RequestFingerprint = ""
	_, err = integrationDB.Exec(`ALTER TABLE gateway_request_billing_links ADD CONSTRAINT ledger_fixture_fail_verification CHECK (NOT settlement_verified) NOT VALID`)
	require.NoError(t, err)
	_, err = repo.Apply(faultCtx, &fault)
	require.Error(t, err)
	_, dropErr := integrationDB.Exec(`ALTER TABLE gateway_request_billing_links DROP CONSTRAINT ledger_fixture_fail_verification`)
	require.NoError(t, dropErr)
	require.NoError(t, integrationDB.QueryRow(`SELECT settlement_state FROM gateway_requests WHERE id=$1`, faultHandle.ID).Scan(&settlement))
	require.Equal(t, "settlement_pending", settlement)
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, .000001)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, fault.RequestID, key.ID).Scan(&count))
	require.Zero(t, count)
	recovered, err := repo.Apply(faultCtx, &fault)
	require.NoError(t, err)
	require.True(t, recovered.Applied)
	repeated, err := repo.Apply(faultCtx, &fault)
	require.NoError(t, err)
	require.False(t, repeated.Applied)
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 97.5, balance, .000001)

}

func TestRequestLedgerTaskFundingReferencesRecoverWithoutCharging(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("ledger-task-%d@example.invalid", time.Now().UnixNano()), PasswordHash: "fixture", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "fixture-ledger-task-" + uuid.NewString(), Name: "fixture"})
	repo := NewUsageBillingRepositoryWithLedger(client, integrationDB, service.NewBalanceLedgerService(integrationDB, nil, nil))
	ledger := requestledger.New(integrationDB)
	h, err := ledger.Begin(ctx, "/v1/images/batches", "POST", "http")
	require.NoError(t, err)
	requestCtx := requestledger.WithHandle(ctx, h)
	taskID := uuid.NewString()
	require.NoError(t, requestledger.BindTask(requestCtx, "mobile_video", taskID, user.ID, key.ID))
	a, err := requestledger.BeginAttempt(requestCtx, 234, 22)
	require.NoError(t, err)
	require.NoError(t, a.Finish(ctx, 200, nil))
	require.NoError(t, h.Finish(ctx, "succeeded", 202, ""))
	job := &service.MobileVideoJob{TaskID: taskID, UserID: user.ID, ExecutionAPIKeyID: key.ID, HoldAmount: 2.5}
	require.NoError(t, service.ReserveMobileVideoBalance(ctx, repo, job))
	require.NoError(t, ledger.ReconcileTaskBilling(ctx))
	rec, err := ledger.Get(ctx, requestledger.Viewer{UserID: user.ID}, h.ID)
	require.NoError(t, err)
	require.Equal(t, "settlement_pending", rec.SettlementState)
	require.NoError(t, service.CaptureMobileVideoBalance(ctx, repo, job))
	for range 2 {
		require.NoError(t, ledger.ReconcileTaskBilling(ctx))
	}
	rec, err = ledger.Get(ctx, requestledger.Viewer{UserID: user.ID}, h.ID)
	require.NoError(t, err)
	require.Equal(t, "settled", rec.SettlementState)
	require.Equal(t, "usage_unknown", rec.UsageState)
	refs, err := ledger.WalletReferences(ctx, requestledger.Viewer{UserID: user.ID}, h.ID)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	_, err = ledger.WalletReferences(ctx, requestledger.Viewer{UserID: user.ID + 100000}, h.ID)
	require.Error(t, err)
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=$1`, user.ID).Scan(&balance, &frozen))
	require.InDelta(t, 97.5, balance, .000001)
	require.InDelta(t, 0, frozen, .000001)
}
