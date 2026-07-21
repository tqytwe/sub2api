package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestFreezeFundRefundBatchKeepsFrozenBatchActive(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	plan := refundBatchPlan{BatchID: 7002, Amount: decimal.RequireFromString("30")}

	mock.ExpectExec("(?s)UPDATE balance_fund_batches\\s+SET remaining_amount = remaining_amount - \\$1::numeric,\\s+refund_frozen_amount = refund_frozen_amount \\+ \\$1::numeric,\\s+status = 'active'").
		WithArgs("30.00000000", int64(7002), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO fund_refund_request_batches").
		WithArgs(int64(8801), int64(7002), "30.00000000", createdAt.UTC()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("(?s)INSERT INTO balance_fund_allocations").
		WithArgs(int64(42), int64(7002), nil, fundAllocationRefundFreeze, "30.00000000", FundRefundLedgerSourceSubmit, "FR202607220001", sqlmock.AnyArg(), createdAt.UTC()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := freezeFundRefundBatch(context.Background(), db, 42, 8801, 0, plan, FundRefundLedgerSourceSubmit, "FR202607220001", createdAt)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
