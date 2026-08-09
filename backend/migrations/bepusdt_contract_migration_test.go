package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBepusdtPaymentDatabaseContractMigration(t *testing.T) {
	content, err := FS.ReadFile("252_bepusdt_payment_contract.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "payment_provider_instances_bepusdt_contract_check")
	require.Contains(t, sql, "supported_types = 'bepusdt'")
	require.Contains(t, sql, "payment_mode = ''")
	require.Contains(t, sql, "refund_enabled = false")
	require.Contains(t, sql, "allow_user_refund = false")
	require.Contains(t, sql, "config::jsonb ->> 'apitoken'")
	require.Contains(t, sql, "config::jsonb ->> 'currency' = 'cny'")
	require.Contains(t, sql, "payment_orders_bepusdt_contract_check")
	require.Contains(t, sql, "provider_key is not distinct from 'bepusdt'")
	require.Contains(t, sql, "payment_type is not distinct from 'bepusdt'")
	require.Contains(t, sql, "payment_type = 'bepusdt'")
	require.Contains(t, sql, "payment_currency = 'cny'")
	require.Contains(t, sql, "provider_snapshot ->> 'schema_version' = '2'")
	require.Contains(t, sql, "provider_snapshot ->> 'provider_key' = 'bepusdt'")
	require.Contains(t, sql, "provider_snapshot ->> 'provider_instance_id' = provider_instance_id")
	require.Contains(t, sql, "provider_snapshot ->> 'currency' = 'cny'")
	require.Contains(t, sql, "provider_snapshot::text !~*")
	require.Contains(t, sql, "api[_]?token|notify[_]?url|return[_]?url")

	predeployCheck, err := os.ReadFile("../scripts/bepusdt-predeploy-check.sql")
	require.NoError(t, err)
	predeploySQL := strings.ToLower(string(predeployCheck))
	require.Contains(t, predeploySQL, "252_bepusdt_payment_contract.sql")
	require.Contains(t, predeploySQL, "missing_or_unvalidated_bepusdt_constraint")
	require.Contains(t, predeploySQL, "convalidated")
}
