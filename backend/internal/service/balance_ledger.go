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
	UserID         int64
	BalanceDelta   float64
	FrozenDelta    float64
	SourceType     string
	SourceID       string
	IdempotencyKey string
	ActorType      string
	ActorUserID    *int64
	Description    string
	Metadata       map[string]any
	IsBackfilled   bool
	Confidence     string
	BalancePolicy  string
	FrozenPolicy   string
	CreatedAt      *time.Time
}

type BalanceTransaction struct {
	ID             int64          `json:"id"`
	UserID         int64          `json:"user_id"`
	BalanceDelta   float64        `json:"balance_delta"`
	BalanceBefore  *float64       `json:"balance_before,omitempty"`
	BalanceAfter   *float64       `json:"balance_after,omitempty"`
	FrozenDelta    float64        `json:"frozen_delta"`
	FrozenBefore   *float64       `json:"frozen_before,omitempty"`
	FrozenAfter    *float64       `json:"frozen_after,omitempty"`
	SourceType     string         `json:"source_type"`
	SourceID       string         `json:"source_id"`
	IdempotencyKey string         `json:"idempotency_key"`
	ActorType      string         `json:"actor_type"`
	ActorUserID    *int64         `json:"actor_user_id,omitempty"`
	Description    string         `json:"description"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IsBackfilled   bool           `json:"is_backfilled"`
	Confidence     string         `json:"confidence"`
	CreatedAt      time.Time      `json:"created_at"`
}

func NewBalanceLedgerService(db *sql.DB, authCacheInvalidator APIKeyAuthCacheInvalidator, balanceCacheInvalidator *BillingCacheService) *BalanceLedgerService {
	return &BalanceLedgerService{
		db:                      db,
		authCacheInvalidator:    authCacheInvalidator,
		balanceCacheInvalidator: balanceCacheInvalidator,
		now:                     time.Now,
	}
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

	if normalized.BalanceDelta != 0 || normalized.FrozenDelta != 0 {
		s.invalidateBalanceCachesAfterCommit(ctx, normalized.UserID)
	}
	return transaction, nil
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

	var beforeBalance, beforeFrozen float64
	if err := queryOneBalanceLedger(ctx, runner, `
SELECT balance::double precision, COALESCE(frozen_balance, 0)::double precision
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, []any{normalized.UserID}, &beforeBalance, &beforeFrozen); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrUserNotFound
		}
		return nil, false, fmt.Errorf("lock user balance: %w", err)
	}

	afterBalance, actualBalanceDelta, err := applyBalanceLedgerPolicy(beforeBalance, normalized.BalanceDelta, normalized.BalancePolicy)
	if err != nil {
		return nil, false, err
	}
	afterFrozen, actualFrozenDelta, err := applyBalanceLedgerPolicy(beforeFrozen, normalized.FrozenDelta, normalized.FrozenPolicy)
	if err != nil {
		return nil, false, err
	}
	normalized.BalanceDelta = actualBalanceDelta
	normalized.FrozenDelta = actualFrozenDelta

	if _, err := runner.ExecContext(ctx, `
UPDATE users
SET balance = $1, frozen_balance = $2, updated_at = NOW()
WHERE id = $3 AND deleted_at IS NULL`, afterBalance, afterFrozen, normalized.UserID); err != nil {
		return nil, false, fmt.Errorf("update user balance: %w", err)
	}

	createdAt := s.now().UTC()
	if normalized.CreatedAt != nil {
		createdAt = normalized.CreatedAt.UTC()
	}
	transaction, err := insertBalanceTransaction(ctx, runner, normalized, beforeBalance, afterBalance, beforeFrozen, afterFrozen, createdAt)
	if err != nil {
		return nil, false, err
	}
	return transaction, normalized.BalanceDelta != 0 || normalized.FrozenDelta != 0, nil
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
	beforeBalance, afterBalance, beforeFrozen, afterFrozen float64,
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
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15, $16, $17
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
		input.BalanceDelta,
		beforeBalance,
		afterBalance,
		input.FrozenDelta,
		beforeFrozen,
		afterFrozen,
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
		transaction                             BalanceTransaction
		balanceBefore, balanceAfter             sql.NullFloat64
		frozenBefore, frozenAfter               sql.NullFloat64
		actorUserID                             sql.NullInt64
		sourceID, actorType, description        sql.NullString
		idempotencyKey, metadataRaw, confidence sql.NullString
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
