package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPublicTeamRewardShowcaseOnlyReadsCompletedPaidAllocations(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)
	paidAt := time.Date(2026, time.August, 1, 0, 10, 0, 0, time.UTC)
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)FROM play_team_reward_allocations a.*JOIN play_team_settlements s.*JOIN play_teams t.*JOIN users u.*s\.status = 'completed'.*a\.payout_status = 'paid'.*a\.reward_amount > 0.*LIMIT \$1`).
		WithArgs(50).
		WillReturnRows(newPlayRewardShowcaseRows().AddRow(
			int64(71), periodStart, "星火小队", int64(501), "", "winner@example.com", "", 14.16, paidAt,
		))

	rows, err := repo.ListPublicTeamRewardWinners(context.Background(), 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "wi***@example.com", rows[0].DisplayName)
	require.Equal(t, "星火小队", rows[0].TeamName)
	require.InDelta(t, 14.16, rows[0].Amount, 0.00000001)
	require.Equal(t, &paidAt, rows[0].PaidAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonthlyArenaRewardShowcaseReadsSettledMonthlyLedger(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)
	settledAt := time.Date(2026, time.August, 1, 0, 10, 0, 0, time.UTC)
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)FROM play_arena_periods.*period_type = 'monthly'.*status = 'settled'.*settled_at DESC`).
		WillReturnRows(newPlayArenaPeriodRows().AddRow(int64(44), "2026-07 月榜", start, end, "settled", "monthly", settledAt))

	period, err := repo.GetLatestSettledMonthlyArenaPeriod(context.Background())
	require.NoError(t, err)
	require.NotNil(t, period)
	require.Equal(t, "monthly", period.PeriodType)

	mock.ExpectQuery(`(?s)FROM play_reward_ledger prl.*prl\.source = \$1.*period_id.*arena_settlement`).
		WithArgs(service.PlayRewardSourceArenaSettlement, int64(44)).
		WillReturnRows(newPlayArenaRewardLedgerRows().AddRow(int64(501), "Mira", "", "", 20.0, 1, int64(12000), settledAt))

	rows, err := repo.ListArenaMonthlyRewardLedger(context.Background(), 44)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "Mira", rows[0].DisplayName)
	require.Equal(t, 1, rows[0].Rank)
	require.InDelta(t, 20.0, rows[0].Amount, 0.00000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newPlayRewardShowcaseRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"settlement_id", "period_start", "team_name", "user_id", "username", "email", "avatar_url", "amount", "paid_at"})
}

func newPlayArenaPeriodRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "start_at", "end_at", "status", "period_type", "settled_at"})
}

func newPlayArenaRewardLedgerRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"user_id", "username", "email", "avatar_url", "amount", "rank", "token_sum", "created_at"})
}
