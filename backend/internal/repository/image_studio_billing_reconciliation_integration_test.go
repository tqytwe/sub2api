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

func TestUsageBillingRepositoryApply_ManagedFailurePersistsIdempotentReconciliation(t *testing.T) {
	ctx := service.WithImageStudioManagedBilling(context.Background())
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reconciliation-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-reconciliation-" + uuid.NewString(),
		Name:   "image-studio-reconciliation",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "image-studio-reconciliation-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	requestID := "image-studio-item-" + uuid.NewString()
	installUsageBillingFailureTrigger(t, requestID, false)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM image_studio_billing_reconciliations WHERE request_id = $1 AND api_key_id = $2",
			requestID, apiKey.ID)
	})

	cmd := &service.UsageBillingCommand{
		RequestID:           requestID,
		APIKeyID:            apiKey.ID,
		UserID:              user.ID,
		AccountID:           account.ID,
		AccountType:         service.AccountTypeAPIKey,
		Model:               "gpt-image-2",
		ImageCount:          1,
		MediaType:           "image",
		ActualCost:          0.25,
		APIKeyQuotaCost:     0.25,
		APIKeyRateLimitCost: 0.25,
		AccountQuotaCost:    0.25,
		RequestPayloadHash:  service.HashUsageRequestPayload([]byte("private prompt")),
	}

	for attempt := 1; attempt <= 2; attempt++ {
		_, err := repo.Apply(ctx, cmd)
		require.ErrorContains(t, err, "forced usage billing apply failure")
	}

	var (
		rowCount           int
		actualCost         float64
		commandPayload     string
		commandFingerprint string
		lastError          string
		status             string
		attempts           int
		firstFailedAt      time.Time
		lastFailedAt       time.Time
		createdAt          time.Time
		updatedAt          time.Time
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*) OVER (),
			actual_cost,
			command_payload::text,
			command_fingerprint,
			last_error,
			status,
			attempts,
			first_failed_at,
			last_failed_at,
			created_at,
			updated_at
		FROM image_studio_billing_reconciliations
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKey.ID).Scan(
		&rowCount,
		&actualCost,
		&commandPayload,
		&commandFingerprint,
		&lastError,
		&status,
		&attempts,
		&firstFailedAt,
		&lastFailedAt,
		&createdAt,
		&updatedAt,
	))

	require.Equal(t, 1, rowCount)
	require.InDelta(t, 0.25, actualCost, 0.000001)
	require.NotEmpty(t, commandFingerprint)
	require.Contains(t, lastError, "forced usage billing apply failure")
	require.Equal(t, "pending", status)
	require.Equal(t, 2, attempts)
	require.False(t, firstFailedAt.IsZero())
	require.False(t, lastFailedAt.IsZero())
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())
	require.Contains(t, commandPayload, requestID)
	require.NotContains(t, commandPayload, "private prompt")

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2",
		requestID, apiKey.ID,
	).Scan(&dedupCount))
	require.Zero(t, dedupCount)
}

func TestUsageBillingRepositoryReconcileImageStudioBillingResolvesPendingOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	reconciler, ok := repo.(service.ImageStudioBillingReconciler)
	require.True(t, ok)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reconcile-replay-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-reconcile-replay-" + uuid.NewString(),
		Name:   "image-studio-reconcile-replay",
		Quota:  100,
	})
	cmd := &service.UsageBillingCommand{
		RequestID:       "image-studio-reconcile-replay-" + uuid.NewString(),
		APIKeyID:        apiKey.ID,
		UserID:          user.ID,
		Model:           "gpt-image-2",
		ImageCount:      1,
		MediaType:       "image",
		ActualCost:      0.25,
		APIKeyQuotaCost: 0.25,
	}
	cmd.Normalize()
	payload, err := marshalImageStudioBillingReconciliationCommand(cmd)
	require.NoError(t, err)
	var reconciliationID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO image_studio_billing_reconciliations (
			request_id, api_key_id, user_id, actual_cost, command_payload,
			command_fingerprint, last_error, status
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, 'pending test record', 'pending')
		RETURNING id`,
		cmd.RequestID,
		cmd.APIKeyID,
		cmd.UserID,
		cmd.ActualCost,
		string(payload),
		cmd.RequestFingerprint,
	).Scan(&reconciliationID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM image_studio_billing_reconciliations WHERE id = $1", reconciliationID)
	})

	resolved, err := reconciler.ReconcileImageStudioBilling(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, resolved)
	resolved, err = reconciler.ReconcileImageStudioBilling(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, resolved)

	var status string
	var quotaUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT status FROM image_studio_billing_reconciliations WHERE id = $1",
		reconciliationID,
	).Scan(&status))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT quota_used FROM api_keys WHERE id = $1",
		apiKey.ID,
	).Scan(&quotaUsed))
	require.Equal(t, "resolved", status)
	require.InDelta(t, 0.25, quotaUsed, 0.000001)
}

func TestUsageBillingRepositoryApply_ManagedFailureReportsReconciliationFailure(t *testing.T) {
	ctx := service.WithImageStudioManagedBilling(context.Background())
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reconciliation-error-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-reconciliation-error-" + uuid.NewString(),
		Name:   "image-studio-reconciliation-error",
	})
	requestID := "image-studio-item-" + uuid.NewString()
	installUsageBillingFailureTrigger(t, requestID, true)

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:          requestID,
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		Model:              "gpt-image-2",
		ImageCount:         1,
		MediaType:          "image",
		ActualCost:         0.25,
		APIKeyQuotaCost:    0.25,
		RequestPayloadHash: service.HashUsageRequestPayload([]byte("private prompt")),
	})

	require.ErrorContains(t, err, "forced usage billing apply failure")
	require.ErrorContains(t, err, "persist image studio billing reconciliation")
	require.ErrorContains(t, err, "forced reconciliation persistence failure")
}

func TestUsageBillingRepositoryApply_ManagedCanceledContextStillPersistsReconciliation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	ctx := service.WithImageStudioManagedBilling(parent)
	cancel()

	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reconciliation-canceled-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-reconciliation-canceled-" + uuid.NewString(),
		Name:   "image-studio-reconciliation-canceled",
	})
	requestID := "image-studio-item-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM image_studio_billing_reconciliations WHERE request_id = $1 AND api_key_id = $2",
			requestID, apiKey.ID)
	})

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:          requestID,
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		Model:              "gpt-image-2",
		ImageCount:         1,
		MediaType:          "image",
		ActualCost:         0.25,
		APIKeyQuotaCost:    0.25,
		RequestPayloadHash: service.HashUsageRequestPayload([]byte("private prompt")),
	})
	require.ErrorIs(t, err, context.Canceled)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM image_studio_billing_reconciliations
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKey.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestUsageBillingRepositoryApply_NonManagedFailureDoesNotPersistReconciliation(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("usage-billing-no-reconciliation-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-usage-billing-no-reconciliation-" + uuid.NewString(),
		Name:   "usage-billing-no-reconciliation",
	})
	requestID := "ordinary-request-" + uuid.NewString()
	installUsageBillingFailureTrigger(t, requestID, false)

	_, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:       requestID,
		APIKeyID:        apiKey.ID,
		UserID:          user.ID,
		ActualCost:      0.25,
		APIKeyQuotaCost: 0.25,
	})
	require.ErrorContains(t, err, "forced usage billing apply failure")

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM image_studio_billing_reconciliations
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKey.ID).Scan(&count))
	require.Zero(t, count)
}

func installUsageBillingFailureTrigger(t *testing.T, requestID string, failReconciliation bool) {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	applyFunction := "fail_usage_billing_apply_" + suffix
	applyTrigger := "fail_usage_billing_apply_trigger_" + suffix
	require.NoError(t, execIntegrationSQL(t, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger AS $$
		BEGIN
			IF NEW.request_id = '%s' THEN
				RAISE EXCEPTION 'forced usage billing apply failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER %s
		BEFORE INSERT ON usage_billing_dedup
		FOR EACH ROW EXECUTE FUNCTION %s();
	`, applyFunction, requestID, applyTrigger, applyFunction)))

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf(
			"DROP TRIGGER IF EXISTS %s ON usage_billing_dedup", applyTrigger))
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf(
			"DROP FUNCTION IF EXISTS %s()", applyFunction))
	})

	if !failReconciliation {
		return
	}

	reconciliationFunction := "fail_image_studio_reconciliation_" + suffix
	reconciliationTrigger := "fail_image_studio_reconciliation_trigger_" + suffix
	require.NoError(t, execIntegrationSQL(t, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger AS $$
		BEGIN
			IF NEW.request_id = '%s' THEN
				RAISE EXCEPTION 'forced reconciliation persistence failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER %s
		BEFORE INSERT ON image_studio_billing_reconciliations
		FOR EACH ROW EXECUTE FUNCTION %s();
	`, reconciliationFunction, requestID, reconciliationTrigger, reconciliationFunction)))

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf(
			"DROP TRIGGER IF EXISTS %s ON image_studio_billing_reconciliations", reconciliationTrigger))
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf(
			"DROP FUNCTION IF EXISTS %s()", reconciliationFunction))
	})
}

func execIntegrationSQL(t *testing.T, query string) error {
	t.Helper()
	_, err := integrationDB.ExecContext(context.Background(), query)
	return err
}
