//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newBillingLogRecoveryFixture(t *testing.T) (*usageLogRepository, *service.UsageLog, *service.UsageBillingCommand) {
	t.Helper()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "reconcile-" + uuid.NewString() + "@example.com", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "test-" + uuid.NewString(), Name: "reconciliation"})
	account := mustCreateAccount(t, client, &service.Account{Name: "reconcile-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), APIKeyID: key.ID, UserID: user.ID, AccountID: account.ID,
		AccountType: service.AccountTypeAPIKey, Model: "test-model", InputTokens: 9, OutputTokens: 2,
		ActualCost: 2, BilledCost: 2.2, BillingSurchargeCost: 0.2, BillingSurchargeMode: "percent_on_charged_cost", BillingSurchargeValue: 0.1,
		BalanceCost: 2.2, RequestPayloadHash: "original-payload"}
	cmd.Normalize()
	stats, multiplier := 4.5, 1.5
	log := &service.UsageLog{RequestID: cmd.RequestID, APIKeyID: key.ID, UserID: user.ID, AccountID: account.ID,
		Model: "test-model", InputTokens: 9, OutputTokens: 2, TotalCost: 3, RateMultiplier: 1,
		AccountStatsCost: &stats, AccountRateMultiplier: &multiplier, BillingRequestFingerprint: cmd.RequestFingerprint,
		BillingSurchargeMode: cmd.BillingSurchargeMode, BillingSurchargeValue: cmd.BillingSurchargeValue,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	return newUsageLogRepositoryWithSQL(client, integrationDB), log, cmd
}

func settleBillingRecoveryLog(t *testing.T, log *service.UsageLog, cmd *service.UsageBillingCommand) *service.UsageLog {
	t.Helper()
	ledger := service.NewBalanceLedgerService(integrationDB, nil, nil)
	result, err := NewUsageBillingRepositoryWithLedger(nil, integrationDB, ledger).Apply(context.Background(), cmd)
	require.NoError(t, err)
	require.True(t, result.SettlementVerified)
	require.Equal(t, cmd.RequestFingerprint, result.SettlementFingerprint)
	settled := *log
	settled.ActualCost, settled.BilledCost, settled.BillingSurchargeCost = cmd.ActualCost, cmd.BilledCost, cmd.BillingSurchargeCost
	settled.BillingSettled = true
	return &settled
}

func assertBillingRecoveryLog(t *testing.T, log *service.UsageLog, settled bool, actual float64) {
	t.Helper()
	var gotActual, billed, surcharge, stats, multiplier float64
	var gotSettled bool
	var model string
	var input int
	var created time.Time
	err := integrationDB.QueryRowContext(context.Background(), `SELECT actual_cost, billed_cost, billing_surcharge_cost,
 account_stats_cost, account_rate_multiplier, billing_settled, model, input_tokens, created_at
 FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, log.RequestID, log.APIKeyID).
		Scan(&gotActual, &billed, &surcharge, &stats, &multiplier, &gotSettled, &model, &input, &created)
	require.NoError(t, err)
	require.Equal(t, actual, gotActual)
	require.Equal(t, settled, gotSettled)
	require.Equal(t, 4.5, stats)
	require.Equal(t, 1.5, multiplier)
	require.Equal(t, "test-model", model)
	require.Equal(t, 9, input)
	require.True(t, log.CreatedAt.Equal(created))
	if settled && actual > 0 {
		require.Equal(t, 2.2, billed)
		require.Equal(t, 0.2, surcharge)
	}
}

func TestBillingLogReconciliationInsertRepairAndStaleFailure(t *testing.T) {
	ctx := context.Background()
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint("batch=", batch), func(t *testing.T) {
			repo, failed, cmd := newBillingLogRecoveryFixture(t)
			create := func(log *service.UsageLog) (bool, error) {
				if batch {
					return repo.Create(ctx, log)
				}
				return repo.createSingle(ctx, integrationDB, log)
			}
			inserted, err := create(failed)
			require.NoError(t, err)
			require.True(t, inserted)
			settled := settleBillingRecoveryLog(t, failed, cmd)
			settled.Model, settled.InputTokens = "must-not-replace", 99
			inserted, err = create(settled)
			require.NoError(t, err)
			require.False(t, inserted, "a repair must not report a new insert")
			_ = settleBillingRecoveryLog(t, failed, cmd)
			inserted, err = create(settled)
			require.NoError(t, err)
			require.False(t, inserted)
			_, err = create(failed)
			require.NoError(t, err)
			assertBillingRecoveryLog(t, failed, true, 2)
			var balance float64
			require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", failed.UserID).Scan(&balance))
			require.Equal(t, 97.8, balance)
			var ledgerCount int
			require.NoError(t, integrationDB.QueryRow("SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1 AND idempotency_key=$2", failed.UserID, usageBillingBalanceLedgerKey(cmd.APIKeyID, cmd.RequestID)).Scan(&ledgerCount))
			require.Equal(t, 1, ledgerCount)
		})
	}
}

func TestBillingLogReconciliationBestEffortCacheAndSameBatch(t *testing.T) {
	ctx := context.Background()
	for _, sameBatch := range []bool{false, true} {
		t.Run(fmt.Sprint("same_batch=", sameBatch), func(t *testing.T) {
			repo, failed, cmd := newBillingLogRecoveryFixture(t)
			settled := settleBillingRecoveryLog(t, failed, cmd)
			if sameBatch {
				firstCh, nextCh := make(chan error, 1), make(chan error, 1)
				repo.flushBestEffortBatch(integrationDB, []usageLogBestEffortRequest{
					{prepared: prepareUsageLogInsert(failed), apiKeyID: failed.APIKeyID, resultCh: firstCh},
					{prepared: prepareUsageLogInsert(settled), apiKeyID: failed.APIKeyID, resultCh: nextCh},
				})
				require.NoError(t, <-firstCh)
				require.NoError(t, <-nextCh)
			} else {
				require.NoError(t, repo.CreateBestEffort(ctx, failed))
				require.NoError(t, repo.CreateBestEffort(ctx, settled), "recent cache must not suppress repair")
			}
			assertBillingRecoveryLog(t, failed, true, 2)
		})
	}
}

func TestBillingLogReconciliationLegacyConflictAndArchiveProof(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"legacy", "different_fingerprint", "missing_proof", "archive", "conflicting_archive", "zero"} {
		t.Run(mode, func(t *testing.T) {
			repo, failed, cmd := newBillingLogRecoveryFixture(t)
			if mode == "zero" {
				cmd.ActualCost, cmd.BilledCost, cmd.BillingSurchargeCost, cmd.BalanceCost = 0, 0, 0, 0
				cmd.RequestFingerprint = ""
				cmd.Normalize()
				failed.BillingRequestFingerprint = cmd.RequestFingerprint
			}
			if mode == "legacy" {
				failed.BillingRequestFingerprint = ""
			}
			_, err := repo.createSingle(ctx, integrationDB, failed)
			require.NoError(t, err)
			settled := *failed
			if mode == "missing_proof" {
				settled.BillingSettled, settled.ActualCost = true, 2
			} else {
				settled = *settleBillingRecoveryLog(t, failed, cmd)
				settled.BillingRequestFingerprint = cmd.RequestFingerprint
			}
			if mode == "different_fingerprint" {
				settled.BillingRequestFingerprint = "unrelated"
			}
			if mode == "archive" || mode == "conflicting_archive" {
				fp := cmd.RequestFingerprint
				if mode == "conflicting_archive" {
					fp = "unrelated"
				}
				_, err = integrationDB.ExecContext(ctx, `INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at) VALUES($1,$2,$3,NOW())`, cmd.RequestID, cmd.APIKeyID, fp)
				require.NoError(t, err)
				if mode == "archive" {
					_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, cmd.APIKeyID)
					require.NoError(t, err)
				}
			}
			inserted, err := repo.createSingle(ctx, integrationDB, &settled)
			require.NoError(t, err)
			require.False(t, inserted)
			wantSettled := mode == "archive" || mode == "zero"
			actual := 0.0
			if mode == "archive" {
				actual = 2
			}
			assertBillingRecoveryLog(t, failed, wantSettled, actual)
		})
	}
}

func TestBillingLogReconciliationConcurrentInsertAndPromotion(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint("batch=", batch), func(t *testing.T) {
			repo, failed, cmd := newBillingLogRecoveryFixture(t)
			settled := settleBillingRecoveryLog(t, failed, cmd)
			start := make(chan struct{})
			type outcome struct {
				inserted bool
				id       int64
				err      error
			}
			results := make(chan outcome, 16)
			var wg sync.WaitGroup
			for i := 0; i < 16; i++ {
				copy := *failed
				if i%2 == 0 {
					copy = *settled
				}
				wg.Add(1)
				go func(log service.UsageLog) {
					defer wg.Done()
					<-start
					var inserted bool
					var err error
					if batch {
						inserted, err = repo.Create(context.Background(), &log)
					} else {
						inserted, err = repo.createSingle(context.Background(), integrationDB, &log)
					}
					results <- outcome{inserted: inserted, id: log.ID, err: err}
				}(copy)
			}
			close(start)
			wg.Wait()
			close(results)
			insertedCount := 0
			var id int64
			for result := range results {
				require.NoError(t, result.err)
				require.Positive(t, result.id)
				if id == 0 {
					id = result.id
				}
				require.Equal(t, id, result.id)
				if result.inserted {
					insertedCount++
				}
			}
			require.Equal(t, 1, insertedCount)
			assertBillingRecoveryLog(t, failed, true, 2)
		})
	}
}

func TestBillingLogReconciliationCreateSameBatchPromotion(t *testing.T) {
	repo, failed, cmd := newBillingLogRecoveryFixture(t)
	settled := settleBillingRecoveryLog(t, failed, cmd)
	changedStats := 99.0
	settled.Model, settled.InputTokens, settled.AccountStatsCost = "must-not-replace", 99, &changedStats
	firstCh, nextCh := make(chan usageLogCreateResult, 1), make(chan usageLogCreateResult, 1)
	repo.flushCreateBatch(integrationDB, []usageLogCreateRequest{
		{log: failed, prepared: prepareUsageLogInsert(failed), resultCh: firstCh},
		{log: settled, prepared: prepareUsageLogInsert(settled), resultCh: nextCh},
	})
	first, next := <-firstCh, <-nextCh
	require.NoError(t, first.err)
	require.NoError(t, next.err)
	require.True(t, first.inserted)
	require.False(t, next.inserted)
	require.Positive(t, failed.ID)
	require.Equal(t, failed.ID, settled.ID)
	assertBillingRecoveryLog(t, failed, true, 2)
}

func TestBillingLogReconciliationBatchConcurrentCommitSnapshot(t *testing.T) {
	for _, pendingSettlement := range []bool{false, true} {
		t.Run(fmt.Sprint("winner_settled=", pendingSettlement), func(t *testing.T) {
			ctx := context.Background()
			repo, failed, cmd := newBillingLogRecoveryFixture(t)
			settled := settleBillingRecoveryLog(t, failed, cmd)
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = tx.Rollback() })
			winner := *failed
			if pendingSettlement {
				winner = *settled
			}
			inserted, err := repo.createSingle(ctx, tx, &winner)
			require.NoError(t, err)
			require.True(t, inserted)
			key := usageLogBatchKey(settled.RequestID, settled.APIKeyID)
			type outcome struct {
				inserted bool
				state    usageLogBatchState
				err      error
			}
			done := make(chan outcome, 1)
			go func() {
				inserts, states, _, err := repo.batchInsertUsageLogs(integrationDB, []string{key}, map[string]usageLogInsertPrepared{key: prepareUsageLogInsert(settled)})
				done <- outcome{inserted: inserts[key], state: states[key], err: err}
			}()
			// Observe the second connection blocked on the uncommitted unique-key row.
			// Only then commit, guaranteeing its original statement snapshot predates it.
			var early *outcome
			require.Eventually(t, func() bool {
				select {
				case result := <-done:
					early = &result
					return true
				default:
				}
				var count int
				err := integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity
     WHERE pid <> pg_backend_pid() AND wait_event_type='Lock' AND query LIKE '%WITH input%'`).Scan(&count)
				return err == nil && count > 0
			}, 5*time.Second, 10*time.Millisecond)
			if early != nil {
				require.NoError(t, early.err)
				t.Fatal("batch completed before the conflicting transaction committed")
			}
			if !pendingSettlement {
				// Move the proof atomically after the waiter's statement snapshot.
				// Its single live/archive proof query must still see a valid claim.
				_, err = integrationDB.ExecContext(ctx, `WITH moved AS (
 DELETE FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2
 RETURNING request_id, api_key_id, request_fingerprint, created_at
) INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at)
 SELECT request_id,api_key_id,request_fingerprint,created_at FROM moved`, cmd.RequestID, cmd.APIKeyID)
				require.NoError(t, err)
			}
			require.NoError(t, tx.Commit())
			result := <-done
			require.NoError(t, result.err)
			require.False(t, result.inserted, "another transaction owns the sole fresh insert")
			require.Equal(t, winner.ID, result.state.ID)
			require.False(t, result.state.CreatedAt.IsZero())
			assertBillingRecoveryLog(t, failed, true, 2)
		})
	}
}

func TestBillingLogReconciliationRollbackIsAtomic(t *testing.T) {
	ctx := context.Background()
	repo, failed, cmd := newBillingLogRecoveryFixture(t)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = repo.createSingle(ctx, tx, failed)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", failed.RequestID, failed.APIKeyID).Scan(&count))
	require.Zero(t, count)

	// A failure after the ledger effect must roll back its claim and debit too.
	cmd.AccountID, cmd.AccountQuotaCost = -999999, 1
	cmd.RequestFingerprint = ""
	ledger := service.NewBalanceLedgerService(integrationDB, nil, nil)
	result, err := NewUsageBillingRepositoryWithLedger(nil, integrationDB, ledger).Apply(ctx, cmd)
	require.Error(t, err)
	require.Nil(t, result)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", failed.UserID).Scan(&balance))
	require.Equal(t, 100.0, balance)
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2", cmd.RequestID, cmd.APIKeyID).Scan(&count))
	require.Zero(t, count)
}
