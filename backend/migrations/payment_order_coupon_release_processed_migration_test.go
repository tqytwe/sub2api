package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentOrderCouponReleaseProcessedMigrations(t *testing.T) {
	columnSQL, err := FS.ReadFile("255_payment_order_coupon_release_processed.sql")
	require.NoError(t, err)
	column := strings.ToLower(string(columnSQL))
	require.Contains(t, column, "coupon_lock_release_processed_at timestamptz null")
	require.Contains(t, column, "status in ('cancelled', 'expired', 'failed')")
	require.Contains(t, column, "paid_at is null")
	require.Contains(t, column, "coupon_id is not null")
	require.Contains(t, column, "locked_order_id = po.id")
	require.Contains(t, column, "coupon_lock_release_processed_at is null")

	indexSQL, err := FS.ReadFile("256_payment_order_coupon_release_processed_index_notx.sql")
	require.NoError(t, err)
	index := strings.ToLower(string(indexSQL))
	require.Contains(t, index, "create index concurrently if not exists")
	require.Contains(t, index, "on payment_orders (updated_at, id)")
	require.Contains(t, index, "coupon_lock_release_processed_at is null")
	require.Contains(t, index, "paid_at is null")
	require.Contains(t, index, "coupon_id is not null")
	require.Contains(t, index, "status in ('cancelled', 'expired', 'failed')")
}
