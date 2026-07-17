package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

type usageBillingRepository struct {
	db *sql.DB
}

const imageStudioBillingReconciliationTimeout = 5 * time.Second

type usageBillingTransactionContextKey struct{}

func withUsageBillingTransaction(ctx context.Context, tx *sql.Tx) context.Context {
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, usageBillingTransactionContextKey{}, tx)
}

func usageBillingTransactionFromContext(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(usageBillingTransactionContextKey{}).(*sql.Tx)
	return tx
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	if cmd != nil {
		cmd.Normalize()
	}
	result, applyErr := r.applyUsageBillingTransaction(ctx, cmd)
	if applyErr == nil || cmd == nil || !service.IsImageStudioManagedBilling(ctx) {
		return result, applyErr
	}
	reconciliationCtx, cancel := detachedImageStudioBillingReconciliationContext(ctx)
	defer cancel()
	if reconciliationErr := r.persistImageStudioBillingReconciliation(reconciliationCtx, cmd, applyErr); reconciliationErr != nil {
		return nil, errors.Join(
			applyErr,
			fmt.Errorf("%w: %w", service.ErrImageStudioBillingReconciliationPersistence, reconciliationErr),
		)
	}
	return nil, applyErr
}

func detachedImageStudioBillingReconciliationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, imageStudioBillingReconciliationTimeout)
}

func (r *usageBillingRepository) applyUsageBillingTransaction(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	if cmd.UserID > 0 {
		if err := validateUsageBillingOwnership(ctx, tx, cmd.APIKeyID, cmd.UserID); err != nil {
			return nil, err
		}
	} else if cmd.BalanceCost > 0 || cmd.SubscriptionCost > 0 {
		return nil, service.ErrUsageBillingOwnershipMismatch
	}
	if cmd.SubscriptionCost > 0 {
		if cmd.SubscriptionID == nil || *cmd.SubscriptionID <= 0 {
			return nil, service.ErrUsageBillingOwnershipMismatch
		}
		if err := validateUsageBillingSubscriptionOwnership(ctx, tx, *cmd.SubscriptionID, cmd.UserID); err != nil {
			return nil, err
		}
	}

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

type imageStudioBillingReconciliationCommand struct {
	RequestID           string  `json:"request_id"`
	APIKeyID            int64   `json:"api_key_id"`
	UserID              int64   `json:"user_id"`
	AccountID           int64   `json:"account_id"`
	SubscriptionID      *int64  `json:"subscription_id,omitempty"`
	AccountType         string  `json:"account_type,omitempty"`
	Model               string  `json:"model,omitempty"`
	ServiceTier         string  `json:"service_tier,omitempty"`
	ReasoningEffort     string  `json:"reasoning_effort,omitempty"`
	BillingType         int8    `json:"billing_type"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	ImageCount          int     `json:"image_count"`
	MediaType           string  `json:"media_type,omitempty"`
	ActualCost          float64 `json:"actual_cost"`
	BalanceCost         float64 `json:"balance_cost"`
	SubscriptionCost    float64 `json:"subscription_cost"`
	APIKeyQuotaCost     float64 `json:"api_key_quota_cost"`
	APIKeyRateLimitCost float64 `json:"api_key_rate_limit_cost"`
	AccountQuotaCost    float64 `json:"account_quota_cost"`
	RequestPayloadHash  string  `json:"request_payload_hash,omitempty"`
	RequestFingerprint  string  `json:"request_fingerprint"`
}

func marshalImageStudioBillingReconciliationCommand(cmd *service.UsageBillingCommand) ([]byte, error) {
	if cmd == nil {
		return nil, errors.New("usage billing command is nil")
	}
	return json.Marshal(imageStudioBillingReconciliationCommand{
		RequestID:           cmd.RequestID,
		APIKeyID:            cmd.APIKeyID,
		UserID:              cmd.UserID,
		AccountID:           cmd.AccountID,
		SubscriptionID:      cmd.SubscriptionID,
		AccountType:         cmd.AccountType,
		Model:               cmd.Model,
		ServiceTier:         cmd.ServiceTier,
		ReasoningEffort:     cmd.ReasoningEffort,
		BillingType:         cmd.BillingType,
		InputTokens:         cmd.InputTokens,
		OutputTokens:        cmd.OutputTokens,
		CacheCreationTokens: cmd.CacheCreationTokens,
		CacheReadTokens:     cmd.CacheReadTokens,
		ImageCount:          cmd.ImageCount,
		MediaType:           cmd.MediaType,
		ActualCost:          cmd.ActualCost,
		BalanceCost:         cmd.BalanceCost,
		SubscriptionCost:    cmd.SubscriptionCost,
		APIKeyQuotaCost:     cmd.APIKeyQuotaCost,
		APIKeyRateLimitCost: cmd.APIKeyRateLimitCost,
		AccountQuotaCost:    cmd.AccountQuotaCost,
		RequestPayloadHash:  cmd.RequestPayloadHash,
		RequestFingerprint:  cmd.RequestFingerprint,
	})
}

func (r *usageBillingRepository) persistImageStudioBillingReconciliation(
	ctx context.Context,
	cmd *service.UsageBillingCommand,
	applyErr error,
) (_ error) {
	if r == nil || r.db == nil {
		return errors.New("usage billing repository db is nil")
	}
	payload, err := marshalImageStudioBillingReconciliationCommand(cmd)
	if err != nil {
		return err
	}
	lastError := sanitizeImageStudioBillingReconciliationError(applyErr)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	var storedFingerprint string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO image_studio_billing_reconciliations (
			request_id,
			api_key_id,
			user_id,
			actual_cost,
			command_payload,
			command_fingerprint,
			last_error,
			status,
			attempts,
			first_failed_at,
			last_failed_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, 'pending', 1, NOW(), NOW(), NOW(), NOW())
		ON CONFLICT (request_id, api_key_id) DO UPDATE
		SET
			last_error = EXCLUDED.last_error,
			attempts = image_studio_billing_reconciliations.attempts + 1,
			last_failed_at = NOW(),
			updated_at = NOW()
		WHERE image_studio_billing_reconciliations.command_fingerprint = EXCLUDED.command_fingerprint
		  AND image_studio_billing_reconciliations.actual_cost = EXCLUDED.actual_cost
		RETURNING command_fingerprint
	`,
		cmd.RequestID,
		cmd.APIKeyID,
		cmd.UserID,
		cmd.ActualCost,
		string(payload),
		cmd.RequestFingerprint,
		lastError,
	).Scan(&storedFingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUsageBillingRequestConflict
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(storedFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
		return service.ErrUsageBillingRequestConflict
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

func sanitizeImageStudioBillingReconciliationError(err error) string {
	if err == nil {
		return "usage billing apply failed"
	}
	message := strings.TrimSpace(strings.ReplaceAll(err.Error(), "\x00", ""))
	if message == "" {
		return "usage billing apply failed"
	}
	message = logredact.RedactText(
		message,
		"api_key",
		"apikey",
		"authorization",
		"cookie",
		"credential",
		"key",
		"payload",
		"prompt",
		"secret",
		"token",
	)
	const maxRunes = 4096
	runes := []rune(message)
	if len(runes) > maxRunes {
		message = string(runes[:maxRunes])
	}
	return message
}

func (r *usageBillingRepository) ReconcileImageStudioBilling(ctx context.Context, limit int) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("usage billing repository db is nil")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	resolved := 0
	for i := 0; i < limit; i++ {
		id, payload, ok, err := r.claimImageStudioBillingReconciliation(ctx)
		if err != nil {
			return resolved, err
		}
		if !ok {
			break
		}
		cmd, err := unmarshalImageStudioBillingReconciliationCommand(payload)
		if err == nil {
			_, err = r.applyUsageBillingTransaction(ctx, cmd)
		}
		if err != nil {
			if updateErr := r.failImageStudioBillingReconciliation(ctx, id, err); updateErr != nil {
				return resolved, errors.Join(err, updateErr)
			}
			continue
		}
		if _, err := r.db.ExecContext(ctx, `
			UPDATE image_studio_billing_reconciliations
			SET status = 'resolved',
			    resolved_at = NOW(),
			    updated_at = NOW(),
			    last_error = ''
			WHERE id = $1
			  AND status = 'processing'`, id); err != nil {
			return resolved, err
		}
		resolved++
	}
	return resolved, nil
}

func (r *usageBillingRepository) claimImageStudioBillingReconciliation(
	ctx context.Context,
) (id int64, payload []byte, ok bool, err error) {
	err = r.db.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM image_studio_billing_reconciliations
			WHERE status = 'pending'
			   OR (status = 'failed' AND last_failed_at <= NOW() - INTERVAL '1 minute')
			   OR (status = 'processing' AND updated_at <= NOW() - INTERVAL '5 minutes')
			ORDER BY last_failed_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE image_studio_billing_reconciliations r
		SET status = 'processing',
		    updated_at = NOW()
		FROM candidate
		WHERE r.id = candidate.id
		RETURNING r.id, r.command_payload::text`,
	).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, err
	}
	return id, payload, true, nil
}

func (r *usageBillingRepository) failImageStudioBillingReconciliation(
	ctx context.Context,
	id int64,
	reconciliationErr error,
) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE image_studio_billing_reconciliations
		SET status = 'failed',
		    attempts = attempts + 1,
		    last_failed_at = NOW(),
		    updated_at = NOW(),
		    last_error = $2
		WHERE id = $1
		  AND status = 'processing'`,
		id,
		sanitizeImageStudioBillingReconciliationError(reconciliationErr),
	)
	return err
}

func unmarshalImageStudioBillingReconciliationCommand(payload []byte) (*service.UsageBillingCommand, error) {
	var stored imageStudioBillingReconciliationCommand
	if err := json.Unmarshal(payload, &stored); err != nil {
		return nil, err
	}
	cmd := &service.UsageBillingCommand{
		RequestID:           stored.RequestID,
		APIKeyID:            stored.APIKeyID,
		UserID:              stored.UserID,
		AccountID:           stored.AccountID,
		SubscriptionID:      stored.SubscriptionID,
		AccountType:         stored.AccountType,
		Model:               stored.Model,
		ServiceTier:         stored.ServiceTier,
		ReasoningEffort:     stored.ReasoningEffort,
		BillingType:         stored.BillingType,
		InputTokens:         stored.InputTokens,
		OutputTokens:        stored.OutputTokens,
		CacheCreationTokens: stored.CacheCreationTokens,
		CacheReadTokens:     stored.CacheReadTokens,
		ImageCount:          stored.ImageCount,
		MediaType:           stored.MediaType,
		ActualCost:          stored.ActualCost,
		BalanceCost:         stored.BalanceCost,
		SubscriptionCost:    stored.SubscriptionCost,
		APIKeyQuotaCost:     stored.APIKeyQuotaCost,
		APIKeyRateLimitCost: stored.APIKeyRateLimitCost,
		AccountQuotaCost:    stored.AccountQuotaCost,
		RequestPayloadHash:  stored.RequestPayloadHash,
		RequestFingerprint:  stored.RequestFingerprint,
	}
	cmd.Normalize()
	if cmd.RequestID == "" || cmd.APIKeyID <= 0 || cmd.RequestFingerprint == "" {
		return nil, errors.New("invalid image studio billing reconciliation command")
	}
	return cmd, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	return r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
}

func (r *usageBillingRepository) claimUsageBillingRequest(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, requestFingerprint string) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, requestID, apiKeyID, requestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, requestID, apiKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) ReserveBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, reserveUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, captureUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) ReleaseBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, releaseUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) applyBatchImageBalanceHold(
	ctx context.Context,
	cmd *service.BatchImageBalanceHoldCommand,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (_ *service.BatchImageBalanceHoldResult, err error) {
	if cmd == nil {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}
	if tx := usageBillingTransactionFromContext(ctx); tx != nil {
		return r.applyBatchImageBalanceHoldTx(ctx, tx, cmd, apply)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	result, err := r.applyBatchImageBalanceHoldTx(ctx, tx, cmd, apply)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) applyBatchImageBalanceHoldTx(
	ctx context.Context,
	tx *sql.Tx,
	cmd *service.BatchImageBalanceHoldCommand,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (*service.BatchImageBalanceHoldResult, error) {
	if err := validateUsageBillingOwnership(ctx, tx, cmd.APIKeyID, cmd.UserID); err != nil {
		return nil, err
	}
	applied, err := r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}
	result, err := apply(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &service.BatchImageBalanceHoldResult{}
	}
	result.Applied = true
	return result, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.UserID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 {
		newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	return nil
}

func validateUsageBillingOwnership(ctx context.Context, tx *sql.Tx, apiKeyID, userID int64) error {
	if apiKeyID <= 0 || userID <= 0 {
		return service.ErrUsageBillingOwnershipMismatch
	}
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM api_keys
		WHERE id = $1 AND user_id = $2
	`, apiKeyID, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUsageBillingOwnershipMismatch
	}
	return err
}

func validateUsageBillingSubscriptionOwnership(ctx context.Context, tx *sql.Tx, subscriptionID, userID int64) error {
	if subscriptionID <= 0 || userID <= 0 {
		return service.ErrUsageBillingOwnershipMismatch
	}
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM user_subscriptions
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, subscriptionID, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUsageBillingOwnershipMismatch
	}
	return err
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID, userID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.user_id = $3
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	var newBalance float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if err == nil {
		return newBalance, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}

	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, service.ErrUserNotFound
	}
	if err != nil {
		return 0, false, err
	}
	return newBalance, false, nil
}

func reserveUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			frozen_balance = COALESCE(frozen_balance, 0) + $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, service.ErrBatchImageInsufficientBalance
}

func captureUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 && cmd.ActualAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if cmd.ActualAmount-cmd.HoldAmount > 0.00000001 && !cmd.AllowBalanceOverage {
		return nil, service.ErrBatchImageSettlementCostExceedsHold
	}
	if err := validateBalanceHoldClaim(ctx, tx, cmd); err != nil {
		return nil, err
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance
				+ CASE WHEN $1 > $2 THEN $1 - $2 ELSE 0 END
				- CASE WHEN $2 > $1 THEN $2 - $1 ELSE 0 END,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.ActualAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func releaseUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	// 释放前校验该 job 确实预留过 hold（hold request id 已被 claim），
	// 防止从未成功冻结的 job 触发"幻影释放"，从其他用户的冻结资金池中凭空生成余额。
	if err := validateBalanceHoldClaim(ctx, tx, cmd); err != nil {
		if errors.Is(err, service.ErrUsageBillingHoldNotFound) {
			if strings.TrimSpace(cmd.HoldRequestID) != "" &&
				strings.TrimSpace(cmd.HoldRequestID) != service.BatchImageHoldRequestID(cmd.BatchID) {
				return nil, err
			}
			logger.LegacyPrintf("repository.usage_billing", "[BatchImage] release skipped, hold was never reserved: batch=%s", cmd.BatchID)
			return &service.BatchImageBalanceHoldResult{}, nil
		}
		return nil, err
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func validateBalanceHoldClaim(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) error {
	holdRequestID := strings.TrimSpace(cmd.HoldRequestID)
	if holdRequestID == "" {
		holdRequestID = service.BatchImageHoldRequestID(cmd.BatchID)
	}
	held, err := batchImageHoldClaimExists(ctx, tx, holdRequestID, cmd.APIKeyID)
	if err != nil {
		return err
	}
	if !held {
		return service.ErrUsageBillingHoldNotFound
	}
	return nil
}

// batchImageHoldClaimExists 检查 hold request id 是否已在 dedup（或归档）表中被 claim，
// 即该 batch 的冻结操作确实成功提交过。
func batchImageHoldClaimExists(ctx context.Context, tx *sql.Tx, holdRequestID string, apiKeyID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func userExistsForBilling(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
