package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBalanceTransactionsMigrationKeepsLedgerConstraints(t *testing.T) {
	t.Parallel()

	sql, err := os.ReadFile("207_balance_transactions.sql")
	require.NoError(t, err)
	text := string(sql)

	require.Contains(t, text, "CREATE TABLE IF NOT EXISTS balance_transactions")
	require.Contains(t, text, "balance_delta NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, text, "balance_before NUMERIC(20,8)")
	require.Contains(t, text, "frozen_delta NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, text, "UNIQUE")
	require.Contains(t, text, "uq_balance_transactions_user_idempotency")
	require.Contains(t, text, "uq_balance_transactions_source")
	require.Contains(t, text, "ON balance_transactions(user_id, source_type, source_id)")
	require.Contains(t, text, "confidence IN ('high', 'medium', 'low', 'estimated', 'needs_review')")
	require.Contains(t, text, "is_backfilled BOOLEAN NOT NULL DEFAULT FALSE")
}

func TestBalanceTransactionsBackfillSkipsAlreadyInsertedRowsBeforeBatchLimit(t *testing.T) {
	t.Parallel()

	sql, err := os.ReadFile("../scripts/backfill-balance-transactions.sql")
	require.NoError(t, err)
	text := string(sql)

	require.Contains(t, text, "ON CONFLICT (user_id, idempotency_key) DO NOTHING")
	require.Contains(t, text, "NOT EXISTS (\n          SELECT 1\n          FROM balance_transactions bt")
	require.Contains(t, text, "bt.user_id = candidates.user_id")
	require.Contains(t, text, "bt.idempotency_key = candidates.idempotency_key")
	require.Contains(t, text, "LIMIT :batch_size")
}
