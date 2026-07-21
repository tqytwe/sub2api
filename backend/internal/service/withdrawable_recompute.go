package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

const (
	WithdrawableRecomputeModeDryRun  = "dry_run"
	WithdrawableRecomputeModeExecute = "execute"

	WithdrawableRecomputeStatusReady       = "ready"
	WithdrawableRecomputeStatusNeedsReview = "needs_review"
)

type WithdrawableRecomputeService struct {
	db  *sql.DB
	now func() time.Time
}

type WithdrawableRecomputeOptions struct {
	Execute bool
	UserID  int64
	Limit   int
}

type WithdrawableRecomputeReport struct {
	Mode             string                            `json:"mode"`
	UsersScanned     int                               `json:"users_scanned"`
	ReadyUsers       int                               `json:"ready_users"`
	NeedsReviewUsers int                               `json:"needs_review_users"`
	GeneratedAt      time.Time                         `json:"generated_at"`
	Users            []WithdrawableRecomputeUserReport `json:"users"`
}

type WithdrawableRecomputeUserReport struct {
	UserID                       int64                        `json:"user_id"`
	Status                       string                       `json:"status"`
	LedgerBalance                decimal.Decimal              `json:"ledger_balance"`
	ComputedWithdrawableBalance  decimal.Decimal              `json:"computed_withdrawable_balance"`
	ComputedPendingBalance       decimal.Decimal              `json:"computed_pending_balance"`
	ComputedEntitlementBalance   decimal.Decimal              `json:"computed_entitlement_balance"`
	ExistingEntitlementCount     int                          `json:"existing_entitlement_count"`
	ExistingEntitlementsVerified bool                         `json:"existing_entitlements_verified"`
	ExistingWithdrawableBalance  decimal.Decimal              `json:"existing_withdrawable_balance"`
	ExistingPendingBalance       decimal.Decimal              `json:"existing_pending_balance"`
	ExistingEntitlementBalance   decimal.Decimal              `json:"existing_entitlement_balance"`
	TransactionCount             int                          `json:"transaction_count"`
	EligibleGrantCount           int                          `json:"eligible_grant_count"`
	Anomalies                    []string                     `json:"anomalies,omitempty"`
	Batches                      []WithdrawableRecomputeBatch `json:"batches,omitempty"`
}

type WithdrawableRecomputeBatch struct {
	SourceTransactionID int64           `json:"source_transaction_id"`
	SourceType          string          `json:"source_type"`
	SourceID            string          `json:"source_id"`
	OriginalAmount      decimal.Decimal `json:"original_amount"`
	RemainingAmount     decimal.Decimal `json:"remaining_amount"`
	ConsumedAmount      decimal.Decimal `json:"consumed_amount"`
	AvailableAt         time.Time       `json:"available_at"`
}

type WithdrawableInvariantReport struct {
	CheckedAt                         time.Time `json:"checked_at"`
	EntitlementsExceedBalanceCount    int64     `json:"entitlements_exceed_balance_count"`
	BatchSumMismatchCount             int64     `json:"batch_sum_mismatch_count"`
	WithdrawalFrozenMismatchCount     int64     `json:"withdrawal_frozen_mismatch_count"`
	ImageTouchedWithdrawalFrozenCount int64     `json:"image_touched_withdrawal_frozen_count"`
	Passed                            bool      `json:"passed"`
}

type withdrawableRecomputeTransaction struct {
	ID             int64
	BalanceDelta   decimal.Decimal
	BalanceBefore  *decimal.Decimal
	BalanceAfter   *decimal.Decimal
	SourceType     string
	SourceID       string
	IdempotencyKey string
	Metadata       map[string]any
	Confidence     string
	CreatedAt      time.Time
}

type withdrawableRecomputeLocalAllocation struct {
	BatchIndex  int
	Amount      decimal.Decimal
	AvailableAt time.Time
}

type withdrawableRecomputeExistingBatch struct {
	Batch                  WithdrawableRecomputeBatch
	WithdrawalFrozenAmount decimal.Decimal
	Status                 string
}

type withdrawableRecomputeQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func NewWithdrawableRecomputeService(db *sql.DB) *WithdrawableRecomputeService {
	return &WithdrawableRecomputeService{db: db, now: time.Now}
}

func (s *WithdrawableRecomputeService) CheckInvariants(ctx context.Context) (*WithdrawableInvariantReport, error) {
	if s == nil || s.db == nil {
		return nil, ErrBalanceLedgerUnavailable
	}
	checkedAt := s.now().UTC()
	report := &WithdrawableInvariantReport{CheckedAt: checkedAt}
	checks := []struct {
		dest  *int64
		query string
	}{
		{
			dest: &report.EntitlementsExceedBalanceCount,
			query: `
WITH entitlement_totals AS (
	SELECT user_id, COALESCE(SUM(remaining_amount + withdrawal_frozen_amount), 0) AS entitlement_amount
	FROM withdrawable_entitlements
	WHERE status = 'active'
	GROUP BY user_id
)
SELECT COUNT(*)::bigint
FROM users u
JOIN entitlement_totals e ON e.user_id = u.id
WHERE u.deleted_at IS NULL
  AND e.entitlement_amount > COALESCE(u.balance, 0) + COALESCE(u.withdrawal_frozen_balance, 0) + 0.00000001`,
		},
		{
			dest: &report.BatchSumMismatchCount,
			query: `
SELECT COUNT(*)::bigint
FROM withdrawable_entitlements
WHERE original_amount <> remaining_amount + consumed_amount + withdrawal_frozen_amount`,
		},
		{
			dest: &report.WithdrawalFrozenMismatchCount,
			query: `
WITH frozen_totals AS (
	SELECT user_id, COALESCE(SUM(withdrawal_frozen_amount), 0) AS entitlement_frozen
	FROM withdrawable_entitlements
	GROUP BY user_id
)
SELECT COUNT(*)::bigint
FROM users u
LEFT JOIN frozen_totals f ON f.user_id = u.id
WHERE u.deleted_at IS NULL
  AND ABS(COALESCE(f.entitlement_frozen, 0) - COALESCE(u.withdrawal_frozen_balance, 0)) > 0.00000001`,
		},
		{
			dest: &report.ImageTouchedWithdrawalFrozenCount,
			query: `
SELECT COUNT(*)::bigint
FROM balance_transactions
WHERE (source_type LIKE '%image%' OR source_type LIKE '%batch_image%')
  AND COALESCE(withdrawal_frozen_delta, 0) <> 0`,
		},
	}
	for _, check := range checks {
		if err := s.db.QueryRowContext(ctx, check.query).Scan(check.dest); err != nil {
			return nil, fmt.Errorf("check withdrawable invariant: %w", err)
		}
	}
	report.Passed = report.EntitlementsExceedBalanceCount == 0 &&
		report.BatchSumMismatchCount == 0 &&
		report.WithdrawalFrozenMismatchCount == 0 &&
		report.ImageTouchedWithdrawalFrozenCount == 0
	return report, nil
}

func (s *WithdrawableRecomputeService) Recompute(ctx context.Context, opts WithdrawableRecomputeOptions) (*WithdrawableRecomputeReport, error) {
	if s == nil || s.db == nil {
		return nil, ErrBalanceLedgerUnavailable
	}
	if opts.Limit <= 0 {
		opts.Limit = 500
	}
	if opts.Limit > 5000 {
		opts.Limit = 5000
	}
	mode := WithdrawableRecomputeModeDryRun
	if opts.Execute {
		mode = WithdrawableRecomputeModeExecute
	}
	generatedAt := s.now().UTC()
	users, err := s.listWithdrawableRecomputeUsers(ctx, opts.UserID, opts.Limit)
	if err != nil {
		return nil, err
	}
	report := &WithdrawableRecomputeReport{
		Mode:        mode,
		GeneratedAt: generatedAt,
		Users:       make([]WithdrawableRecomputeUserReport, 0, len(users)),
	}
	for _, userID := range users {
		userReport, err := s.recomputeUser(ctx, userID, generatedAt)
		if err != nil {
			userReport = WithdrawableRecomputeUserReport{
				UserID:    userID,
				Status:    WithdrawableRecomputeStatusNeedsReview,
				Anomalies: []string{err.Error()},
			}
		}
		if opts.Execute {
			if err := s.persistWithdrawableRecomputeUser(ctx, userReport, mode, generatedAt); err != nil {
				userReport.Status = WithdrawableRecomputeStatusNeedsReview
				userReport.Anomalies = append(userReport.Anomalies, err.Error())
			}
		}
		report.Users = append(report.Users, userReport)
		report.UsersScanned++
		if userReport.Status == WithdrawableRecomputeStatusReady {
			report.ReadyUsers++
		} else {
			report.NeedsReviewUsers++
		}
	}
	return report, nil
}

func (s *WithdrawableRecomputeService) listWithdrawableRecomputeUsers(ctx context.Context, userID int64, limit int) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id
FROM users
WHERE deleted_at IS NULL
  AND ($1::bigint = 0 OR id = $1::bigint)
ORDER BY id ASC
LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list withdrawable recompute users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var users []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		users = append(users, id)
	}
	return users, rows.Err()
}

func (s *WithdrawableRecomputeService) recomputeUser(ctx context.Context, userID int64, asOf time.Time) (WithdrawableRecomputeUserReport, error) {
	currentBalance, existingEntitlements, err := s.queryWithdrawableRecomputeUserState(ctx, userID)
	if err != nil {
		return WithdrawableRecomputeUserReport{}, err
	}
	transactions, err := s.listWithdrawableRecomputeTransactions(ctx, userID)
	if err != nil {
		return WithdrawableRecomputeUserReport{}, err
	}
	report := WithdrawableRecomputeUserReport{
		UserID:           userID,
		Status:           WithdrawableRecomputeStatusReady,
		LedgerBalance:    currentBalance,
		TransactionCount: len(transactions),
		Batches:          make([]WithdrawableRecomputeBatch, 0),
	}

	consumedByKey := map[string][]withdrawableRecomputeLocalAllocation{}
	runningBalance := decimal.Zero
	haveRunningBalance := false
	for _, tx := range transactions {
		if tx.Confidence != BalanceLedgerConfidenceHigh {
			report.Status = WithdrawableRecomputeStatusNeedsReview
			report.Anomalies = append(report.Anomalies, fmt.Sprintf("transaction %d confidence is %s", tx.ID, tx.Confidence))
			continue
		}
		if tx.BalanceBefore != nil {
			runningBalance = *tx.BalanceBefore
			haveRunningBalance = true
		}
		if tx.BalanceDelta.IsNegative() && !haveRunningBalance {
			report.Status = WithdrawableRecomputeStatusNeedsReview
			report.Anomalies = append(report.Anomalies, fmt.Sprintf("transaction %d missing reliable balance_before", tx.ID))
			continue
		}

		if tx.BalanceDelta.IsPositive() {
			restoreKey := withdrawableRestoreSourceKey(BalanceLedgerApplyInput{Metadata: tx.Metadata})
			if restoreKey != "" {
				restored := applyWithdrawableRecomputeRestore(&report, tx.BalanceDelta, consumedByKey[restoreKey], asOf)
				if restored.LessThan(tx.BalanceDelta) {
					report.Status = WithdrawableRecomputeStatusNeedsReview
					report.Anomalies = append(report.Anomalies, fmt.Sprintf("transaction %d could restore only %s of %s", tx.ID, decimalString(restored), decimalString(tx.BalanceDelta)))
				}
			} else if policy := classifyWithdrawableGrant(tx.SourceType, tx.CreatedAt); policy.Eligible {
				report.EligibleGrantCount++
				report.Batches = append(report.Batches, WithdrawableRecomputeBatch{
					SourceTransactionID: tx.ID,
					SourceType:          tx.SourceType,
					SourceID:            tx.SourceID,
					OriginalAmount:      tx.BalanceDelta,
					RemainingAmount:     tx.BalanceDelta,
					AvailableAt:         policy.AvailableAt,
				})
			}
		}
		if tx.BalanceDelta.IsNegative() {
			allocations := applyWithdrawableRecomputeConsumption(&report, runningBalance, tx.BalanceDelta.Neg(), asOf)
			if len(allocations) > 0 {
				consumedByKey[tx.IdempotencyKey] = append(consumedByKey[tx.IdempotencyKey], allocations...)
			}
		}

		runningBalance = runningBalance.Add(tx.BalanceDelta)
		if tx.BalanceAfter != nil {
			runningBalance = *tx.BalanceAfter
		}
		haveRunningBalance = true
	}
	for i := range report.Batches {
		batch := &report.Batches[i]
		batch.OriginalAmount = clampDecimalScale(batch.OriginalAmount)
		batch.RemainingAmount = clampDecimalScale(batch.RemainingAmount)
		batch.ConsumedAmount = clampDecimalScale(batch.ConsumedAmount)
		report.ComputedEntitlementBalance = report.ComputedEntitlementBalance.Add(batch.RemainingAmount)
		if batch.RemainingAmount.IsPositive() {
			if batch.AvailableAt.After(asOf) {
				report.ComputedPendingBalance = report.ComputedPendingBalance.Add(batch.RemainingAmount)
			} else {
				report.ComputedWithdrawableBalance = report.ComputedWithdrawableBalance.Add(batch.RemainingAmount)
			}
		}
	}
	report.ComputedEntitlementBalance = clampDecimalScale(report.ComputedEntitlementBalance)
	report.ComputedPendingBalance = clampDecimalScale(report.ComputedPendingBalance)
	report.ComputedWithdrawableBalance = decimalMax(decimal.Zero, decimalMin(currentBalance, clampDecimalScale(report.ComputedWithdrawableBalance)))
	if report.ComputedEntitlementBalance.GreaterThan(currentBalance.Add(decimal.RequireFromString("0.00000001"))) {
		report.Status = WithdrawableRecomputeStatusNeedsReview
		report.Anomalies = append(report.Anomalies, "computed entitlement balance exceeds current balance")
	}
	if haveRunningBalance && !runningBalance.Equal(currentBalance) {
		report.Status = WithdrawableRecomputeStatusNeedsReview
		report.Anomalies = append(report.Anomalies, fmt.Sprintf("ledger replay balance %s does not match users.balance %s", decimalString(runningBalance), decimalString(currentBalance)))
	}
	if existingEntitlements > 0 {
		existing, err := queryExistingWithdrawableRecomputeBatches(ctx, s.db, userID, false)
		if err != nil {
			return WithdrawableRecomputeUserReport{}, err
		}
		applyExistingWithdrawableEntitlementReview(&report, existing, asOf)
	}
	if report.Status != WithdrawableRecomputeStatusReady && len(report.Batches) == 0 {
		report.Batches = nil
	}
	return report, nil
}

func (s *WithdrawableRecomputeService) queryWithdrawableRecomputeUserState(ctx context.Context, userID int64) (decimal.Decimal, int64, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
	COALESCE(u.balance, 0)::text,
	COALESCE((SELECT COUNT(*) FROM withdrawable_entitlements we WHERE we.user_id = u.id), 0)::bigint
FROM users u
WHERE u.id = $1 AND u.deleted_at IS NULL`, userID)
	var balanceRaw string
	var entitlementCount int64
	if err := row.Scan(&balanceRaw, &entitlementCount); err != nil {
		if err == sql.ErrNoRows {
			return decimal.Zero, 0, ErrUserNotFound
		}
		return decimal.Zero, 0, fmt.Errorf("query withdrawable recompute user state: %w", err)
	}
	balance, err := parseLedgerDecimal(balanceRaw, "recompute user balance")
	return balance, entitlementCount, err
}

func (s *WithdrawableRecomputeService) listWithdrawableRecomputeTransactions(ctx context.Context, userID int64) ([]withdrawableRecomputeTransaction, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
	id,
	balance_delta::text,
	balance_before::text,
	balance_after::text,
	source_type,
	source_id,
	idempotency_key,
	metadata::text,
	confidence,
	created_at
FROM balance_transactions
WHERE user_id = $1
ORDER BY created_at ASC, id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawable recompute transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []withdrawableRecomputeTransaction
	for rows.Next() {
		var tx withdrawableRecomputeTransaction
		var deltaRaw string
		var beforeRaw, afterRaw sql.NullString
		var metadataRaw string
		if err := rows.Scan(&tx.ID, &deltaRaw, &beforeRaw, &afterRaw, &tx.SourceType, &tx.SourceID, &tx.IdempotencyKey, &metadataRaw, &tx.Confidence, &tx.CreatedAt); err != nil {
			return nil, err
		}
		delta, err := parseLedgerDecimal(deltaRaw, "recompute balance delta")
		if err != nil {
			return nil, err
		}
		tx.BalanceDelta = delta
		if beforeRaw.Valid {
			value, err := parseLedgerDecimal(beforeRaw.String, "recompute balance before")
			if err != nil {
				return nil, err
			}
			tx.BalanceBefore = &value
		}
		if afterRaw.Valid {
			value, err := parseLedgerDecimal(afterRaw.String, "recompute balance after")
			if err != nil {
				return nil, err
			}
			tx.BalanceAfter = &value
		}
		tx.Metadata = map[string]any{}
		if err := json.Unmarshal([]byte(metadataRaw), &tx.Metadata); err != nil {
			return nil, fmt.Errorf("parse recompute metadata for transaction %d: %w", tx.ID, err)
		}
		tx.CreatedAt = tx.CreatedAt.UTC()
		out = append(out, tx)
	}
	return out, rows.Err()
}

func applyWithdrawableRecomputeConsumption(report *WithdrawableRecomputeUserReport, beforeBalance decimal.Decimal, debit decimal.Decimal, asOf time.Time) []withdrawableRecomputeLocalAllocation {
	snapshots := make([]withdrawableEntitlementSnapshot, 0, len(report.Batches))
	for index, batch := range report.Batches {
		if batch.RemainingAmount.IsPositive() {
			snapshots = append(snapshots, withdrawableEntitlementSnapshot{
				ID:          int64(index),
				Remaining:   batch.RemainingAmount,
				AvailableAt: batch.AvailableAt,
			})
		}
	}
	plan := planWithdrawableConsumption(beforeBalance, debit, snapshots, asOf)
	out := make([]withdrawableRecomputeLocalAllocation, 0, len(plan.Allocations))
	for _, allocation := range plan.Allocations {
		index := int(allocation.EntitlementID)
		if index < 0 || index >= len(report.Batches) {
			continue
		}
		report.Batches[index].RemainingAmount = report.Batches[index].RemainingAmount.Sub(allocation.Amount)
		report.Batches[index].ConsumedAmount = report.Batches[index].ConsumedAmount.Add(allocation.Amount)
		out = append(out, withdrawableRecomputeLocalAllocation{
			BatchIndex:  index,
			Amount:      allocation.Amount,
			AvailableAt: allocation.AvailableAt,
		})
	}
	return out
}

func applyWithdrawableRecomputeRestore(report *WithdrawableRecomputeUserReport, amount decimal.Decimal, consumed []withdrawableRecomputeLocalAllocation, asOf time.Time) decimal.Decimal {
	snapshots := make([]withdrawableConsumedAllocationSnapshot, 0, len(consumed))
	for _, allocation := range consumed {
		snapshots = append(snapshots, withdrawableConsumedAllocationSnapshot{
			EntitlementID: int64(allocation.BatchIndex),
			Amount:        allocation.Amount,
			AvailableAt:   allocation.AvailableAt,
		})
	}
	plan := planWithdrawableRestore(amount, snapshots, asOf)
	for _, allocation := range plan.Allocations {
		index := int(allocation.EntitlementID)
		if index < 0 || index >= len(report.Batches) {
			continue
		}
		report.Batches[index].RemainingAmount = report.Batches[index].RemainingAmount.Add(allocation.Amount)
		report.Batches[index].ConsumedAmount = report.Batches[index].ConsumedAmount.Sub(allocation.Amount)
	}
	return plan.TotalAmount
}

func (s *WithdrawableRecomputeService) persistWithdrawableRecomputeUser(ctx context.Context, report WithdrawableRecomputeUserReport, mode string, generatedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin withdrawable recompute execute: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var lockedUserID int64
	if err := tx.QueryRowContext(ctx, `
SELECT id
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, report.UserID).Scan(&lockedUserID); err != nil {
		return fmt.Errorf("lock withdrawable recompute user: %w", err)
	}
	existing, err := queryExistingWithdrawableRecomputeBatches(ctx, tx, report.UserID, true)
	if err != nil {
		return fmt.Errorf("lock existing withdrawable entitlements: %w", err)
	}
	if len(existing) > 0 && report.Status == WithdrawableRecomputeStatusReady {
		if !withdrawableRecomputeExistingBatchesMatch(report.Batches, existing) {
			return fmt.Errorf("existing withdrawable entitlements do not match recompute report")
		}
	}
	if report.Status == WithdrawableRecomputeStatusReady && len(existing) == 0 {
		for _, batch := range report.Batches {
			if err := persistWithdrawableRecomputeBatch(ctx, tx, report.UserID, batch, generatedAt); err != nil {
				return err
			}
		}
	}
	reportRaw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET withdrawable_balance = CASE WHEN $2 = 'ready' THEN $1 ELSE withdrawable_balance END,
    withdrawal_recalc_status = $2,
    withdrawal_recalc_checked_at = $3,
    updated_at = NOW()
WHERE id = $4 AND deleted_at IS NULL`,
		decimalString(report.ComputedWithdrawableBalance),
		report.Status,
		generatedAt,
		report.UserID,
	); err != nil {
		return fmt.Errorf("update withdrawable recompute status: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO withdrawable_recalculation_runs (
	user_id,
	mode,
	status,
	ledger_balance,
	computed_withdrawable_balance,
	computed_pending_balance,
	anomaly_count,
	report,
	created_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9
)`,
		report.UserID,
		mode,
		report.Status,
		decimalString(report.LedgerBalance),
		decimalString(report.ComputedWithdrawableBalance),
		decimalString(report.ComputedPendingBalance),
		len(report.Anomalies),
		string(reportRaw),
		generatedAt,
	); err != nil {
		return fmt.Errorf("insert withdrawable recompute run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit withdrawable recompute execute: %w", err)
	}
	committed = true
	return nil
}

func queryExistingWithdrawableRecomputeBatches(ctx context.Context, q withdrawableRecomputeQueryer, userID int64, lock bool) ([]withdrawableRecomputeExistingBatch, error) {
	query := `
SELECT
	balance_transaction_id,
	source_type,
	source_id,
	original_amount::text,
	remaining_amount::text,
	consumed_amount::text,
	withdrawal_frozen_amount::text,
	available_at,
	status
FROM withdrawable_entitlements
WHERE user_id = $1
ORDER BY COALESCE(balance_transaction_id, 0), id`
	if lock {
		query += `
FOR UPDATE`
	}
	rows, err := q.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []withdrawableRecomputeExistingBatch
	for rows.Next() {
		var existing withdrawableRecomputeExistingBatch
		var sourceTransactionID sql.NullInt64
		var originalRaw, remainingRaw, consumedRaw, frozenRaw string
		if err := rows.Scan(
			&sourceTransactionID,
			&existing.Batch.SourceType,
			&existing.Batch.SourceID,
			&originalRaw,
			&remainingRaw,
			&consumedRaw,
			&frozenRaw,
			&existing.Batch.AvailableAt,
			&existing.Status,
		); err != nil {
			return nil, err
		}
		if sourceTransactionID.Valid {
			existing.Batch.SourceTransactionID = sourceTransactionID.Int64
		}
		var err error
		if existing.Batch.OriginalAmount, err = parseLedgerDecimal(originalRaw, "existing entitlement original amount"); err != nil {
			return nil, err
		}
		if existing.Batch.RemainingAmount, err = parseLedgerDecimal(remainingRaw, "existing entitlement remaining amount"); err != nil {
			return nil, err
		}
		if existing.Batch.ConsumedAmount, err = parseLedgerDecimal(consumedRaw, "existing entitlement consumed amount"); err != nil {
			return nil, err
		}
		if existing.WithdrawalFrozenAmount, err = parseLedgerDecimal(frozenRaw, "existing entitlement withdrawal frozen amount"); err != nil {
			return nil, err
		}
		existing.Batch.AvailableAt = existing.Batch.AvailableAt.UTC()
		out = append(out, existing)
	}
	return out, rows.Err()
}

func applyExistingWithdrawableEntitlementReview(report *WithdrawableRecomputeUserReport, existing []withdrawableRecomputeExistingBatch, asOf time.Time) {
	asOf = asOf.UTC()
	report.ExistingEntitlementCount = len(existing)
	for _, existingBatch := range existing {
		if existingBatch.Status == "void" {
			continue
		}
		current := existingBatch.Batch.RemainingAmount
		if !current.IsPositive() {
			continue
		}
		report.ExistingEntitlementBalance = report.ExistingEntitlementBalance.Add(current)
		if existingBatch.Batch.AvailableAt.After(asOf) {
			report.ExistingPendingBalance = report.ExistingPendingBalance.Add(current)
		} else {
			report.ExistingWithdrawableBalance = report.ExistingWithdrawableBalance.Add(current)
		}
	}
	report.ExistingEntitlementBalance = clampDecimalScale(report.ExistingEntitlementBalance)
	report.ExistingPendingBalance = clampDecimalScale(report.ExistingPendingBalance)
	report.ExistingWithdrawableBalance = clampDecimalScale(report.ExistingWithdrawableBalance)

	if len(existing) == 0 {
		return
	}
	matched := withdrawableRecomputeExistingBatchesMatch(report.Batches, existing)
	report.ExistingEntitlementsVerified = matched
	if matched {
		return
	}
	report.Status = WithdrawableRecomputeStatusNeedsReview
	report.Anomalies = append(report.Anomalies, fmt.Sprintf(
		"existing withdrawable entitlements do not match recompute report: existing_count=%d recompute_count=%d existing_total=%s recompute_total=%s",
		len(existing),
		len(report.Batches),
		decimalString(report.ExistingEntitlementBalance),
		decimalString(report.ComputedEntitlementBalance),
	))
}

func withdrawableRecomputeExistingBatchesMatch(recomputed []WithdrawableRecomputeBatch, existing []withdrawableRecomputeExistingBatch) bool {
	if len(recomputed) != len(existing) {
		return false
	}
	recomputedSorted := append([]WithdrawableRecomputeBatch(nil), recomputed...)
	existingSorted := append([]withdrawableRecomputeExistingBatch(nil), existing...)
	sort.SliceStable(recomputedSorted, func(i, j int) bool {
		return withdrawableRecomputeBatchLess(recomputedSorted[i], recomputedSorted[j])
	})
	sort.SliceStable(existingSorted, func(i, j int) bool {
		return withdrawableRecomputeBatchLess(existingSorted[i].Batch, existingSorted[j].Batch)
	})
	for i, recomputedBatch := range recomputedSorted {
		existingBatch := existingSorted[i]
		if !existingBatch.WithdrawalFrozenAmount.IsZero() {
			return false
		}
		expectedStatus := withdrawableEntitlementStatusActive
		if !recomputedBatch.RemainingAmount.IsPositive() {
			expectedStatus = withdrawableEntitlementStatusConsumed
		}
		if existingBatch.Status != expectedStatus {
			return false
		}
		if existingBatch.Batch.SourceTransactionID != recomputedBatch.SourceTransactionID ||
			existingBatch.Batch.SourceType != recomputedBatch.SourceType ||
			existingBatch.Batch.SourceID != recomputedBatch.SourceID ||
			!clampDecimalScale(existingBatch.Batch.OriginalAmount).Equal(clampDecimalScale(recomputedBatch.OriginalAmount)) ||
			!clampDecimalScale(existingBatch.Batch.RemainingAmount).Equal(clampDecimalScale(recomputedBatch.RemainingAmount)) ||
			!clampDecimalScale(existingBatch.Batch.ConsumedAmount).Equal(clampDecimalScale(recomputedBatch.ConsumedAmount)) ||
			!existingBatch.Batch.AvailableAt.UTC().Equal(recomputedBatch.AvailableAt.UTC()) {
			return false
		}
	}
	return true
}

func withdrawableRecomputeBatchLess(a, b WithdrawableRecomputeBatch) bool {
	if a.SourceTransactionID != b.SourceTransactionID {
		return a.SourceTransactionID < b.SourceTransactionID
	}
	if a.SourceType != b.SourceType {
		return a.SourceType < b.SourceType
	}
	if a.SourceID != b.SourceID {
		return a.SourceID < b.SourceID
	}
	if !a.AvailableAt.UTC().Equal(b.AvailableAt.UTC()) {
		return a.AvailableAt.UTC().Before(b.AvailableAt.UTC())
	}
	if !clampDecimalScale(a.OriginalAmount).Equal(clampDecimalScale(b.OriginalAmount)) {
		return clampDecimalScale(a.OriginalAmount).LessThan(clampDecimalScale(b.OriginalAmount))
	}
	if !clampDecimalScale(a.RemainingAmount).Equal(clampDecimalScale(b.RemainingAmount)) {
		return clampDecimalScale(a.RemainingAmount).LessThan(clampDecimalScale(b.RemainingAmount))
	}
	if !clampDecimalScale(a.ConsumedAmount).Equal(clampDecimalScale(b.ConsumedAmount)) {
		return clampDecimalScale(a.ConsumedAmount).LessThan(clampDecimalScale(b.ConsumedAmount))
	}
	return false
}

func persistWithdrawableRecomputeBatch(ctx context.Context, tx *sql.Tx, userID int64, batch WithdrawableRecomputeBatch, createdAt time.Time) error {
	status := withdrawableEntitlementStatusActive
	if !batch.RemainingAmount.IsPositive() {
		status = withdrawableEntitlementStatusConsumed
	}
	var entitlementID int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO withdrawable_entitlements (
	user_id,
	balance_transaction_id,
	source_type,
	source_id,
	original_amount,
	remaining_amount,
	consumed_amount,
	available_at,
	status,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()
)
RETURNING id`,
		userID,
		batch.SourceTransactionID,
		batch.SourceType,
		batch.SourceID,
		decimalString(batch.OriginalAmount),
		decimalString(batch.RemainingAmount),
		decimalString(batch.ConsumedAmount),
		batch.AvailableAt,
		status,
		createdAt,
	).Scan(&entitlementID); err != nil {
		return fmt.Errorf("persist recomputed entitlement: %w", err)
	}
	metadata := map[string]any{
		"recomputed":      true,
		"consumed_amount": decimalString(batch.ConsumedAmount),
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO withdrawable_entitlement_allocations (
	user_id,
	entitlement_id,
	balance_transaction_id,
	action,
	amount,
	available_at,
	source_type,
	source_id,
	metadata,
	created_at
) VALUES (
	$1, $2, $3, 'recompute_adjustment', $4, $5, $6, $7, $8::jsonb, $9
)`,
		userID,
		entitlementID,
		batch.SourceTransactionID,
		decimalString(batch.OriginalAmount),
		batch.AvailableAt,
		batch.SourceType,
		batch.SourceID,
		string(raw),
		createdAt,
	); err != nil {
		return fmt.Errorf("persist recomputed allocation: %w", err)
	}
	return nil
}
