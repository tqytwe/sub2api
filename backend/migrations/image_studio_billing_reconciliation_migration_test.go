package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioBillingReconciliationMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("195_image_studio_billing_reconciliation.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))
	normalized := strings.Join(strings.Fields(sql), " ")

	for _, fragment := range []string{
		"create table if not exists image_studio_billing_reconciliations",
		"request_id",
		"api_key_id",
		"user_id",
		"actual_cost",
		"command_payload jsonb",
		"command_fingerprint",
		"last_error",
		"status",
		"attempts",
		"first_failed_at",
		"last_failed_at",
		"created_at",
		"updated_at",
		"resolved_at",
		"unique (request_id, api_key_id)",
		"'pending'",
		"'processing'",
		"'resolved'",
		"'failed'",
	} {
		require.Contains(t, normalized, fragment)
	}

	for _, forbidden := range []string{
		"prompt",
		"credential",
		"api_key_value",
		"request_payload_encrypted",
	} {
		require.NotContains(t, sql, forbidden)
	}
}
