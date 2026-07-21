package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	withdrawableEntitlementStatusActive   = "active"
	withdrawableEntitlementStatusConsumed = "consumed"

	withdrawableAllocationGrant   = "grant"
	withdrawableAllocationConsume = "consume"
	withdrawableAllocationRestore = "restore"

	withdrawableRewardMaturityDelay = 72 * time.Hour
)

type withdrawableGrantPolicy struct {
	Eligible    bool
	AvailableAt time.Time
}

type withdrawableEntitlementSnapshot struct {
	ID          int64
	Remaining   decimal.Decimal
	AvailableAt time.Time
}

type withdrawableConsumedAllocationSnapshot struct {
	EntitlementID int64
	Amount        decimal.Decimal
	AvailableAt   time.Time
}

type withdrawableAllocationPlan struct {
	EntitlementID int64
	Amount        decimal.Decimal
	AvailableAt   time.Time
}

type withdrawableConsumptionPlan struct {
	NonEntitlementAmount     decimal.Decimal
	EntitlementAmount        decimal.Decimal
	MatureWithdrawableAmount decimal.Decimal
	Allocations              []withdrawableAllocationPlan
}

type withdrawableRestorePlan struct {
	TotalAmount              decimal.Decimal
	MatureWithdrawableAmount decimal.Decimal
	Allocations              []withdrawableAllocationPlan
}

type withdrawableGrantPlan struct {
	Amount      decimal.Decimal
	AvailableAt time.Time
	SourceType  string
	SourceID    string
}

type withdrawableLedgerEffects struct {
	WithdrawableDelta decimal.Decimal
	Grant             *withdrawableGrantPlan
	Consume           withdrawableConsumptionPlan
	Restore           withdrawableRestorePlan
	RestoreFromKey    string
}

type withdrawableEntitlementSums struct {
	Mature  decimal.Decimal
	Pending decimal.Decimal
	Total   decimal.Decimal
}

func classifyWithdrawableGrant(source string, createdAt time.Time) withdrawableGrantPolicy {
	createdAt = createdAt.UTC()
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "affiliate_balance":
		return withdrawableGrantPolicy{Eligible: true, AvailableAt: createdAt}
	case PlayRewardSourceArenaDaily, PlayRewardSourceArenaSettlement, PlayRewardSourceTeamSharedReward:
		return withdrawableGrantPolicy{Eligible: true, AvailableAt: createdAt.Add(withdrawableRewardMaturityDelay)}
	default:
		return withdrawableGrantPolicy{}
	}
}

func planWithdrawableConsumption(beforeBalance, debit decimal.Decimal, entitlements []withdrawableEntitlementSnapshot, asOf time.Time) withdrawableConsumptionPlan {
	asOf = asOf.UTC()
	debit = clampDecimalScale(debit)
	if !debit.IsPositive() {
		return withdrawableConsumptionPlan{}
	}

	totalEntitlements := decimal.Zero
	for _, entitlement := range entitlements {
		if entitlement.Remaining.IsPositive() {
			totalEntitlements = totalEntitlements.Add(entitlement.Remaining)
		}
	}

	nonEntitlement := beforeBalance.Sub(totalEntitlements)
	if nonEntitlement.IsNegative() {
		nonEntitlement = decimal.Zero
	}
	if nonEntitlement.GreaterThan(debit) {
		nonEntitlement = debit
	}

	remaining := debit.Sub(nonEntitlement)
	plan := withdrawableConsumptionPlan{
		NonEntitlementAmount: clampDecimalScale(nonEntitlement),
		Allocations:          make([]withdrawableAllocationPlan, 0),
	}
	for _, entitlement := range entitlements {
		if !remaining.IsPositive() {
			break
		}
		if !entitlement.Remaining.IsPositive() {
			continue
		}
		amount := decimalMin(entitlement.Remaining, remaining)
		amount = clampDecimalScale(amount)
		plan.Allocations = append(plan.Allocations, withdrawableAllocationPlan{
			EntitlementID: entitlement.ID,
			Amount:        amount,
			AvailableAt:   entitlement.AvailableAt.UTC(),
		})
		plan.EntitlementAmount = plan.EntitlementAmount.Add(amount)
		if !entitlement.AvailableAt.After(asOf) {
			plan.MatureWithdrawableAmount = plan.MatureWithdrawableAmount.Add(amount)
		}
		remaining = remaining.Sub(amount)
	}
	plan.EntitlementAmount = clampDecimalScale(plan.EntitlementAmount)
	plan.MatureWithdrawableAmount = clampDecimalScale(plan.MatureWithdrawableAmount)
	return plan
}

func planWithdrawableRestore(amount decimal.Decimal, consumed []withdrawableConsumedAllocationSnapshot, asOf time.Time) withdrawableRestorePlan {
	asOf = asOf.UTC()
	remaining := clampDecimalScale(amount)
	plan := withdrawableRestorePlan{Allocations: make([]withdrawableAllocationPlan, 0)}
	if !remaining.IsPositive() {
		return plan
	}
	for _, allocation := range consumed {
		if !remaining.IsPositive() {
			break
		}
		if !allocation.Amount.IsPositive() {
			continue
		}
		restored := decimalMin(allocation.Amount, remaining)
		restored = clampDecimalScale(restored)
		plan.Allocations = append(plan.Allocations, withdrawableAllocationPlan{
			EntitlementID: allocation.EntitlementID,
			Amount:        restored,
			AvailableAt:   allocation.AvailableAt.UTC(),
		})
		plan.TotalAmount = plan.TotalAmount.Add(restored)
		if !allocation.AvailableAt.After(asOf) {
			plan.MatureWithdrawableAmount = plan.MatureWithdrawableAmount.Add(restored)
		}
		remaining = remaining.Sub(restored)
	}
	plan.TotalAmount = clampDecimalScale(plan.TotalAmount)
	plan.MatureWithdrawableAmount = clampDecimalScale(plan.MatureWithdrawableAmount)
	return plan
}

func decimalFromLedgerFloat(value float64) decimal.Decimal {
	if math.Abs(value) < 0.000000005 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(value).Round(8)
}

func decimalToLedgerFloat(value decimal.Decimal) float64 {
	out, _ := value.Round(8).Float64()
	if math.Abs(out) < 0.000000005 {
		return 0
	}
	return out
}

func clampDecimalScale(value decimal.Decimal) decimal.Decimal {
	if value.Abs().LessThan(decimal.RequireFromString("0.000000005")) {
		return decimal.Zero
	}
	return value.Round(8)
}

func decimalMin(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

func decimalMax(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

func decimalString(value decimal.Decimal) string {
	return value.Round(8).StringFixed(8)
}

func parseLedgerDecimal(raw string, field string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse %s: %w", field, err)
	}
	return value.Round(8), nil
}

func selectWithdrawableEntitlementSums(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, asOf time.Time) (withdrawableEntitlementSums, error) {
	rows, err := runner.QueryContext(ctx, `
SELECT
	COALESCE(SUM(CASE WHEN status = 'active' AND remaining_amount > 0 AND available_at <= $2 THEN remaining_amount ELSE 0 END), 0)::text AS mature_amount,
	COALESCE(SUM(CASE WHEN status = 'active' AND remaining_amount > 0 AND available_at > $2 THEN remaining_amount ELSE 0 END), 0)::text AS pending_amount,
	COALESCE(SUM(CASE WHEN status = 'active' AND remaining_amount > 0 THEN remaining_amount ELSE 0 END), 0)::text AS total_amount
FROM withdrawable_entitlements
WHERE user_id = $1`, userID, asOf.UTC())
	if err != nil {
		return withdrawableEntitlementSums{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return withdrawableEntitlementSums{}, err
		}
		return withdrawableEntitlementSums{}, sql.ErrNoRows
	}
	var matureRaw, pendingRaw, totalRaw string
	if err := rows.Scan(&matureRaw, &pendingRaw, &totalRaw); err != nil {
		return withdrawableEntitlementSums{}, err
	}
	if rows.Next() {
		return withdrawableEntitlementSums{}, ErrBalanceLedgerInvalidInput
	}
	mature, err := parseLedgerDecimal(matureRaw, "mature withdrawable")
	if err != nil {
		return withdrawableEntitlementSums{}, err
	}
	pending, err := parseLedgerDecimal(pendingRaw, "pending withdrawable")
	if err != nil {
		return withdrawableEntitlementSums{}, err
	}
	total, err := parseLedgerDecimal(totalRaw, "total withdrawable")
	if err != nil {
		return withdrawableEntitlementSums{}, err
	}
	return withdrawableEntitlementSums{Mature: mature, Pending: pending, Total: total}, rows.Err()
}

func selectWithdrawableEntitlementSnapshots(ctx context.Context, runner balanceLedgerSQLRunner, userID int64) ([]withdrawableEntitlementSnapshot, error) {
	rows, err := runner.QueryContext(ctx, `
SELECT id, remaining_amount::text, available_at
FROM withdrawable_entitlements
WHERE user_id = $1
  AND status = 'active'
  AND remaining_amount > 0
ORDER BY available_at ASC, id ASC
FOR UPDATE`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []withdrawableEntitlementSnapshot
	for rows.Next() {
		var item withdrawableEntitlementSnapshot
		var amountRaw string
		if err := rows.Scan(&item.ID, &amountRaw, &item.AvailableAt); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(amountRaw, "withdrawable entitlement remaining")
		if err != nil {
			return nil, err
		}
		item.Remaining = amount
		item.AvailableAt = item.AvailableAt.UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func selectConsumedAllocationsForRestore(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, originalKey string) ([]withdrawableConsumedAllocationSnapshot, error) {
	originalKey = strings.TrimSpace(originalKey)
	if originalKey == "" {
		return nil, nil
	}
	rows, err := runner.QueryContext(ctx, `
WITH consumed AS (
	SELECT wea.entitlement_id, SUM(wea.amount) AS amount
	FROM withdrawable_entitlement_allocations wea
	JOIN balance_transactions bt ON bt.id = wea.balance_transaction_id
	WHERE wea.user_id = $1
	  AND wea.action = 'consume'
	  AND bt.idempotency_key = $2
	GROUP BY wea.entitlement_id
),
restored AS (
	SELECT entitlement_id, SUM(amount) AS amount
	FROM withdrawable_entitlement_allocations
	WHERE user_id = $1
	  AND action = 'restore'
	  AND metadata->>'restored_from_idempotency_key' = $2
	GROUP BY entitlement_id
)
SELECT c.entitlement_id, (c.amount - COALESCE(r.amount, 0))::text AS restorable_amount, we.available_at
FROM consumed c
JOIN withdrawable_entitlements we ON we.id = c.entitlement_id
LEFT JOIN restored r ON r.entitlement_id = c.entitlement_id
WHERE c.amount > COALESCE(r.amount, 0)
ORDER BY we.available_at ASC, we.id ASC
FOR UPDATE OF we`, userID, originalKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []withdrawableConsumedAllocationSnapshot
	for rows.Next() {
		var item withdrawableConsumedAllocationSnapshot
		var amountRaw string
		if err := rows.Scan(&item.EntitlementID, &amountRaw, &item.AvailableAt); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(amountRaw, "withdrawable consumed allocation")
		if err != nil {
			return nil, err
		}
		item.Amount = amount
		item.AvailableAt = item.AvailableAt.UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func insertWithdrawableGrant(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, transactionID int64, plan withdrawableGrantPlan, createdAt time.Time) error {
	if !plan.Amount.IsPositive() {
		return nil
	}
	var entitlementID int64
	if err := queryOneBalanceLedger(ctx, runner, `
INSERT INTO withdrawable_entitlements (
	user_id,
	balance_transaction_id,
	source_type,
	source_id,
	original_amount,
	remaining_amount,
	available_at,
	status,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $5, $6, 'active', $7, NOW()
)
ON CONFLICT (user_id, balance_transaction_id) WHERE balance_transaction_id IS NOT NULL DO UPDATE
SET updated_at = withdrawable_entitlements.updated_at
RETURNING id`, []any{
		userID,
		transactionID,
		plan.SourceType,
		plan.SourceID,
		decimalString(plan.Amount),
		plan.AvailableAt.UTC(),
		createdAt.UTC(),
	}, &entitlementID); err != nil {
		return fmt.Errorf("insert withdrawable entitlement: %w", err)
	}
	return insertWithdrawableAllocation(ctx, runner, userID, entitlementID, transactionID, withdrawableAllocationGrant, plan.Amount, plan.AvailableAt, plan.SourceType, plan.SourceID, nil, createdAt)
}

func applyWithdrawableConsumption(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, transactionID int64, plan withdrawableConsumptionPlan, sourceType string, sourceID string, createdAt time.Time) error {
	for _, allocation := range plan.Allocations {
		if !allocation.Amount.IsPositive() {
			continue
		}
		res, err := runner.ExecContext(ctx, `
UPDATE withdrawable_entitlements
SET remaining_amount = remaining_amount - $1::numeric,
    consumed_amount = consumed_amount + $1::numeric,
    status = CASE WHEN remaining_amount - $1::numeric <= 0 THEN 'consumed' ELSE status END,
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND status = 'active'
  AND remaining_amount >= $1::numeric`, decimalString(allocation.Amount), allocation.EntitlementID, userID)
		if err != nil {
			return fmt.Errorf("consume withdrawable entitlement: %w", err)
		}
		if err := requireRowsAffected(res, "consume withdrawable entitlement"); err != nil {
			return err
		}
		if err := insertWithdrawableAllocation(ctx, runner, userID, allocation.EntitlementID, transactionID, withdrawableAllocationConsume, allocation.Amount, allocation.AvailableAt, sourceType, sourceID, nil, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func applyWithdrawableRestore(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, transactionID int64, plan withdrawableRestorePlan, sourceType string, sourceID string, restoreFromKey string, createdAt time.Time) error {
	for _, allocation := range plan.Allocations {
		if !allocation.Amount.IsPositive() {
			continue
		}
		res, err := runner.ExecContext(ctx, `
UPDATE withdrawable_entitlements
SET remaining_amount = remaining_amount + $1::numeric,
    consumed_amount = consumed_amount - $1::numeric,
    status = 'active',
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND consumed_amount >= $1::numeric`, decimalString(allocation.Amount), allocation.EntitlementID, userID)
		if err != nil {
			return fmt.Errorf("restore withdrawable entitlement: %w", err)
		}
		if err := requireRowsAffected(res, "restore withdrawable entitlement"); err != nil {
			return err
		}
		metadata := map[string]any{"restored_from_idempotency_key": restoreFromKey}
		if err := insertWithdrawableAllocation(ctx, runner, userID, allocation.EntitlementID, transactionID, withdrawableAllocationRestore, allocation.Amount, allocation.AvailableAt, sourceType, sourceID, metadata, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func insertWithdrawableAllocation(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	userID int64,
	entitlementID int64,
	transactionID int64,
	action string,
	amount decimal.Decimal,
	availableAt time.Time,
	sourceType string,
	sourceID string,
	metadata map[string]any,
	createdAt time.Time,
) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return ErrBalanceLedgerInvalidInput.WithCause(err)
	}
	_, err = runner.ExecContext(ctx, `
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
	$1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10
)`, userID, entitlementID, transactionID, action, decimalString(amount), availableAt.UTC(), sourceType, sourceID, string(raw), createdAt.UTC())
	if err != nil {
		return fmt.Errorf("insert withdrawable allocation: %w", err)
	}
	return nil
}

func requireRowsAffected(res sql.Result, action string) error {
	if res == nil {
		return ErrBalanceLedgerInvalidInput.WithMetadata(map[string]string{"action": action})
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", action, err)
	}
	if affected != 1 {
		return ErrBalanceLedgerInvalidInput.WithMetadata(map[string]string{"action": action})
	}
	return nil
}

func withdrawableRestoreSourceKey(input BalanceLedgerApplyInput) string {
	if input.Metadata == nil {
		return ""
	}
	for _, key := range []string{
		"ledger_deduct_key",
		"restore_ledger_key",
		"reverses_idempotency_key",
		"original_idempotency_key",
		"deduct_idempotency_key",
	} {
		if value, ok := input.Metadata[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}
