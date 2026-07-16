package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestTeamContributionUsesActualCostInsideMonthAndMembershipIntervals(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	windowStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, shanghai)
	windowEnd := windowStart.AddDate(0, 1, 0)

	mock.ExpectQuery(`(?is)
		FROM play_team_members m
		JOIN usage_logs ul
		  ON ul\.user_id = m\.user_id
		 AND ul\.actual_cost > 0
		 AND ul\.created_at >= \$2
		 AND ul\.created_at < \$3
		 AND ul\.created_at >= m\.joined_at
		 AND \(m\.left_at IS NULL OR ul\.created_at < m\.left_at\)
		WHERE m\.team_id = \$1
		  AND m\.joined_at < \$3
		  AND \(m\.left_at IS NULL OR m\.left_at > \$2\)
		GROUP BY m\.user_id`).
		WithArgs(int64(7), windowStart, windowEnd).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "contribution"}).
			AddRow(int64(11), "30.00000000").
			AddRow(int64(19), "5.12345678"))

	got, err := repo.ListTeamRewardContributions(
		context.Background(),
		7,
		windowStart,
		windowEnd,
	)

	require.NoError(t, err)
	require.Equal(t, []service.TeamContribution{
		{UserID: 11, Amount: decimal.RequireFromString("30.00000000")},
		{UserID: 19, Amount: decimal.RequireFromString("5.12345678")},
	}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamSettlementSnapshotReturnsExistingWithoutRecreatingAllocations(t *testing.T) {
	db, mock, client := newTeamRewardRepositoryTestClient(t)
	_ = db
	repo := &playRepository{client: client, sql: db}
	periodStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	windowStart := time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, time.June, 30, 16, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?is)INSERT INTO play_team_settlements.*ON CONFLICT \(team_id, period_start\) DO NOTHING.*RETURNING id`).
		WithArgs(
			int64(7),
			"2026-06-01",
			windowStart,
			windowEnd,
			"999.00000000",
			"500.00000000",
			"0.04000000",
			"39.96000000",
			"250.00000000",
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`(?is)SELECT.*FROM play_team_settlements.*WHERE team_id = \$1 AND period_start = \$2`).
		WithArgs(int64(7), "2026-06-01").
		WillReturnRows(teamSettlementRows().
			AddRow(
				int64(41),
				int64(7),
				periodStart,
				windowStart,
				windowEnd,
				"30.00000000",
				"20.00000000",
				"0.02000000",
				"0.60000000",
				"250.00000000",
				service.PlayTeamSettlementStatusCompleted,
				nil,
				windowEnd,
				windowEnd,
			))
	mock.ExpectCommit()

	got, created, err := repo.CreateTeamRewardSnapshot(
		context.Background(),
		service.PlayTeamSettlement{
			TeamID:           7,
			PeriodStart:      periodStart,
			WindowStart:      windowStart,
			WindowEnd:        windowEnd,
			TeamSpend:        decimal.NewFromInt(999),
			ReachedThreshold: decimal.NewFromInt(500),
			RewardRate:       decimal.RequireFromString("0.04"),
			PoolAmount:       decimal.RequireFromString("39.96"),
			CapAmount:        decimal.NewFromInt(250),
		},
		[]service.PlayTeamRewardAllocation{{
			UserID:         11,
			Contribution:   decimal.NewFromInt(999),
			Ratio:          decimal.NewFromInt(1),
			RewardAmount:   decimal.RequireFromString("39.96"),
			PayoutStatus:   service.PlayTeamRewardAllocationStatusPending,
			IdempotencyKey: "team_reward:7:2026-06:11",
		}},
	)

	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(41), got.ID)
	require.Equal(t, "30.00000000", got.TeamSpend.StringFixed(8))
	require.Equal(t, service.PlayTeamSettlementStatusCompleted, got.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTeamSettlementSnapshotRollsBackSettlementWhenAllocationInsertFails(t *testing.T) {
	db, mock, client := newTeamRewardRepositoryTestClient(t)
	repo := &playRepository{client: client, sql: db}
	periodStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	windowStart := time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, time.June, 30, 16, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?is)INSERT INTO play_team_settlements .* RETURNING id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mock.ExpectExec(`(?is)INSERT INTO play_team_reward_allocations`).
		WillReturnError(errors.New("allocation insert failed"))
	mock.ExpectRollback()

	_, created, err := repo.CreateTeamRewardSnapshot(
		context.Background(),
		service.PlayTeamSettlement{
			TeamID:           7,
			PeriodStart:      periodStart,
			WindowStart:      windowStart,
			WindowEnd:        windowEnd,
			TeamSpend:        decimal.NewFromInt(30),
			ReachedThreshold: decimal.NewFromInt(20),
			RewardRate:       decimal.RequireFromString("0.02"),
			PoolAmount:       decimal.RequireFromString("0.6"),
			CapAmount:        decimal.NewFromInt(250),
		},
		[]service.PlayTeamRewardAllocation{{
			UserID:         11,
			Contribution:   decimal.NewFromInt(30),
			Ratio:          decimal.NewFromInt(1),
			RewardAmount:   decimal.RequireFromString("0.6"),
			PayoutStatus:   service.PlayTeamRewardAllocationStatusPending,
			IdempotencyKey: "team_reward:7:2026-06:11",
		}},
	)

	require.ErrorContains(t, err, "allocation insert failed")
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newTeamRewardRepositoryTestClient(
	t *testing.T,
) (*sql.DB, sqlmock.Sqlmock, *dbent.Client) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return db, mock, client
}

func teamSettlementRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"team_id",
		"period_start",
		"window_start",
		"window_end",
		"team_spend",
		"reached_threshold",
		"reward_rate",
		"pool_amount",
		"cap_amount",
		"status",
		"last_error",
		"processing_started_at",
		"completed_at",
	})
}
