package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestAdminBalanceFlowHistoryReturnsPlayRewardsAndBlindboxDetails(t *testing.T) {
	t.Parallel()

	client, mock, cleanup := newAdminBalanceFlowMockClient(t)
	defer cleanup()

	userID := int64(301)
	now := time.Date(2026, 7, 19, 10, 30, 0, 0, time.UTC)

	expectBalanceTransactionsProbe(mock, userID, "", true, false)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance::double precision, COALESCE(frozen_balance, 0)::double precision")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(1.0, 0.0))
	mock.ExpectQuery("(?s)WITH flow_union AS .*COUNT\\(\\*\\)::bigint").
		WithArgs(userID, "").
		WillReturnRows(sqlmock.NewRows([]string{"count", "total_in", "total_out", "net_delta", "recharge_total"}).
			AddRow(int64(3), 1.0, 0.5, 0.5, 0.0))
	mock.ExpectQuery("(?s)WITH flow_union AS .*SELECT\\s+flow_id,\\s+type,\\s+source_type").
		WithArgs(userID, "", 0, 15).
		WillReturnRows(adminBalanceFlowRows().
			AddRow(
				"play_reward:12", "blindbox", "play_reward_ledger", "12",
				0.0, 0.0, 0.0, nil, nil, nil, nil, now,
				"盲盒净变动", "system", nil, "play_blindbox_open", "88", "blindbox:301:2026-07-19",
				"", `{"blindbox_open_id":88,"cost_amount":0.5,"reward_amount":0.5,"net_amount":0}`,
				"high",
			).
			AddRow(
				"play_reward:11", "quiz", "play_reward_ledger", "11",
				0.5, 0.5, 0.0, nil, nil, nil, nil, now.Add(-time.Minute),
				"答题奖励", "system", nil, "play_reward_ledger", "11", "quiz:301:2026-07-19",
				"", `{"attempt_date":"2026-07-19"}`,
				"high",
			).
			AddRow(
				"play_reward:10", "checkin", "play_reward_ledger", "10",
				0.5, 0.5, 0.0, nil, nil, nil, nil, now.Add(-2*time.Minute),
				"签到奖励", "system", nil, "play_reward_ledger", "10", "checkin:301:2026-07-19",
				"", `{"checkin_date":"2026-07-19"}`,
				"high",
			))

	svc := &adminServiceImpl{entClient: client}
	got, err := svc.GetUserBalanceHistory(context.Background(), userID, 1, 15, "")
	require.NoError(t, err)
	require.Len(t, got.Items, 3)
	require.Equal(t, 1.0, got.Summary.CurrentBalance)
	require.Equal(t, 1.0, got.Summary.TotalIn)
	require.Equal(t, 0.5, got.Summary.TotalOut)
	require.Equal(t, "blindbox", got.Items[0].Type)
	require.Equal(t, "play_blindbox_open", got.Items[0].RelatedObjectType)
	require.Equal(t, float64(0.5), got.Items[0].Metadata["cost_amount"])
	require.Equal(t, float64(0.5), got.Items[0].Metadata["reward_amount"])
	require.Equal(t, "quiz", got.Items[1].Type)
	require.Equal(t, 0.5, got.Items[1].BalanceDelta)
	require.Equal(t, "checkin", got.Items[2].Type)
	require.Equal(t, 0.5, got.Items[2].BalanceDelta)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminBalanceFlowHistoryPrefersBalanceTransactions(t *testing.T) {
	t.Parallel()

	client, mock, cleanup := newAdminBalanceFlowMockClient(t)
	defer cleanup()

	userID := int64(302)
	now := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)

	expectBalanceTransactionsProbe(mock, userID, "", true, true)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance::double precision, COALESCE(frozen_balance, 0)::double precision")).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(2.0, 0.0))
	mock.ExpectQuery("(?s)WITH filtered AS .*FROM balance_transactions.*COUNT\\(\\*\\)::bigint").
		WithArgs(userID, "").
		WillReturnRows(sqlmock.NewRows([]string{"count", "total_in", "total_out", "net_delta", "recharge_total"}).
			AddRow(int64(1), 1.0, 0.0, 1.0, 1.0))
	mock.ExpectQuery("(?s)WITH filtered AS .*FROM balance_transactions.*SELECT\\s+flow_id,\\s+type,\\s+source_type").
		WithArgs(userID, "", 0, 15).
		WillReturnRows(adminBalanceFlowRows().AddRow(
			"balance_transaction:99", "payment_recharge", "balance_transactions", "77",
			1.0, 1.0, 0.0, 1.0, 2.0, 0.0, 0.0, now,
			"订单充值", "system", nil, "payment_recharge", "77", "payment_order:77",
			"", `{"order_id":77}`, "high",
		))

	svc := &adminServiceImpl{entClient: client}
	got, err := svc.GetUserBalanceHistory(context.Background(), userID, 1, 15, "")
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, "payment_recharge", got.Items[0].Type)
	require.Equal(t, "balance_transactions", got.Items[0].SourceType)
	require.Equal(t, "77", got.Items[0].SourceID)
	require.Equal(t, 1.0, *got.Items[0].BalanceBefore)
	require.Equal(t, 2.0, *got.Items[0].BalanceAfter)
	require.Equal(t, 1.0, got.Summary.RechargeTotal)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminBalanceFlowCTEContracts(t *testing.T) {
	t.Parallel()

	require.Contains(t, adminBalanceFlowCTE, "prl.source <> 'team_affiliate_bonus'")
	require.Contains(t, adminBalanceFlowCTE, "($2 = '' AND affects_balance = TRUE)")
	require.Contains(t, adminBalanceFlowCTE, "'usage_charge'::text AS type")
	require.Contains(t, adminBalanceFlowCTE, "'refund'::text AS type")
	require.Contains(t, adminBalanceFlowCTE, "regexp_match(")
	require.NotContains(t, adminBalanceFlowCTE, "detail::jsonb")
}

func newAdminBalanceFlowMockClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	return client, mock, func() {
		_ = client.Close()
		_ = db.Close()
	}
}

func expectBalanceTransactionsProbe(mock sqlmock.Sqlmock, userID int64, flowType string, tableExists bool, hasRows bool) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT to_regclass('public.balance_transactions') IS NOT NULL")).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tableExists))
	if tableExists {
		mock.ExpectQuery("(?s)SELECT EXISTS \\(\\s+SELECT 1\\s+FROM balance_transactions").
			WithArgs(userID, flowType).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(hasRows))
	}
}

func adminBalanceFlowRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"flow_id",
		"type",
		"source_type",
		"source_id",
		"amount",
		"balance_delta",
		"frozen_delta",
		"balance_before",
		"balance_after",
		"frozen_before",
		"frozen_after",
		"occurred_at",
		"description",
		"actor_type",
		"actor_user_id",
		"related_object_type",
		"related_object_id",
		"reference",
		"notes",
		"metadata",
		"confidence",
	})
}
