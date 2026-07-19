//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestUsageBillingRepositoryApply_DeduplicatesBalanceBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-" + uuid.NewString(),
		Name:   "billing",
		Quota:  1,
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:           requestID,
		APIKeyID:            apiKey.ID,
		UserID:              user.ID,
		AccountID:           account.ID,
		AccountType:         service.AccountTypeAPIKey,
		BalanceCost:         1.25,
		APIKeyQuotaCost:     1.25,
		APIKeyRateLimitCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result1)
	require.True(t, result1.Applied)
	require.True(t, result1.APIKeyQuotaExhausted)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id = $1", apiKey.ID).Scan(&quotaUsed))
	require.InDelta(t, 1.25, quotaUsed, 0.000001)

	var usage5h float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT usage_5h FROM api_keys WHERE id = $1", apiKey.ID).Scan(&usage5h))
	require.InDelta(t, 1.25, usage5h, 0.000001)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM api_keys WHERE id = $1", apiKey.ID).Scan(&status))
	require.Equal(t, service.StatusAPIKeyQuotaExhausted, status)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Equal(t, 1, dedupCount)
}

func TestUsageBillingRepositoryApply_WithLedgerWritesUsageCharge(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	ledger := service.NewBalanceLedgerService(integrationDB, nil, nil)
	repo := NewUsageBillingRepositoryWithLedger(client, integrationDB, ledger)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-ledger-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-ledger-" + uuid.NewString(),
		Name:   "billing-ledger",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-ledger-account-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:          requestID,
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		AccountID:          account.ID,
		AccountType:        service.AccountTypeAPIKey,
		Model:              "gpt-ledger",
		BalanceCost:        1.25,
		RequestPayloadHash: "payload-ledger",
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)
	require.NotNil(t, result1.NewBalance)
	require.InDelta(t, 98.75, *result1.NewBalance, 0.000001)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	requireUserBalanceAndFrozen(t, ctx, user.ID, 98.75, 0)
	row := requireBalanceTransaction(t, ctx, user.ID, "usage_charge", fmt.Sprintf("usage_billing:%d:%s", apiKey.ID, requestID))
	require.InDelta(t, -1.25, row.balanceDelta, 0.000001)
	require.InDelta(t, 100, row.balanceBefore, 0.000001)
	require.InDelta(t, 98.75, row.balanceAfter, 0.000001)
	require.Contains(t, row.metadata, `"request_id": "`+requestID+`"`)
	require.Contains(t, row.metadata, `"api_key_id": `+fmt.Sprint(apiKey.ID))
}

func TestUsageBillingRepositoryApply_RejectsCrossUserCharge(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	owner := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-owner-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	victim := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-victim-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: owner.ID,
		Key:    "sk-usage-billing-cross-user-" + uuid.NewString(),
		Name:   "cross-user",
	})
	requestID := uuid.NewString()

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      victim.ID,
		BalanceCost: 9.99,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingOwnershipMismatch)

	var victimBalance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", victim.ID).Scan(&victimBalance))
	require.InDelta(t, 100, victimBalance, 0.000001)
	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Zero(t, dedupCount)
}

func TestUsageBillingRepositoryApply_RejectsCrossUserSubscriptionCharge(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	owner := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-owner-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	otherUser := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-other-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-cross-sub-group-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  owner.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-cross-sub-" + uuid.NewString(),
		Name:    "cross-subscription",
	})
	otherSubscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  otherUser.ID,
		GroupID: group.ID,
	})
	requestID := uuid.NewString()

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        requestID,
		APIKeyID:         apiKey.ID,
		UserID:           owner.ID,
		SubscriptionID:   &otherSubscription.ID,
		SubscriptionCost: 7.5,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingOwnershipMismatch)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", otherSubscription.ID).Scan(&dailyUsage))
	require.Zero(t, dailyUsage)
	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Zero(t, dedupCount)
}

func TestUsageBillingRepositoryReserveBatchImageBalance_RejectsCrossUserCharge(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	owner := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-owner-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	otherUser := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-other-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: owner.ID,
		Key:    "sk-batch-image-cross-user-" + uuid.NewString(),
		Name:   "batch-image-cross-user",
	})
	requestID := uuid.NewString()

	_, err := repo.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:  requestID,
		APIKeyID:   apiKey.ID,
		UserID:     otherUser.ID,
		BatchID:    uuid.NewString(),
		HoldAmount: 12.5,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingOwnershipMismatch)

	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance, frozen_balance FROM users WHERE id = $1", otherUser.ID).Scan(&balance, &frozen))
	require.InDelta(t, 100, balance, 0.000001)
	require.Zero(t, frozen)
	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&dedupCount))
	require.Zero(t, dedupCount)
}

func TestUsageBillingRepositoryBatchImageHoldAllowsOnlyOneTerminalOutcome(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-terminal-hold-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-batch-image-terminal-hold-" + uuid.NewString(),
		Name:   "batch-image-terminal-hold",
	})

	reserve := func(batchID string, amount float64) {
		t.Helper()
		_, err := repo.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
			RequestID:          service.BatchImageHoldRequestID(batchID),
			HoldRequestID:      service.BatchImageHoldRequestID(batchID),
			APIKeyID:           apiKey.ID,
			UserID:             user.ID,
			BatchID:            batchID,
			HoldAmount:         amount,
			RequestPayloadHash: "request-" + batchID,
		})
		require.NoError(t, err)
	}

	batchA := "imgbatch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	batchB := "imgbatch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	reserve(batchA, 10)
	reserve(batchB, 20)
	requireUserBalanceAndFrozen(t, ctx, user.ID, 70, 30)

	_, err := repo.CaptureBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:          service.BatchImageCaptureRequestID(batchA),
		HoldRequestID:      service.BatchImageHoldRequestID(batchA),
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		BatchID:            batchA,
		HoldAmount:         10,
		ActualAmount:       6,
		RequestPayloadHash: "settlement-" + batchA,
	})
	require.NoError(t, err)
	requireUserBalanceAndFrozen(t, ctx, user.ID, 74, 20)

	_, err = repo.ReleaseBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:          service.BatchImageReleaseRequestID(batchA),
		HoldRequestID:      service.BatchImageHoldRequestID(batchA),
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		BatchID:            batchA,
		HoldAmount:         10,
		RequestPayloadHash: "request-" + batchA,
	})
	require.Error(t, err)
	requireUserBalanceAndFrozen(t, ctx, user.ID, 74, 20)
}

func TestBatchImageSettlementPersistsRealBalanceLifecycle(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	billingRepo := NewUsageBillingRepository(client, integrationDB)
	batchRepo := NewBatchImageRepository(integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-settlement-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-batch-image-settlement-" + uuid.NewString(),
		Name:   "batch-image-settlement",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name:        "batch-image-settlement-" + uuid.NewString(),
		Platform:    service.PlatformGemini,
		Type:        service.AccountTypeAPIKey,
		Schedulable: true,
	})

	t.Run("capture charges successful images and releases the remainder", func(t *testing.T) {
		batchID := "imgbatch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		holdID := service.BatchImageHoldRequestID(batchID)
		requestHash := "request-" + uuid.NewString()
		holdAmount := 10.0
		apiKeyID := apiKey.ID
		accountID := account.ID
		_, err := batchRepo.CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
			BatchID:                 batchID,
			UserID:                  user.ID,
			APIKeyID:                &apiKeyID,
			AccountID:               &accountID,
			Provider:                service.BatchImageProviderGeminiAPI,
			Model:                   "gemini-2.5-flash-image",
			Status:                  service.BatchImageJobStatusSettling,
			ItemCount:               5,
			SuccessCount:            3,
			OutputImageCount:        3,
			FailCount:               2,
			EstimatedCost:           6,
			HoldAmount:              &holdAmount,
			BillableUnitPrice:       2,
			PricingSnapshotVersion:  1,
			GroupRateMultiplier:     1,
			AccountRateMultiplier:   1,
			BatchDiscountMultiplier: 1,
			HoldMultiplier:          1,
			HoldID:                  &holdID,
			RequestHash:             &requestHash,
		})
		require.NoError(t, err)

		reserved, err := billingRepo.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
			RequestID:          holdID,
			HoldRequestID:      holdID,
			APIKeyID:           apiKey.ID,
			UserID:             user.ID,
			BatchID:            batchID,
			HoldAmount:         holdAmount,
			RequestPayloadHash: requestHash,
		})
		require.NoError(t, err)
		require.True(t, reserved.Applied)
		requireUserBalanceAndFrozen(t, ctx, user.ID, 90, 10)

		settlement := &service.BatchImageSettlementService{
			Repo:        batchRepo,
			BillingRepo: billingRepo,
			Pricing:     &service.BatchImageModelPricingResolver{},
		}
		result, err := settlement.Settle(ctx, batchID)
		require.NoError(t, err)
		require.InDelta(t, 6, result.ActualCost, 0.000001)
		requireUserBalanceAndFrozen(t, ctx, user.ID, 94, 0)

		job, err := batchRepo.GetBatchImageJobByBatchID(ctx, batchID)
		require.NoError(t, err)
		require.Equal(t, service.BatchImageJobStatusCompleted, job.Status)
		require.NotNil(t, job.ActualCost)
		require.InDelta(t, 6, *job.ActualCost, 0.000001)

		replayed, err := settlement.Settle(ctx, batchID)
		require.NoError(t, err)
		require.True(t, replayed.AlreadySettled)
		requireUserBalanceAndFrozen(t, ctx, user.ID, 94, 0)
	})

	t.Run("release restores the full hold idempotently", func(t *testing.T) {
		batchID := "imgbatch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		holdID := service.BatchImageHoldRequestID(batchID)
		holdAmount := 8.0
		requestHash := "request-" + uuid.NewString()

		reserve := &service.BatchImageBalanceHoldCommand{
			RequestID:          holdID,
			HoldRequestID:      holdID,
			APIKeyID:           apiKey.ID,
			UserID:             user.ID,
			BatchID:            batchID,
			HoldAmount:         holdAmount,
			RequestPayloadHash: requestHash,
		}
		_, err := billingRepo.ReserveBatchImageBalance(ctx, reserve)
		require.NoError(t, err)
		requireUserBalanceAndFrozen(t, ctx, user.ID, 86, 8)

		release := &service.BatchImageBalanceHoldCommand{
			RequestID:          service.BatchImageReleaseRequestID(batchID),
			HoldRequestID:      holdID,
			APIKeyID:           apiKey.ID,
			UserID:             user.ID,
			BatchID:            batchID,
			HoldAmount:         holdAmount,
			RequestPayloadHash: requestHash,
		}
		first, err := billingRepo.ReleaseBatchImageBalance(ctx, release)
		require.NoError(t, err)
		require.True(t, first.Applied)
		requireUserBalanceAndFrozen(t, ctx, user.ID, 94, 0)

		second, err := billingRepo.ReleaseBatchImageBalance(ctx, release)
		require.NoError(t, err)
		require.False(t, second.Applied)
		requireUserBalanceAndFrozen(t, ctx, user.ID, 94, 0)
	})
}

func TestUsageBillingRepositoryBatchImageHold_WithLedgerWritesFrozenLifecycle(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	ledger := service.NewBalanceLedgerService(integrationDB, nil, nil)
	repo := NewUsageBillingRepositoryWithLedger(client, integrationDB, ledger)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("batch-image-ledger-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-batch-image-ledger-" + uuid.NewString(),
		Name:   "batch-image-ledger",
	})
	batchID := "imgbatch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	holdID := service.BatchImageHoldRequestID(batchID)
	hash := "batch-ledger-" + uuid.NewString()

	reserved, err := repo.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:          holdID,
		HoldRequestID:      holdID,
		CaptureRequestID:   service.BatchImageCaptureRequestID(batchID),
		ReleaseRequestID:   service.BatchImageReleaseRequestID(batchID),
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		BatchID:            batchID,
		HoldAmount:         10,
		RequestPayloadHash: hash,
	})
	require.NoError(t, err)
	require.True(t, reserved.Applied)
	requireUserBalanceAndFrozen(t, ctx, user.ID, 90, 10)

	captured, err := repo.CaptureBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:          service.BatchImageCaptureRequestID(batchID),
		HoldRequestID:      holdID,
		CaptureRequestID:   service.BatchImageCaptureRequestID(batchID),
		ReleaseRequestID:   service.BatchImageReleaseRequestID(batchID),
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		BatchID:            batchID,
		HoldAmount:         10,
		ActualAmount:       6,
		RequestPayloadHash: hash,
	})
	require.NoError(t, err)
	require.True(t, captured.Applied)
	requireUserBalanceAndFrozen(t, ctx, user.ID, 94, 0)

	holdRow := requireBalanceTransaction(t, ctx, user.ID, "image_balance_hold", fmt.Sprintf("image_balance_hold:%d:%s", apiKey.ID, holdID))
	require.InDelta(t, -10, holdRow.balanceDelta, 0.000001)
	require.InDelta(t, 10, holdRow.frozenDelta, 0.000001)
	require.Contains(t, holdRow.metadata, `"batch_id": "`+batchID+`"`)

	captureRow := requireBalanceTransaction(t, ctx, user.ID, "image_balance_capture", fmt.Sprintf("image_balance_capture:%d:%s", apiKey.ID, service.BatchImageCaptureRequestID(batchID)))
	require.InDelta(t, 4, captureRow.balanceDelta, 0.000001)
	require.InDelta(t, -10, captureRow.frozenDelta, 0.000001)
	require.Contains(t, captureRow.metadata, `"actual_amount": 6`)
}

func requireUserBalanceAndFrozen(t *testing.T, ctx context.Context, userID int64, wantBalance, wantFrozen float64) {
	t.Helper()
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance, frozen_balance
		FROM users
		WHERE id = $1
	`, userID).Scan(&balance, &frozen))
	require.InDelta(t, wantBalance, balance, 0.000001)
	require.InDelta(t, wantFrozen, frozen, 0.000001)
}

type balanceTransactionAssertionRow struct {
	balanceDelta  float64
	balanceBefore float64
	balanceAfter  float64
	frozenDelta   float64
	frozenBefore  float64
	frozenAfter   float64
	metadata      string
}

func requireBalanceTransaction(t *testing.T, ctx context.Context, userID int64, sourceType, idempotencyKey string) balanceTransactionAssertionRow {
	t.Helper()
	var row balanceTransactionAssertionRow
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			balance_delta::double precision,
			COALESCE(balance_before, 0)::double precision,
			COALESCE(balance_after, 0)::double precision,
			frozen_delta::double precision,
			COALESCE(frozen_before, 0)::double precision,
			COALESCE(frozen_after, 0)::double precision,
			metadata::text
		FROM balance_transactions
		WHERE user_id = $1
		  AND source_type = $2
		  AND idempotency_key = $3
	`, userID, sourceType, idempotencyKey).Scan(
		&row.balanceDelta,
		&row.balanceBefore,
		&row.balanceAfter,
		&row.frozenDelta,
		&row.frozenBefore,
		&row.frozenAfter,
		&row.metadata,
	))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM balance_transactions
		WHERE user_id = $1
		  AND source_type = $2
		  AND idempotency_key = $3
	`, userID, sourceType, idempotencyKey).Scan(&count))
	require.Equal(t, 1, count)
	return row
}

func TestUsageBillingRepositoryApply_DeduplicatesSubscriptionBilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-sub-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "usage-billing-group-" + uuid.NewString(),
		Platform:         service.PlatformAnthropic,
		SubscriptionType: service.SubscriptionTypeSubscription,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:  user.ID,
		GroupID: &group.ID,
		Key:     "sk-usage-billing-sub-" + uuid.NewString(),
		Name:    "billing-sub",
	})
	subscription := mustCreateSubscription(t, client, &service.UserSubscription{
		UserID:  user.ID,
		GroupID: group.ID,
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:        requestID,
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        0,
		SubscriptionID:   &subscription.ID,
		SubscriptionCost: 2.5,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1", subscription.ID).Scan(&dailyUsage))
	require.InDelta(t, 2.5, dailyUsage, 0.000001)
}

func TestUsageBillingRepositoryApply_RequestFingerprintConflict(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-conflict-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-conflict-" + uuid.NewString(),
		Name:   "billing-conflict",
	})

	requestID := uuid.NewString()
	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	})
	require.NoError(t, err)

	_, err = repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 2.50,
	})
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
}

func TestUsageBillingRepositoryApply_UpdatesAccountQuota(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-account-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-account-" + uuid.NewString(),
		Name:   "billing-account",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "usage-billing-account-quota-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_limit": 100.0,
		},
	})

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:        uuid.NewString(),
		APIKeyID:         apiKey.ID,
		UserID:           user.ID,
		AccountID:        account.ID,
		AccountType:      service.AccountTypeAPIKey,
		AccountQuotaCost: 3.5,
	})
	require.NoError(t, err)

	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COALESCE((extra->>'quota_used')::numeric, 0) FROM accounts WHERE id = $1", account.ID).Scan(&quotaUsed))
	require.InDelta(t, 3.5, quotaUsed, 0.000001)
}

func TestUsageBillingRepositoryApply_EnqueuesSchedulerOutboxOnQuotaCrossing(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	newFixture := func(t *testing.T, extra map[string]any) (int64, int64) {
		t.Helper()
		user := mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("usage-billing-outbox-user-%d-%s@example.com", time.Now().UnixNano(), uuid.NewString()),
			PasswordHash: "hash",
		})
		apiKey := mustCreateApiKey(t, client, &service.APIKey{
			UserID: user.ID,
			Key:    "sk-usage-billing-outbox-" + uuid.NewString(),
			Name:   "billing-outbox",
		})
		account := mustCreateAccount(t, client, &service.Account{
			Name:  "usage-billing-outbox-" + uuid.NewString(),
			Type:  service.AccountTypeAPIKey,
			Extra: extra,
		})
		return apiKey.ID, account.ID
	}

	outboxCountFor := func(t *testing.T, accountID int64) int {
		t.Helper()
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1 AND account_id = $2",
			service.SchedulerOutboxEventAccountChanged, accountID,
		).Scan(&count))
		return count
	}

	t.Run("daily_first_crossing_enqueues", func(t *testing.T) {
		apiKeyID, accountID := newFixture(t, map[string]any{
			"quota_daily_limit": 10.0,
		})
		// 第一次低于日限额：不应入队 outbox
		_, err := repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 4,
		})
		require.NoError(t, err)
		require.Equal(t, 0, outboxCountFor(t, accountID), "below limit should not enqueue")

		// 第二次跨越日限额：应入队一次 outbox
		_, err = repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 8,
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "crossing daily limit should enqueue once")

		// 再次递增（已超）：不应重复入队
		_, err = repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 2,
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "subsequent increments beyond limit should not re-enqueue")
	})

	t.Run("weekly_first_crossing_enqueues", func(t *testing.T) {
		apiKeyID, accountID := newFixture(t, map[string]any{
			"quota_weekly_limit": 10.0,
		})
		_, err := repo.Apply(ctx, &service.UsageBillingCommand{
			RequestID:        uuid.NewString(),
			APIKeyID:         apiKeyID,
			AccountID:        accountID,
			AccountType:      service.AccountTypeAPIKey,
			AccountQuotaCost: 15, // 单次即跨越
		})
		require.NoError(t, err)
		require.Equal(t, 1, outboxCountFor(t, accountID), "single-shot crossing weekly limit should enqueue once")
	})
}

func TestDashboardAggregationRepositoryCleanupUsageBillingDedup_BatchDeletesOldRows(t *testing.T) {
	ctx := context.Background()
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	oldRequestID := "dedup-old-" + uuid.NewString()
	newRequestID := "dedup-new-" + uuid.NewString()
	oldCreatedAt := time.Now().UTC().AddDate(0, 0, -400)
	newCreatedAt := time.Now().UTC().Add(-time.Hour)

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint, created_at)
		VALUES ($1, 1, $2, $3), ($4, 1, $5, $6)
	`,
		oldRequestID, strings.Repeat("a", 64), oldCreatedAt,
		newRequestID, strings.Repeat("b", 64), newCreatedAt,
	)
	require.NoError(t, err)

	require.NoError(t, repo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	var oldCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", oldRequestID).Scan(&oldCount))
	require.Equal(t, 0, oldCount)

	var newCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", newRequestID).Scan(&newCount))
	require.Equal(t, 1, newCount)

	var archivedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup_archive WHERE request_id = $1", oldRequestID).Scan(&archivedCount))
	require.Equal(t, 1, archivedCount)
}

func TestUsageBillingRepositoryApply_DeduplicatesAgainstArchivedKey(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	aggRepo := newDashboardAggregationRepositoryWithSQL(integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-archive-user-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-archive-" + uuid.NewString(),
		Name:   "billing-archive",
	})

	requestID := uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		UserID:      user.ID,
		BalanceCost: 1.25,
	}

	result1, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result1.Applied)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE usage_billing_dedup
		SET created_at = $1
		WHERE request_id = $2 AND api_key_id = $3
	`, time.Now().UTC().AddDate(0, 0, -400), requestID, apiKey.ID)
	require.NoError(t, err)
	require.NoError(t, aggRepo.CleanupUsageBillingDedup(ctx, time.Now().UTC().AddDate(0, 0, -365)))

	result2, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result2.Applied)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)
}
