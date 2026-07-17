package repository

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestMarshalImageStudioBillingReconciliationCommandKeepsOnlyBillingData(t *testing.T) {
	subscriptionID := int64(55)
	cmd := &service.UsageBillingCommand{
		RequestID:           "image-studio-item-job-1-0",
		APIKeyID:            20,
		UserID:              10,
		AccountID:           30,
		SubscriptionID:      &subscriptionID,
		AccountType:         service.AccountTypeAPIKey,
		Model:               "gpt-image-2",
		BillingType:         service.BillingTypeBalance,
		ImageCount:          1,
		MediaType:           "image",
		ActualCost:          0.25,
		APIKeyQuotaCost:     0.25,
		APIKeyRateLimitCost: 0.25,
		AccountQuotaCost:    0.25,
		RequestPayloadHash:  service.HashUsageRequestPayload([]byte("private prompt must not be stored")),
	}
	cmd.Normalize()

	payload, err := marshalImageStudioBillingReconciliationCommand(cmd)
	require.NoError(t, err)
	require.True(t, json.Valid(payload))

	text := string(payload)
	for _, expected := range []string{
		`"request_id":"image-studio-item-job-1-0"`,
		`"api_key_id":20`,
		`"user_id":10`,
		`"actual_cost":0.25`,
		`"request_payload_hash":"`,
	} {
		require.Contains(t, text, expected)
	}
	for _, forbidden := range []string{
		"private prompt must not be stored",
		"credential",
		"api_key_value",
	} {
		require.NotContains(t, text, forbidden)
	}
}

func TestSanitizeImageStudioBillingReconciliationErrorRedactsSensitiveValues(t *testing.T) {
	got := sanitizeImageStudioBillingReconciliationError(errors.New(
		"apply failed api_key=sk-private token=tok-private prompt=private-scene password=secret-pass",
	))

	require.Contains(t, got, "apply failed")
	require.Contains(t, got, "***")
	for _, forbidden := range []string{
		"sk-private",
		"tok-private",
		"private-scene",
		"secret-pass",
	} {
		require.NotContains(t, got, forbidden)
	}
}
