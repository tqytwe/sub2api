package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/shopspring/decimal"
)

const (
	BalanceLedgerPolicyRejectNegative = "reject_negative"
	BalanceLedgerPolicyAllowOverdraft = "allow_overdraft"
	BalanceLedgerPolicyClampZero      = "clamp_zero"

	BalanceLedgerActorSystem = "system"
	BalanceLedgerActorAdmin  = "admin"
	BalanceLedgerActorUser   = "user"

	BalanceLedgerConfidenceHigh        = "high"
	BalanceLedgerConfidenceEstimated   = "estimated"
	BalanceLedgerConfidenceNeedsReview = "needs_review"
)

var (
	ErrBalanceLedgerUnavailable         = infraerrors.InternalServer("BALANCE_LEDGER_UNAVAILABLE", "balance ledger service is unavailable")
	ErrBalanceLedgerInvalidInput        = infraerrors.BadRequest("BALANCE_LEDGER_INVALID_INPUT", "invalid balance ledger input")
	ErrBalanceLedgerInsufficientBalance = infraerrors.BadRequest("BALANCE_LEDGER_INSUFFICIENT_BALANCE", "balance ledger operation would make balance negative")
	ErrBalanceLedgerIdempotencyConflict = infraerrors.Conflict("BALANCE_LEDGER_IDEMPOTENCY_CONFLICT", "idempotency key has already been used by another balance operation")
)

type balanceLedgerCacheInvalidator interface {
	InvalidateUserBalance(ctx context.Context, userID int64) error
}

type balanceLedgerSQLRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type BalanceLedgerService struct {
	db                      *sql.DB
	authCacheInvalidator    APIKeyAuthCacheInvalidator
	balanceCacheInvalidator balanceLedgerCacheInvalidator
	now                     func() time.Time
}

type BalanceLedgerApplyInput struct {
	UserID                 int64
	BalanceDelta           float64
	FrozenDelta            float64
	WithdrawableDelta      float64
	WithdrawalFrozenDelta  float64
	SourceType             string
	SourceID               string
	IdempotencyKey         string
	ActorType              string
	ActorUserID            *int64
	Description            string
	Metadata               map[string]any
	IsBackfilled           bool
	Confidence             string
	BalancePolicy          string
	FrozenPolicy           string
	WithdrawablePolicy     string
	WithdrawalFrozenPolicy string
	CreatedAt              *time.Time
	// SkipWithdrawableEntitlementEffects lets higher-level workflows that
	// manage entitlement batches themselves record the user-balance deltas
	// without also consuming/restoring entitlement rows. CP5 withdrawals use it
	// to freeze exact mature batches instead of treating a pending withdrawal as
	// a normal spend.
	SkipWithdrawableEntitlementEffects bool
	// SkipFundBatchEffects lets refund workflows lock exact recharge batches
	// themselves instead of letting the generic spend priority consume gift or
	// recharge batches automatically.
	SkipFundBatchEffects bool
}

type BalanceTransaction struct {
	ID                     int64          `json:"id"`
	UserID                 int64          `json:"user_id"`
	BalanceDelta           float64        `json:"balance_delta"`
	BalanceBefore          *float64       `json:"balance_before,omitempty"`
	BalanceAfter           *float64       `json:"balance_after,omitempty"`
	FrozenDelta            float64        `json:"frozen_delta"`
	FrozenBefore           *float64       `json:"frozen_before,omitempty"`
	FrozenAfter            *float64       `json:"frozen_after,omitempty"`
	WithdrawableDelta      float64        `json:"withdrawable_delta"`
	WithdrawableBefore     *float64       `json:"withdrawable_before,omitempty"`
	WithdrawableAfter      *float64       `json:"withdrawable_after,omitempty"`
	WithdrawalFrozenDelta  float64        `json:"withdrawal_frozen_delta"`
	WithdrawalFrozenBefore *float64       `json:"withdrawal_frozen_before,omitempty"`
	WithdrawalFrozenAfter  *float64       `json:"withdrawal_frozen_after,omitempty"`
	SourceType             string         `json:"source_type"`
	SourceID               string         `json:"source_id"`
	IdempotencyKey         string         `json:"idempotency_key"`
	ActorType              string         `json:"actor_type"`
	ActorUserID            *int64         `json:"actor_user_id,omitempty"`
	Description            string         `json:"description"`
	Metadata               map[string]any `json:"metadata,omitempty"`
	IsBackfilled           bool           `json:"is_backfilled"`
	Confidence             string         `json:"confidence"`
	CreatedAt              time.Time      `json:"created_at"`
}

type balanceLedgerUserState struct {
	Balance          decimal.Decimal
	Frozen           decimal.Decimal
	Withdrawable     decimal.Decimal
	WithdrawalFrozen decimal.Decimal
}

func NewBalanceLedgerService(db *sql.DB, authCacheInvalidator APIKeyAuthCacheInvalidator, balanceCacheInvalidator *BillingCacheService) *BalanceLedgerService {
	svc := &BalanceLedgerService{
		db:                   db,
		authCacheInvalidator: authCacheInvalidator,
		now:                  time.Now,
	}
	if balanceCacheInvalidator != nil {
		svc.balanceCacheInvalidator = balanceCacheInvalidator
	}
	return svc
}

func (s *BalanceLedgerService) ApplyDelta(ctx context.Context, input BalanceLedgerApplyInput) (*BalanceTransaction, error) {
	if s == nil || s.db == nil {
		return nil, ErrBalanceLedgerUnavailable
	}
	normalized, err := normalizeBalanceLedgerInput(input)
	if err != nil {
		return nil, err
	}

	if entTx := dbent.TxFromContext(ctx); entTx != nil {
		transaction, changed, err := s.applyDeltaWithRunner(ctx, entTx.Client(), normalized)
		if err != nil {
			return nil, err
		}
		if changed {
			entTx.OnCommit(func(next dbent.Committer) dbent.Committer {
				return dbent.CommitFunc(func(commitCtx context.Context, tx *dbent.Tx) error {
					if err := next.Commit(commitCtx, tx); err != nil {
						return err
					}
					s.invalidateBalanceCachesAfterCommit(ctx, normalized.UserID)
					return nil
				})
			})
		}
		return transaction, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin balance ledger tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	transaction, changed, err := s.applyDeltaWithRunner(ctx, tx, normalized)
	if err != nil {
		return nil, err
	}
	if !changed {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit balance ledger replay tx: %w", err)
		}
		committed = true
		return transaction, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit balance ledger tx: %w", err)
	}
	committed = true

	if changed {
		s.invalidateBalanceCachesAfterCommit(ctx, normalized.UserID)
	}
	return transaction, nil
}

func (s *BalanceLedgerService) ApplyDeltaInSQLTx(ctx context.Context, tx *sql.Tx, input BalanceLedgerApplyInput) (*BalanceTransaction, error) {
	if s == nil || tx == nil {
		return nil, ErrBalanceLedgerUnavailable
	}
	normalized, err := normalizeBalanceLedgerInput(input)
	if err != nil {
		return nil, err
	}
	transaction, _, err := s.applyDeltaWithRunner(ctx, tx, normalized)
	return transaction, err
}

func (s *BalanceLedgerService) InvalidateUserBalanceCaches(ctx context.Context, userID int64) {
	if s == nil || userID <= 0 {
		return
	}
	s.invalidateBalanceCachesAfterCommit(ctx, userID)
}

func (s *BalanceLedgerService) applyDeltaWithRunner(ctx context.Context, runner balanceLedgerSQLRunner, normalized BalanceLedgerApplyInput) (*BalanceTransaction, bool, error) {
	existing, err := selectBalanceTransactionForIdempotency(ctx, runner, normalized.UserID, normalized.IdempotencyKey)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if existing.SourceType != normalized.SourceType || existing.SourceID != normalized.SourceID {
			return nil, false, ErrBalanceLedgerIdempotencyConflict
		}
		return existing, false, nil
	}

	nowUTC := s.now().UTC()
	createdAt := nowUTC
	if normalized.CreatedAt != nil {
		createdAt = normalized.CreatedAt.UTC()
	}
	before, err := selectBalanceLedgerUserState(ctx, runner, normalized.UserID)
	if err != nil {
		return nil, false, err
	}
	if sums, sumErr := selectWithdrawableEntitlementSums(ctx, runner, normalized.UserID, nowUTC); sumErr != nil {
		return nil, false, fmt.Errorf("sync withdrawable entitlement maturity: %w", sumErr)
	} else {
		before.Withdrawable = decimalMin(before.Balance, decimalMax(decimal.Zero, sums.Mature))
	}

	balanceDelta := decimalFromLedgerFloat(normalized.BalanceDelta)
	frozenDelta := decimalFromLedgerFloat(normalized.FrozenDelta)
	withdrawalFrozenDelta := decimalFromLedgerFloat(normalized.WithdrawalFrozenDelta)

	afterBalance, actualBalanceDelta, err := applyBalanceLedgerPolicyDecimal(before.Balance, balanceDelta, normalized.BalancePolicy)
	if err != nil {
		return nil, false, err
	}
	afterFrozen, actualFrozenDelta, err := applyBalanceLedgerPolicyDecimal(before.Frozen, frozenDelta, normalized.FrozenPolicy)
	if err != nil {
		return nil, false, err
	}
	afterWithdrawalFrozen, actualWithdrawalFrozenDelta, err := applyBalanceLedgerPolicyDecimal(before.WithdrawalFrozen, withdrawalFrozenDelta, normalized.WithdrawalFrozenPolicy)
	if err != nil {
		return nil, false, err
	}
	normalized.BalanceDelta = decimalToLedgerFloat(actualBalanceDelta)
	normalized.FrozenDelta = decimalToLedgerFloat(actualFrozenDelta)
	normalized.WithdrawalFrozenDelta = decimalToLedgerFloat(actualWithdrawalFrozenDelta)

	effects, err := planWithdrawableLedgerEffects(ctx, runner, normalized, before, actualBalanceDelta, createdAt, nowUTC)
	if err != nil {
		return nil, false, err
	}
	inputWithdrawableDelta := decimalFromLedgerFloat(normalized.WithdrawableDelta)
	totalWithdrawableDelta := inputWithdrawableDelta.Add(effects.WithdrawableDelta)
	afterWithdrawable, actualWithdrawableDelta, err := applyBalanceLedgerPolicyDecimal(before.Withdrawable, totalWithdrawableDelta, normalized.WithdrawablePolicy)
	if err != nil {
		return nil, false, err
	}
	maxWithdrawable := decimalMax(decimal.Zero, afterBalance)
	if afterWithdrawable.GreaterThan(maxWithdrawable.Add(decimal.RequireFromString("0.00000001"))) {
		return nil, false, ErrBalanceLedgerInvalidInput.WithMetadata(map[string]string{"field": "withdrawable_balance"})
	}
	normalized.WithdrawableDelta = decimalToLedgerFloat(actualWithdrawableDelta)

	if _, err := runner.ExecContext(ctx, `
UPDATE users
SET balance = $1,
    frozen_balance = $2,
    withdrawable_balance = $3,
    withdrawal_frozen_balance = $4,
    updated_at = NOW()
WHERE id = $5 AND deleted_at IS NULL`,
		decimalString(afterBalance),
		decimalString(afterFrozen),
		decimalString(afterWithdrawable),
		decimalString(afterWithdrawalFrozen),
		normalized.UserID,
	); err != nil {
		return nil, false, fmt.Errorf("update user balance: %w", err)
	}

	transaction, err := insertBalanceTransaction(ctx, runner, normalized, before, balanceLedgerUserState{
		Balance:          afterBalance,
		Frozen:           afterFrozen,
		Withdrawable:     afterWithdrawable,
		WithdrawalFrozen: afterWithdrawalFrozen,
	}, createdAt)
	if err != nil {
		return nil, false, err
	}
	if err := applyWithdrawableLedgerEffects(ctx, runner, normalized, transaction.ID, effects, createdAt); err != nil {
		return nil, false, err
	}
	if err := applyFundBatchLedgerEffects(ctx, runner, normalized, transaction.ID, actualBalanceDelta, createdAt); err != nil {
		return nil, false, err
	}
	changed := normalized.BalanceDelta != 0 ||
		normalized.FrozenDelta != 0 ||
		normalized.WithdrawableDelta != 0 ||
		normalized.WithdrawalFrozenDelta != 0
	return transaction, changed, nil
}

func normalizeBalanceLedgerInput(input BalanceLedgerApplyInput) (BalanceLedgerApplyInput, error) {
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ActorType = strings.TrimSpace(input.ActorType)
	input.Description = strings.TrimSpace(input.Description)
	input.Confidence = strings.TrimSpace(input.Confidence)
	input.BalancePolicy = strings.TrimSpace(input.BalancePolicy)
	input.FrozenPolicy = strings.TrimSpace(input.FrozenPolicy)
	input.WithdrawablePolicy = strings.TrimSpace(input.WithdrawablePolicy)
	input.WithdrawalFrozenPolicy = strings.TrimSpace(input.WithdrawalFrozenPolicy)
	if input.UserID <= 0 || input.SourceType == "" || input.IdempotencyKey == "" {
		return BalanceLedgerApplyInput{}, ErrBalanceLedgerInvalidInput
	}
	if input.ActorType == "" {
		input.ActorType = BalanceLedgerActorSystem
	}
	if input.Confidence == "" {
		input.Confidence = BalanceLedgerConfidenceHigh
	}
	if input.BalancePolicy == "" {
		input.BalancePolicy = BalanceLedgerPolicyRejectNegative
	}
	if input.FrozenPolicy == "" {
		input.FrozenPolicy = BalanceLedgerPolicyRejectNegative
	}
	if input.WithdrawablePolicy == "" {
		input.WithdrawablePolicy = BalanceLedgerPolicyRejectNegative
	}
	if input.WithdrawalFrozenPolicy == "" {
		input.WithdrawalFrozenPolicy = BalanceLedgerPolicyRejectNegative
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	return input, nil
}

func applyBalanceLedgerPolicy(before, delta float64, policy string) (float64, float64, error) {
	after := before + delta
	switch policy {
	case BalanceLedgerPolicyAllowOverdraft:
		return after, delta, nil
	case BalanceLedgerPolicyClampZero:
		if after < 0 {
			return 0, -before, nil
		}
		return after, delta, nil
	case BalanceLedgerPolicyRejectNegative:
		if after < -0.00000001 {
			return 0, 0, ErrBalanceLedgerInsufficientBalance
		}
		if math.Abs(after) < 0.00000001 {
			after = 0
		}
		return after, delta, nil
	default:
		return 0, 0, ErrBalanceLedgerInvalidInput
	}
}

func applyBalanceLedgerPolicyDecimal(before, delta decimal.Decimal, policy string) (decimal.Decimal, decimal.Decimal, error) {
	before = clampDecimalScale(before)
	delta = clampDecimalScale(delta)
	after := clampDecimalScale(before.Add(delta))
	switch policy {
	case BalanceLedgerPolicyAllowOverdraft:
		return after, delta, nil
	case BalanceLedgerPolicyClampZero:
		if after.IsNegative() {
			return decimal.Zero, before.Neg(), nil
		}
		return after, delta, nil
	case BalanceLedgerPolicyRejectNegative:
		if after.LessThan(decimal.RequireFromString("-0.00000001")) {
			return decimal.Zero, decimal.Zero, ErrBalanceLedgerInsufficientBalance
		}
		if after.Abs().LessThan(decimal.RequireFromString("0.00000001")) {
			after = decimal.Zero
		}
		return after, delta, nil
	default:
		return decimal.Zero, decimal.Zero, ErrBalanceLedgerInvalidInput
	}
}

func selectBalanceLedgerUserState(ctx context.Context, runner balanceLedgerSQLRunner, userID int64) (balanceLedgerUserState, error) {
	var balanceRaw, frozenRaw, withdrawableRaw, withdrawalFrozenRaw string
	if err := queryOneBalanceLedger(ctx, runner, `
SELECT
	balance::text,
	COALESCE(frozen_balance, 0)::text,
	COALESCE(withdrawable_balance, 0)::text,
	COALESCE(withdrawal_frozen_balance, 0)::text
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, []any{userID}, &balanceRaw, &frozenRaw, &withdrawableRaw, &withdrawalFrozenRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return balanceLedgerUserState{}, ErrUserNotFound
		}
		return balanceLedgerUserState{}, fmt.Errorf("lock user balance: %w", err)
	}
	balance, err := parseLedgerDecimal(balanceRaw, "balance")
	if err != nil {
		return balanceLedgerUserState{}, err
	}
	frozen, err := parseLedgerDecimal(frozenRaw, "frozen balance")
	if err != nil {
		return balanceLedgerUserState{}, err
	}
	withdrawable, err := parseLedgerDecimal(withdrawableRaw, "withdrawable balance")
	if err != nil {
		return balanceLedgerUserState{}, err
	}
	withdrawalFrozen, err := parseLedgerDecimal(withdrawalFrozenRaw, "withdrawal frozen balance")
	if err != nil {
		return balanceLedgerUserState{}, err
	}
	return balanceLedgerUserState{
		Balance:          balance,
		Frozen:           frozen,
		Withdrawable:     withdrawable,
		WithdrawalFrozen: withdrawalFrozen,
	}, nil
}

func planWithdrawableLedgerEffects(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	input BalanceLedgerApplyInput,
	before balanceLedgerUserState,
	actualBalanceDelta decimal.Decimal,
	createdAt time.Time,
	asOf time.Time,
) (withdrawableLedgerEffects, error) {
	effects := withdrawableLedgerEffects{}
	if input.SkipWithdrawableEntitlementEffects {
		return effects, nil
	}
	if actualBalanceDelta.IsPositive() {
		restoreKey := withdrawableRestoreSourceKey(input)
		if restoreKey != "" {
			consumed, err := selectConsumedAllocationsForRestore(ctx, runner, input.UserID, restoreKey)
			if err != nil {
				return effects, fmt.Errorf("select withdrawable restore allocations: %w", err)
			}
			restore := planWithdrawableRestore(actualBalanceDelta, consumed, asOf)
			effects.Restore = restore
			effects.RestoreFromKey = restoreKey
			effects.WithdrawableDelta = effects.WithdrawableDelta.Add(restore.MatureWithdrawableAmount)
			return effects, nil
		}

		policy := classifyWithdrawableGrant(input.SourceType, createdAt)
		if policy.Eligible {
			grant := withdrawableGrantPlan{
				Amount:      actualBalanceDelta,
				AvailableAt: policy.AvailableAt,
				SourceType:  input.SourceType,
				SourceID:    input.SourceID,
			}
			effects.Grant = &grant
			if !policy.AvailableAt.After(asOf) {
				effects.WithdrawableDelta = effects.WithdrawableDelta.Add(actualBalanceDelta)
			}
		}
		return effects, nil
	}

	if actualBalanceDelta.IsNegative() {
		entitlements, err := selectWithdrawableEntitlementSnapshots(ctx, runner, input.UserID)
		if err != nil {
			return effects, fmt.Errorf("select withdrawable entitlements: %w", err)
		}
		consume := planWithdrawableConsumption(before.Balance, actualBalanceDelta.Neg(), entitlements, asOf)
		effects.Consume = consume
		effects.WithdrawableDelta = effects.WithdrawableDelta.Sub(consume.MatureWithdrawableAmount)
	}
	return effects, nil
}

func applyWithdrawableLedgerEffects(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	input BalanceLedgerApplyInput,
	transactionID int64,
	effects withdrawableLedgerEffects,
	createdAt time.Time,
) error {
	if effects.Grant != nil {
		if err := insertWithdrawableGrant(ctx, runner, input.UserID, transactionID, *effects.Grant, createdAt); err != nil {
			return err
		}
	}
	if len(effects.Consume.Allocations) > 0 {
		if err := applyWithdrawableConsumption(ctx, runner, input.UserID, transactionID, effects.Consume, input.SourceType, input.SourceID, createdAt); err != nil {
			return err
		}
	}
	if len(effects.Restore.Allocations) > 0 {
		if err := applyWithdrawableRestore(ctx, runner, input.UserID, transactionID, effects.Restore, input.SourceType, input.SourceID, effects.RestoreFromKey, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func selectBalanceTransactionForIdempotency(ctx context.Context, runner balanceLedgerSQLRunner, userID int64, key string) (*BalanceTransaction, error) {
	transaction, err := queryBalanceTransaction(ctx, runner, `
SELECT
	id,
	user_id,
	balance_delta::double precision,
	balance_before::double precision,
	balance_after::double precision,
	frozen_delta::double precision,
	frozen_before::double precision,
	frozen_after::double precision,
	withdrawable_delta::double precision,
	withdrawable_before::double precision,
	withdrawable_after::double precision,
	withdrawal_frozen_delta::double precision,
	withdrawal_frozen_before::double precision,
	withdrawal_frozen_after::double precision,
	source_type,
	source_id,
	idempotency_key,
	actor_type,
	actor_user_id,
	description,
	metadata::text,
	is_backfilled,
	confidence,
	created_at
FROM balance_transactions
WHERE user_id = $1 AND idempotency_key = $2`, userID, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return transaction, nil
}

func insertBalanceTransaction(
	ctx context.Context,
	runner balanceLedgerSQLRunner,
	input BalanceLedgerApplyInput,
	before balanceLedgerUserState,
	after balanceLedgerUserState,
	createdAt time.Time,
) (*BalanceTransaction, error) {
	metadataRaw, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, ErrBalanceLedgerInvalidInput.WithCause(err)
	}
	transaction, err := queryBalanceTransaction(ctx, runner, `
INSERT INTO balance_transactions (
	user_id,
	balance_delta,
	balance_before,
	balance_after,
	frozen_delta,
	frozen_before,
	frozen_after,
	withdrawable_delta,
	withdrawable_before,
	withdrawable_after,
	withdrawal_frozen_delta,
	withdrawal_frozen_before,
	withdrawal_frozen_after,
	source_type,
	source_id,
	idempotency_key,
	actor_type,
	actor_user_id,
	description,
	metadata,
	is_backfilled,
	confidence,
	created_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20::jsonb, $21, $22, $23
)
RETURNING
	id,
	user_id,
	balance_delta::double precision,
	balance_before::double precision,
	balance_after::double precision,
	frozen_delta::double precision,
	frozen_before::double precision,
	frozen_after::double precision,
	withdrawable_delta::double precision,
	withdrawable_before::double precision,
	withdrawable_after::double precision,
	withdrawal_frozen_delta::double precision,
	withdrawal_frozen_before::double precision,
	withdrawal_frozen_after::double precision,
	source_type,
	source_id,
	idempotency_key,
	actor_type,
	actor_user_id,
	description,
	metadata::text,
	is_backfilled,
	confidence,
	created_at`,
		input.UserID,
		decimalString(decimalFromLedgerFloat(input.BalanceDelta)),
		decimalString(before.Balance),
		decimalString(after.Balance),
		decimalString(decimalFromLedgerFloat(input.FrozenDelta)),
		decimalString(before.Frozen),
		decimalString(after.Frozen),
		decimalString(decimalFromLedgerFloat(input.WithdrawableDelta)),
		decimalString(before.Withdrawable),
		decimalString(after.Withdrawable),
		decimalString(decimalFromLedgerFloat(input.WithdrawalFrozenDelta)),
		decimalString(before.WithdrawalFrozen),
		decimalString(after.WithdrawalFrozen),
		input.SourceType,
		input.SourceID,
		input.IdempotencyKey,
		input.ActorType,
		input.ActorUserID,
		input.Description,
		string(metadataRaw),
		input.IsBackfilled,
		input.Confidence,
		createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert balance transaction: %w", err)
	}
	return transaction, nil
}

func queryBalanceTransaction(ctx context.Context, runner balanceLedgerSQLRunner, query string, args ...any) (*BalanceTransaction, error) {
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	transaction, err := scanBalanceTransaction(rows.Scan)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, ErrBalanceLedgerInvalidInput
	}
	return transaction, rows.Err()
}

func queryOneBalanceLedger(ctx context.Context, runner balanceLedgerSQLRunner, query string, args []any, dest ...any) error {
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	if rows.Next() {
		return ErrBalanceLedgerInvalidInput
	}
	return rows.Err()
}

func scanBalanceTransaction(scan func(dest ...any) error) (*BalanceTransaction, error) {
	var (
		transaction                                   BalanceTransaction
		balanceBefore, balanceAfter                   sql.NullFloat64
		frozenBefore, frozenAfter                     sql.NullFloat64
		withdrawableBefore, withdrawableAfter         sql.NullFloat64
		withdrawalFrozenBefore, withdrawalFrozenAfter sql.NullFloat64
		actorUserID                                   sql.NullInt64
		sourceID, actorType, description              sql.NullString
		idempotencyKey, metadataRaw, confidence       sql.NullString
	)
	if err := scan(
		&transaction.ID,
		&transaction.UserID,
		&transaction.BalanceDelta,
		&balanceBefore,
		&balanceAfter,
		&transaction.FrozenDelta,
		&frozenBefore,
		&frozenAfter,
		&transaction.WithdrawableDelta,
		&withdrawableBefore,
		&withdrawableAfter,
		&transaction.WithdrawalFrozenDelta,
		&withdrawalFrozenBefore,
		&withdrawalFrozenAfter,
		&transaction.SourceType,
		&sourceID,
		&idempotencyKey,
		&actorType,
		&actorUserID,
		&description,
		&metadataRaw,
		&transaction.IsBackfilled,
		&confidence,
		&transaction.CreatedAt,
	); err != nil {
		return nil, err
	}
	transaction.BalanceBefore = float64PtrFromNull(balanceBefore)
	transaction.BalanceAfter = float64PtrFromNull(balanceAfter)
	transaction.FrozenBefore = float64PtrFromNull(frozenBefore)
	transaction.FrozenAfter = float64PtrFromNull(frozenAfter)
	transaction.WithdrawableBefore = float64PtrFromNull(withdrawableBefore)
	transaction.WithdrawableAfter = float64PtrFromNull(withdrawableAfter)
	transaction.WithdrawalFrozenBefore = float64PtrFromNull(withdrawalFrozenBefore)
	transaction.WithdrawalFrozenAfter = float64PtrFromNull(withdrawalFrozenAfter)
	transaction.SourceID = stringOrEmpty(sourceID)
	transaction.IdempotencyKey = stringOrEmpty(idempotencyKey)
	transaction.ActorType = stringOrDefault(actorType, BalanceLedgerActorSystem)
	transaction.ActorUserID = int64PtrFromNull(actorUserID)
	transaction.Description = stringOrEmpty(description)
	transaction.Confidence = stringOrDefault(confidence, BalanceLedgerConfidenceHigh)
	if metadataRaw.Valid && strings.TrimSpace(metadataRaw.String) != "" {
		var meta map[string]any
		if err := json.Unmarshal([]byte(metadataRaw.String), &meta); err == nil {
			transaction.Metadata = meta
		}
	}
	return &transaction, nil
}

func (s *BalanceLedgerService) invalidateBalanceCachesAfterCommit(ctx context.Context, userID int64) {
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}
	if s.balanceCacheInvalidator != nil {
		if err := s.balanceCacheInvalidator.InvalidateUserBalance(cacheCtx, userID); err != nil {
			logger.LegacyPrintf("service.balance_ledger", "invalidate user balance cache failed: user_id=%d err=%v", userID, err)
		}
	}
}
