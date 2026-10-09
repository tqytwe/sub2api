//go:build unit

package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogBillingPreparedMetadataAndPromotion(t *testing.T) {
	first := &service.UsageLog{RequestID: "r", APIKeyID: 2, UserID: 3, AccountID: 4, Model: "original", InputTokens: 9, TotalCost: 4, BillingRequestFingerprint: "fp"}
	settled := *first
	settled.Model = "must-not-replace"
	settled.InputTokens = 99
	settled.ActualCost, settled.BilledCost, settled.BillingSurchargeCost = 2, 2.2, 0.2
	settled.BillingSurchargeMode, settled.BillingSurchargeValue = "percent_on_charged_cost", 0.1
	settled.BillingSettled = true
	a, b := prepareUsageLogInsert(first), prepareUsageLogInsert(&settled)
	require.Len(t, a.args, 72)
	require.Equal(t, "fp", a.args[usageLogBillingFingerprintArg])
	require.Equal(t, false, a.args[usageLogBillingSettledArg])
	promoted := mergeUsageLogBillingPrepared(a, b)
	for _, index := range []int{usageLogActualCostArg, usageLogBilledCostArg, usageLogSurchargeCostArg, usageLogSurchargeModeArg, usageLogSurchargeValueArg, usageLogBillingSettledArg} {
		require.Equal(t, b.args[index], promoted.args[index])
	}
	require.Equal(t, a.args[4], promoted.args[4])
	require.Equal(t, a.args[11], promoted.args[11])
	require.Equal(t, a.args[25], promoted.args[25])
	require.False(t, a.billingSettled, "promotion must not mutate another queued snapshot")
	require.Equal(t, promoted, mergeUsageLogBillingPrepared(promoted, a), "stale failure cannot downgrade")
	b.billingFingerprint = "other"
	require.Equal(t, a, mergeUsageLogBillingPrepared(a, b))
	a.billingFingerprint = ""
	require.Equal(t, a, mergeUsageLogBillingPrepared(a, b), "legacy row is never adopted")
}

func TestUsageLogBillingQueryKeepsLegacyContractAndSingleSnapshotProof(t *testing.T) {
	legacy := prepareUsageLogInsert(&service.UsageLog{RequestID: "r", APIKeyID: 1})
	query, _ := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{legacy})
	require.Contains(t, query, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	marked := prepareUsageLogInsert(&service.UsageLog{RequestID: "r", APIKeyID: 1, BillingRequestFingerprint: "fp", BillingSettled: true})
	query, _ = buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{marked})
	require.Contains(t, query, "BOOL_AND(proof.request_fingerprint = EXCLUDED.billing_request_fingerprint)")
	require.Contains(t, query, "UNION ALL")
	require.Contains(t, query, "AND NOT usage_logs.billing_settled")
	require.NotContains(t, query, "account_stats_cost = EXCLUDED")
	key := usageLogBatchKey("r", 1)
	query, _ = buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: marked})
	require.Contains(t, query, "(xmax = 0) AS actually_inserted")
	require.Contains(t, query, "COALESCE(inserted.actually_inserted, FALSE) AS inserted")
}

func TestUsageBillingSettlementProofBoundaries(t *testing.T) {
	for _, mode := range []string{"commit", "commit_error", "duplicate", "archive", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &usageBillingRepository{db: db}
			cmd := &service.UsageBillingCommand{RequestID: "proof-zero", APIKeyID: 1}
			cmd.Normalize()
			mock.ExpectBegin()
			claim := mock.ExpectQuery("INSERT INTO usage_billing_dedup")
			if mode == "duplicate" || mode == "conflict" {
				claim.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				fp := cmd.RequestFingerprint
				if mode == "conflict" {
					fp = "different"
				}
				mock.ExpectQuery("SELECT request_fingerprint.*FROM usage_billing_dedup").WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint"}).AddRow(fp))
				mock.ExpectRollback()
			} else {
				claim.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				rows := sqlmock.NewRows([]string{"request_fingerprint"})
				if mode == "archive" {
					rows.AddRow(cmd.RequestFingerprint)
				}
				mock.ExpectQuery("SELECT request_fingerprint.*FROM usage_billing_dedup_archive").WillReturnRows(rows)
				if mode == "archive" {
					mock.ExpectRollback()
				} else if mode == "commit_error" {
					mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
				} else {
					mock.ExpectCommit()
				}
			}
			result, err := repo.Apply(context.Background(), cmd)
			if strings.Contains(mode, "error") || mode == "conflict" {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.True(t, result.SettlementVerified)
				require.Equal(t, cmd.RequestFingerprint, result.SettlementFingerprint)
				require.Equal(t, mode == "commit", result.Applied)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUsageLogBillingBatchFallbackPreservesMetadata(t *testing.T) {
	for _, bestEffort := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "best_effort"}[bestEffort], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := newUsageLogRepositoryWithSQL(nil, db)
			log := &service.UsageLog{RequestID: "fallback", APIKeyID: 2, BillingRequestFingerprint: "verified-fp", BillingSettled: true, ActualCost: 2, CreatedAt: time.Unix(1700000000, 0).UTC()}
			prepared := prepareUsageLogInsert(log)
			args := make([]driver.Value, len(prepared.args))
			for i, arg := range prepared.args {
				args[i] = arg
			}
			pattern := "(?s)INSERT INTO usage_logs.*billing_request_fingerprint.*ON CONFLICT.*DO UPDATE.*BOOL_AND"
			if bestEffort {
				mock.ExpectExec("WITH input").WillReturnError(errors.New("force batch fallback"))
				mock.ExpectExec(pattern).WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
				ch := make(chan error, 1)
				repo.flushBestEffortBatch(db, []usageLogBestEffortRequest{{prepared: prepared, apiKeyID: 2, resultCh: ch}})
				require.NoError(t, <-ch)
			} else {
				mock.ExpectQuery("WITH input").WillReturnError(errors.New("force batch fallback"))
				mock.ExpectQuery(pattern).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "inserted"}).AddRow(1, time.Now(), false))
				ch := make(chan usageLogCreateResult, 1)
				repo.flushCreateBatch(db, []usageLogCreateRequest{{log: log, prepared: prepared, resultCh: ch}})
				result := <-ch
				require.NoError(t, result.err)
				require.False(t, result.inserted)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUsageLogBillingFreshSnapshotLookup(t *testing.T) {
	for _, lookupError := range []bool{false, true} {
		t.Run(map[bool]string{false: "resolve", true: "error"}[lookupError], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			log := &service.UsageLog{RequestID: "missing", APIKeyID: 2}
			key := usageLogBatchKey(log.RequestID, log.APIKeyID)
			prepared := map[string]usageLogInsertPrepared{key: prepareUsageLogInsert(log)}
			inserted := map[string]bool{key: true}
			states := map[string]usageLogBatchState{key: {}}
			expectation := mock.ExpectQuery("SELECT request_id, api_key_id, id, created_at FROM usage_logs").WithArgs("missing", int64(2))
			if lookupError {
				expectation.WillReturnError(errors.New("lookup failed"))
			} else {
				expectation.WillReturnRows(sqlmock.NewRows([]string{"request_id", "api_key_id", "id", "created_at"}).AddRow("missing", 2, 17, time.Now()))
			}
			err = resolveUsageLogBatchMissingStates(context.Background(), db, []string{key}, prepared, inserted, states)
			require.False(t, inserted[key])
			if lookupError {
				require.Error(t, err)
				require.NotContains(t, states, key, "null state must never be reported resolved")
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(17), states[key].ID)
				require.NoError(t, resolveUsageLogBatchMissingStates(context.Background(), db, []string{key}, prepared, inserted, states), "resolved rows need no extra query")
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
