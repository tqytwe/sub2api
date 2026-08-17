package scripts_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageQuotaRetrofitScriptIsExplicitAndIdempotent(t *testing.T) {
	contents, err := os.ReadFile("retrofit-package-entitlements.sql")
	require.NoError(t, err)
	script := string(contents)

	require.Contains(t, script, "\\set dry_run true")
	require.Contains(t, script, "331")
	require.Contains(t, script, "332")
	require.Contains(t, script, "PACKAGE_QUOTA_RETROFIT")
	require.Contains(t, script, "ON CONFLICT (payment_order_id) DO NOTHING")
	require.Contains(t, script, "request_limit")
	require.Contains(t, script, "amount_limit_usd")
	require.Contains(t, script, "token_limit")
	require.Contains(t, script, "usage_logs")
	require.True(t, strings.Contains(script, "ROLLBACK"), "dry-run must not commit writes")
	require.Contains(t, script, "WHERE existing_entitlement_id IS NULL\n          AND (")
}
