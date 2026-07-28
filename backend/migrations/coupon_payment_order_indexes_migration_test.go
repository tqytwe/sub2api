package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCouponPaymentOrderIndexesUseOnlineMigration(t *testing.T) {
	settlement, err := FS.ReadFile("224_payment_order_coupon_settlement.sql")
	require.NoError(t, err)
	require.NotContains(t, strings.ToUpper(string(settlement)), "CREATE INDEX")

	indexes, err := FS.ReadFile("226_payment_order_coupon_indexes_notx.sql")
	require.NoError(t, err)
	sql := strings.ToUpper(strings.Join(strings.Fields(string(indexes)), " "))
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS IDX_PAYMENT_ORDERS_COUPON_ID")
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS IDX_PAYMENT_ORDERS_COUPON_TEMPLATE_ID")
	require.Contains(t, sql, "ON PAYMENT_ORDERS (COUPON_ID)")
	require.Contains(t, sql, "ON PAYMENT_ORDERS (COUPON_TEMPLATE_ID)")
}
