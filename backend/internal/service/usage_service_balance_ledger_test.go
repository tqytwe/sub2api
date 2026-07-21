package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type usageServiceLedgerLogRepoStub struct {
	UsageLogRepository
	inserted  bool
	id        int64
	createdAt time.Time
}

func (r *usageServiceLedgerLogRepoStub) Create(_ context.Context, log *UsageLog) (bool, error) {
	log.ID = r.id
	log.CreatedAt = r.createdAt
	return r.inserted, nil
}

type usageServiceLedgerUserRepoStub struct {
	UserRepository
	updateBalanceCalls int
}

func (r *usageServiceLedgerUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id}, nil
}

func (r *usageServiceLedgerUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	r.updateBalanceCalls++
	return nil
}

func TestUsageServiceCreateWritesUsageChargeBalanceLedger(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()

	createdAt := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	usageRepo := &usageServiceLedgerLogRepoStub{inserted: true, id: 501, createdAt: createdAt}
	userRepo := &usageServiceLedgerUserRepoStub{}
	ledger := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt.Add(time.Second) }}
	svc := NewUsageService(usageRepo, userRepo, client, nil, ledger)

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "usage_log:501").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("5.00000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt.Add(time.Second), "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectQuery("(?s)FROM withdrawable_entitlements\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "available_at"}))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("4.25000000", "0.00000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WithArgs(
			int64(42),
			"-0.75000000",
			"5.00000000",
			"4.25000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"usage_charge",
			"501",
			"usage_log:501",
			"system",
			nil,
			"API 消耗扣费",
			sqlmock.AnyArg(),
			false,
			"high",
			createdAt,
		).
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(8001), int64(42), -0.75, 5.0, 4.25, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			"usage_charge", "501", "usage_log:501", "system", nil,
			"API 消耗扣费", `{"usage_log_id":501}`, false, "high", createdAt,
		))
	expectEmptyFundBatchConsumption(mock, 42)
	mock.ExpectCommit()

	got, err := svc.Create(context.Background(), CreateUsageLogRequest{
		UserID:         42,
		APIKeyID:       7,
		AccountID:      9,
		RequestID:      "req-usage-ledger",
		Model:          "gpt-test",
		InputTokens:    10,
		OutputTokens:   12,
		TotalCost:      1.5,
		ActualCost:     0.75,
		RateMultiplier: 0.5,
	})
	require.NoError(t, err)
	require.Equal(t, int64(501), got.ID)
	require.Zero(t, userRepo.updateBalanceCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}
