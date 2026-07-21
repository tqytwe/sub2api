package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDailyArenaRewardSummaryRepositoryUsesSettledAtAndMasksLedgerWinners(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)
	settledAt := time.Date(2026, time.July, 21, 0, 8, 0, 0, time.UTC)
	start := time.Date(2026, time.July, 20, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.July, 21, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT id, name, start_at, end_at, status, .*period_type, settled_at.*period_type = 'daily'.*status = 'settled'.*settled_at DESC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "start_at", "end_at", "status", "period_type", "settled_at"}).
			AddRow(int64(44), "2026-07-20", start, end, "settled", "daily", settledAt))

	period, err := repo.GetLatestSettledDailyArenaPeriod(context.Background())
	require.NoError(t, err)
	require.NotNil(t, period)
	require.Equal(t, int64(44), period.ID)
	require.Equal(t, "daily", period.PeriodType)
	require.Equal(t, &settledAt, period.SettledAt)

	mock.ExpectQuery(`(?s)FROM play_reward_ledger prl.*JOIN users u.*LEFT JOIN user_avatars ua.*prl\.source = \$1.*period_id.*idempotency_key.*arena_daily_settlement`).
		WithArgs(service.PlayRewardSourceArenaDaily, int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id",
			"username",
			"email",
			"avatar_url",
			"amount",
			"rank",
			"token_sum",
			"created_at",
		}).AddRow(int64(501), "", "winner@example.com", "", 0.5, 1, int64(12000), settledAt))

	rows, err := repo.ListArenaDailyRewardLedger(context.Background(), 44)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(501), rows[0].UserID)
	require.Equal(t, "wi***@example.com", rows[0].DisplayName)
	require.Equal(t, 1, rows[0].Rank)
	require.Equal(t, int64(12000), rows[0].TokenSum)
	require.InDelta(t, 0.5, rows[0].Amount, 0.00000001)
}

func TestArenaPeriodQueriesExposePeriodTypeAndSettledAt(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)
	settledAt := time.Date(2026, time.July, 21, 0, 8, 0, 0, time.UTC)
	start := time.Date(2026, time.July, 20, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.July, 21, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT id, name, start_at, end_at, status, .*period_type, settled_at.*FROM play_arena_periods WHERE id = \$1`).
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "start_at", "end_at", "status", "period_type", "settled_at"}).
			AddRow(int64(44), "2026-07-20", start, end, "settled", "daily", settledAt))

	period, err := repo.GetArenaPeriodByID(context.Background(), 44)
	require.NoError(t, err)
	require.NotNil(t, period)
	require.Equal(t, "daily", period.PeriodType)
	require.Equal(t, &settledAt, period.SettledAt)

	mock.ExpectExec(`(?s)UPDATE play_arena_periods.*SET status = 'settled'.*settled_at = COALESCE\(settled_at, NOW\(\)\).*WHERE id = \$1 AND status = 'active'`).
		WithArgs(int64(44)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.MarkArenaPeriodSettled(context.Background(), 44))
}
