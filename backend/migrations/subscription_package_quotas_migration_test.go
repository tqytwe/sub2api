package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionPackageQuotasMigrationDefinesImmutableEntitlements(t *testing.T) {
	sql, err := FS.ReadFile("254_subscription_package_quotas.sql")
	require.NoError(t, err)
	text := string(sql)
	require.Contains(t, text, "request_limit BIGINT")
	require.Contains(t, text, "amount_limit_usd DECIMAL(20,10)")
	require.Contains(t, text, "token_limit BIGINT")
	require.Contains(t, text, "payment_order_id BIGINT NOT NULL UNIQUE")
	require.Contains(t, text, "plan_snapshot JSONB NOT NULL")
	require.Contains(t, text, "status IN ('active', 'exhausted', 'expired', 'revoked')")
}
