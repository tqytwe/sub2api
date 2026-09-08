package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManualPaymentConfirmationReferenceMigrationRejectsDuplicatesBeforeUniqueIndex(t *testing.T) {
	sql, err := os.ReadFile("270_payment_manual_confirmation_reference.sql")
	require.NoError(t, err)
	text := string(sql)
	require.Contains(t, text, "HAVING COUNT(*) > 1")
	require.Contains(t, text, "RAISE EXCEPTION")
	require.Contains(t, text, "CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_orders_provider_trade_no_unique")
	require.Contains(t, text, "WHEN NULLIF(provider_instance_id, '') IS NOT NULL")
	require.Contains(t, text, "THEN 'instance:' || provider_instance_id")
	require.Contains(t, text, "THEN 'provider:' || lower(provider_key)")
	require.Contains(t, text, "ELSE 'type:' || lower(payment_type)")
	require.Contains(t, text, "WHERE payment_trade_no <> ''")
}
