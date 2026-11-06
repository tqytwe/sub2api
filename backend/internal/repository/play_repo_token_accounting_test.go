package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const playTokenSumSQL = `(?s)(SUM|COALESCE\(SUM)\(input_tokens::bigint \+ output_tokens \+ cache_creation_tokens \+ cache_read_tokens\)`

func TestArenaTokenQueriesUseAllUsageTokenCategories(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { mock.ExpectClose(); require.NoError(t, db.Close()) }()
	repo := &playRepository{sql: db}
	ctx := context.Background()
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	mock.ExpectQuery(`(?s)WITH policy AS.*token_accounting_version.*CASE WHEN policy.version=2 THEN cache_read_tokens`).
		WithArgs(start, end, 10).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "avatar_url", "token_sum"}).
			AddRow(10, "user", "user@example.com", "", 20000))
	rows, err := repo.ListArenaLeaderboard(ctx, start, end, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 20000, rows[0].TokenSum)

	mock.ExpectQuery(`(?s)WITH policy AS.*SELECT m.token_sum.*s.user_id<m.user_id`).
		WithArgs(start, end, int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"token_sum", "rank"}).AddRow(20000, 1))
	tokenSum, rank, err := repo.GetUserArenaScore(ctx, 10, start, end)
	require.NoError(t, err)
	require.EqualValues(t, 20000, tokenSum)
	require.Equal(t, 1, rank)

	mock.ExpectQuery(`(?s)WITH policy AS.*LIMIT 1 OFFSET`).
		WithArgs(start, end, 0).
		WillReturnRows(sqlmock.NewRows([]string{"token_sum"}).AddRow(25000))
	gap, err := repo.GetArenaTokensToPrevRank(ctx, 10, start, end, 2, 20000)
	require.NoError(t, err)
	require.EqualValues(t, 5000, gap)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDailyQuestTokenQueryUsesAllUsageTokenCategories(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { mock.ExpectClose(); require.NoError(t, db.Close()) }()
	repo := &playRepository{sql: db}
	ctx := context.Background()
	start := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)

	mock.ExpectQuery(playTokenSumSQL).
		WithArgs(int64(10), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"token_sum"}).AddRow(20000))
	daily, err := repo.GetUserDailyTokenSum(ctx, 10, start, end)
	require.NoError(t, err)
	require.EqualValues(t, 20000, daily)

	require.NoError(t, mock.ExpectationsWereMet())
}
