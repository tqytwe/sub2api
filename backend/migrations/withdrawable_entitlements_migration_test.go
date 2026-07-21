package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithdrawableEntitlementsMigrationContract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("210_withdrawable_entitlements.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "withdrawable_balance NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "withdrawal_frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "withdrawal_recalc_status VARCHAR(24) NOT NULL DEFAULT 'needs_review'")
	require.Contains(t, sql, "withdrawable_delta NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "withdrawal_frozen_delta NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS withdrawable_entitlements")
	require.Contains(t, sql, "original_amount NUMERIC(20,8) NOT NULL")
	require.Contains(t, sql, "remaining_amount NUMERIC(20,8) NOT NULL")
	require.Contains(t, sql, "available_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "original_amount = remaining_amount + consumed_amount + withdrawal_frozen_amount")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS withdrawable_entitlement_allocations")
	require.Contains(t, sql, "action IN ('grant', 'consume', 'restore', 'freeze', 'unfreeze', 'recompute_adjustment')")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS withdrawable_recalculation_runs")
	require.Contains(t, sql, "status IN ('ready', 'needs_review')")
	require.Contains(t, sql, "Task and image reservation balance only")
}
