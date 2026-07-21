package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	FundSourceKindOnlineRecharge  = "online_recharge"
	FundSourceKindOfflineRecharge = "offline_recharge"
	FundSourceKindSignupGift      = "signup_gift"
	FundSourceKindOpsGift         = "ops_gift"
	FundSourceKindCompensation    = "compensation"
	FundSourceKindRedeemGift      = "redeem_gift"
	FundSourceKindPromotionGift   = "promotion_gift"
	FundSourceKindUnknown         = "unknown"

	FundLedgerSourceOfflineRecharge = "offline_recharge"
	FundLedgerSourceOpsGift         = "ops_gift"
	FundLedgerSourceSignupGift      = "signup_gift"
	FundLedgerSourceCompensation    = "compensation"

	FundRefundLedgerSourceSubmit = "fund_refund_submit"
	FundRefundLedgerSourceCancel = "fund_refund_cancel"
	FundRefundLedgerSourceReject = "fund_refund_reject"
	FundRefundLedgerSourcePaid   = "fund_refund_paid"

	fundAllocationGrant          = "grant"
	fundAllocationConsume        = "consume"
	fundAllocationRestore        = "restore"
	fundAllocationRefundFreeze   = "refund_freeze"
	fundAllocationRefundUnfreeze = "refund_unfreeze"
	fundAllocationRefundComplete = "refund_complete"
	fundAllocationReclassify     = "reclassify"
)

type fundBatchGrantPolicy struct {
	Eligible       bool
	SourceKind     string
	Refundable     bool
	PaymentOrderID *int64
}

type fundBatchSnapshot struct {
	ID        int64
	Kind      string
	Remaining decimal.Decimal
}

type fundBatchAllocationPlan struct {
	BatchID int64
	Amount  decimal.Decimal
}

type fundBatchConsumedAllocationSnapshot struct {
	BatchID int64
	Amount  decimal.Decimal
}

func applyFundBatchLedgerEffects(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	input BalanceLedgerApplyInput,
	transactionID int64,
	actualBalanceDelta decimal.Decimal,
	createdAt time.Time,
) error {
	if input.SkipFundBatchEffects || !actualBalanceDelta.IsPositive() && !actualBalanceDelta.IsNegative() {
		return nil
	}
	if actualBalanceDelta.IsPositive() {
		restoreKey := withdrawableRestoreSourceKey(input)
		if restoreKey != "" {
			consumed, err := selectFundConsumedAllocationsForRestore(ctx, runner, input.UserID, restoreKey)
			if err != nil {
				return fmt.Errorf("select fund restore allocations: %w", err)
			}
			if len(consumed) > 0 {
				return applyFundBatchRestore(ctx, runner, input.UserID, transactionID, actualBalanceDelta, consumed, input.SourceType, input.SourceID, restoreKey, createdAt)
			}
		}
		policy := classifyFundBatchGrant(input.SourceType, input.SourceID, input.Metadata)
		if !policy.Eligible {
			return nil
		}
		return insertFundBatchGrant(ctx, runner, input.UserID, transactionID, policy, actualBalanceDelta, input.SourceType, input.SourceID, input.Metadata, createdAt)
	}
	batches, err := selectConsumableFundBatches(ctx, runner, input)
	if err != nil {
		return fmt.Errorf("select consumable fund batches: %w", err)
	}
	plan := planFundBatchConsumption(actualBalanceDelta.Neg(), batches)
	return applyFundBatchConsumption(ctx, runner, input.UserID, transactionID, plan, input.SourceType, input.SourceID, createdAt)
}

func classifyFundBatchGrant(sourceType string, sourceID string, metadata map[string]any) fundBatchGrantPolicy {
	source := strings.ToLower(strings.TrimSpace(sourceType))
	switch source {
	case BalanceFlowTypePaymentRecharge, "payment_balance", "recharge":
		return fundBatchGrantPolicy{
			Eligible:       true,
			SourceKind:     FundSourceKindOnlineRecharge,
			Refundable:     true,
			PaymentOrderID: fundPaymentOrderID(sourceID, metadata),
		}
	case FundLedgerSourceOfflineRecharge:
		return fundBatchGrantPolicy{Eligible: true, SourceKind: FundSourceKindOfflineRecharge, Refundable: true}
	case FundLedgerSourceSignupGift, "auth_first_bind_grant":
		return fundBatchGrantPolicy{Eligible: true, SourceKind: FundSourceKindSignupGift}
	case FundLedgerSourceOpsGift, "admin_balance", "admin_adjustment":
		return fundBatchGrantPolicy{Eligible: true, SourceKind: FundSourceKindOpsGift}
	case FundLedgerSourceCompensation:
		return fundBatchGrantPolicy{Eligible: true, SourceKind: FundSourceKindCompensation}
	case "balance", "redeem", "redeem_code":
		return fundBatchGrantPolicy{Eligible: true, SourceKind: FundSourceKindRedeemGift}
	case BalanceFlowTypePromoBonus, "promo_code", "promotion":
		return fundBatchGrantPolicy{Eligible: true, SourceKind: FundSourceKindPromotionGift}
	default:
		return fundBatchGrantPolicy{}
	}
}

func fundPaymentOrderID(sourceID string, metadata map[string]any) *int64 {
	for _, raw := range []string{strings.TrimSpace(sourceID), strings.TrimSpace(fmt.Sprint(metadata["order_id"]))} {
		if raw == "" || raw == "<nil>" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 {
			return &id
		}
	}
	return nil
}

func insertFundBatchGrant(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	userID int64,
	transactionID int64,
	policy fundBatchGrantPolicy,
	amount decimal.Decimal,
	sourceType string,
	sourceID string,
	metadata map[string]any,
	createdAt time.Time,
) error {
	if !amount.IsPositive() {
		return nil
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return ErrBalanceLedgerInvalidInput.WithCause(err)
	}
	var batchID int64
	if err := queryOneBalanceLedger(ctx, runner, `
INSERT INTO balance_fund_batches (
	user_id,
	balance_transaction_id,
	payment_order_id,
	source_kind,
	source_type,
	source_id,
	original_amount,
	remaining_amount,
	consumed_amount,
	refunded_amount,
	refund_frozen_amount,
	refundable,
	available_at,
	status,
	metadata,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $7, 0, 0, 0, $8, $9, 'active', $10::jsonb, $11, NOW()
)
ON CONFLICT (user_id, balance_transaction_id) WHERE balance_transaction_id IS NOT NULL DO UPDATE
SET updated_at = balance_fund_batches.updated_at
RETURNING id`, []any{
		userID,
		transactionID,
		nullableInt64Ptr(policy.PaymentOrderID),
		policy.SourceKind,
		sourceType,
		sourceID,
		decimalString(amount),
		policy.Refundable,
		createdAt.UTC(),
		string(raw),
		createdAt.UTC(),
	}, &batchID); err != nil {
		return fmt.Errorf("insert fund batch: %w", err)
	}
	return insertFundBatchAllocation(ctx, runner, userID, batchID, transactionID, fundAllocationGrant, amount, sourceType, sourceID, nil, createdAt)
}

func selectConsumableFundBatches(ctx context.Context, runner balanceLedgerSQLRunner, input BalanceLedgerApplyInput) ([]fundBatchSnapshot, error) {
	refundPriority := strings.EqualFold(strings.TrimSpace(input.SourceType), BalanceFlowTypeRefund)
	paymentOrderID := int64(0)
	if refundPriority {
		if id := fundPaymentOrderID(input.SourceID, input.Metadata); id != nil {
			paymentOrderID = *id
		}
	}
	rows, err := runner.QueryContext(ctx, `
SELECT id, source_kind, remaining_amount::text
FROM balance_fund_batches
WHERE user_id = $1
  AND status = 'active'
  AND remaining_amount > 0
ORDER BY
  CASE
    WHEN $3::boolean AND $2::bigint > 0 AND payment_order_id = $2::bigint AND source_kind = 'online_recharge' THEN 1
    WHEN $3::boolean AND source_kind = 'online_recharge' THEN 2
    WHEN $3::boolean AND source_kind = 'offline_recharge' THEN 3
    WHEN source_kind IN ('signup_gift', 'ops_gift', 'compensation', 'redeem_gift', 'promotion_gift', 'unknown') THEN
      CASE WHEN $3::boolean THEN 4 ELSE 1 END
    WHEN source_kind IN ('online_recharge', 'offline_recharge') THEN
      CASE WHEN $3::boolean THEN 5 ELSE 2 END
    ELSE 6
  END,
  available_at ASC,
  id ASC
FOR UPDATE`, input.UserID, paymentOrderID, refundPriority)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []fundBatchSnapshot
	for rows.Next() {
		var item fundBatchSnapshot
		var amountRaw string
		if err := rows.Scan(&item.ID, &item.Kind, &amountRaw); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(amountRaw, "fund batch remaining")
		if err != nil {
			return nil, err
		}
		item.Remaining = amount
		out = append(out, item)
	}
	return out, rows.Err()
}

func planFundBatchConsumption(debit decimal.Decimal, batches []fundBatchSnapshot) []fundBatchAllocationPlan {
	remaining := clampDecimalScale(debit)
	if !remaining.IsPositive() {
		return nil
	}
	plans := make([]fundBatchAllocationPlan, 0)
	for _, batch := range batches {
		if !remaining.IsPositive() {
			break
		}
		if !batch.Remaining.IsPositive() {
			continue
		}
		amount := decimalMin(batch.Remaining, remaining)
		amount = clampDecimalScale(amount)
		plans = append(plans, fundBatchAllocationPlan{BatchID: batch.ID, Amount: amount})
		remaining = remaining.Sub(amount)
	}
	return plans
}

func applyFundBatchConsumption(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, transactionID int64, plan []fundBatchAllocationPlan, sourceType string, sourceID string, createdAt time.Time) error {
	for _, allocation := range plan {
		if !allocation.Amount.IsPositive() {
			continue
		}
		res, err := runner.ExecContext(ctx, `
UPDATE balance_fund_batches
SET remaining_amount = remaining_amount - $1::numeric,
    consumed_amount = consumed_amount + $1::numeric,
    status = CASE WHEN remaining_amount - $1::numeric <= 0 THEN 'consumed' ELSE status END,
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND status = 'active'
  AND remaining_amount >= $1::numeric`, decimalString(allocation.Amount), allocation.BatchID, userID)
		if err != nil {
			return fmt.Errorf("consume fund batch: %w", err)
		}
		if err := requireRowsAffected(res, "consume fund batch"); err != nil {
			return err
		}
		if err := insertFundBatchAllocation(ctx, runner, userID, allocation.BatchID, transactionID, fundAllocationConsume, allocation.Amount, sourceType, sourceID, nil, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func selectFundConsumedAllocationsForRestore(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, originalKey string) ([]fundBatchConsumedAllocationSnapshot, error) {
	originalKey = strings.TrimSpace(originalKey)
	if originalKey == "" {
		return nil, nil
	}
	rows, err := runner.QueryContext(ctx, `
WITH consumed AS (
	SELECT bfa.batch_id, SUM(bfa.amount) AS amount
	FROM balance_fund_allocations bfa
	JOIN balance_transactions bt ON bt.id = bfa.balance_transaction_id
	WHERE bfa.user_id = $1
	  AND bfa.action = 'consume'
	  AND bt.idempotency_key = $2
	GROUP BY bfa.batch_id
),
restored AS (
	SELECT batch_id, SUM(amount) AS amount
	FROM balance_fund_allocations
	WHERE user_id = $1
	  AND action = 'restore'
	  AND metadata->>'restored_from_idempotency_key' = $2
	GROUP BY batch_id
)
SELECT c.batch_id, (c.amount - COALESCE(r.amount, 0))::text AS restorable_amount
FROM consumed c
JOIN balance_fund_batches bfb ON bfb.id = c.batch_id
LEFT JOIN restored r ON r.batch_id = c.batch_id
WHERE c.amount > COALESCE(r.amount, 0)
ORDER BY bfb.available_at ASC, bfb.id ASC
FOR UPDATE OF bfb`, userID, originalKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []fundBatchConsumedAllocationSnapshot
	for rows.Next() {
		var item fundBatchConsumedAllocationSnapshot
		var amountRaw string
		if err := rows.Scan(&item.BatchID, &amountRaw); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(amountRaw, "fund consumed allocation")
		if err != nil {
			return nil, err
		}
		item.Amount = amount
		out = append(out, item)
	}
	return out, rows.Err()
}

func applyFundBatchRestore(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, transactionID int64, amount decimal.Decimal, consumed []fundBatchConsumedAllocationSnapshot, sourceType string, sourceID string, restoreFromKey string, createdAt time.Time) error {
	remaining := clampDecimalScale(amount)
	for _, allocation := range consumed {
		if !remaining.IsPositive() {
			break
		}
		restoreAmount := decimalMin(allocation.Amount, remaining)
		restoreAmount = clampDecimalScale(restoreAmount)
		if !restoreAmount.IsPositive() {
			continue
		}
		res, err := runner.ExecContext(ctx, `
UPDATE balance_fund_batches
SET remaining_amount = remaining_amount + $1::numeric,
    consumed_amount = consumed_amount - $1::numeric,
    status = 'active',
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND consumed_amount >= $1::numeric`, decimalString(restoreAmount), allocation.BatchID, userID)
		if err != nil {
			return fmt.Errorf("restore fund batch: %w", err)
		}
		if err := requireRowsAffected(res, "restore fund batch"); err != nil {
			return err
		}
		metadata := map[string]any{"restored_from_idempotency_key": restoreFromKey}
		if err := insertFundBatchAllocation(ctx, runner, userID, allocation.BatchID, transactionID, fundAllocationRestore, restoreAmount, sourceType, sourceID, metadata, createdAt); err != nil {
			return err
		}
		remaining = remaining.Sub(restoreAmount)
	}
	return nil
}

func insertFundBatchAllocation(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	userID int64,
	batchID int64,
	transactionID int64,
	action string,
	amount decimal.Decimal,
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
INSERT INTO balance_fund_allocations (
	user_id,
	batch_id,
	balance_transaction_id,
	action,
	amount,
	source_type,
	source_id,
	metadata,
	created_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9
)`, userID, batchID, nullableInt64(transactionID), action, decimalString(amount), sourceType, sourceID, string(raw), createdAt.UTC())
	if err != nil {
		return fmt.Errorf("insert fund allocation: %w", err)
	}
	return nil
}

func nullableInt64Ptr(value *int64) any {
	if value == nil || *value <= 0 {
		return nil
	}
	return *value
}
