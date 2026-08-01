package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPublicTeamLeaderboardUsesEligibleMembershipWindowAndStableOrder(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	mock.ExpectQuery(`(?s)GREATEST\(m\.joined_at, m\.reward_eligible_at, \$1\).*actual_cost > 0.*t\.archived_at IS NULL.*ROW_NUMBER\(\) OVER \(ORDER BY spend DESC, team_id ASC\).*LIMIT \$3`).
		WithArgs(start, end, 50).
		WillReturnRows(sqlmock.NewRows([]string{"rank", "team_id", "team_name", "member_count", "spend", "gap_to_previous", "total_teams"}).
			AddRow(1, int64(11), "星火战队", 8, "120.50000000", "0.00000000", 2))

	rows, total, err := repo.ListPublicTeamLeaderboard(context.Background(), start, end, 50)

	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, rows, 1)
	require.Equal(t, int64(11), rows[0].TeamID)
	require.Equal(t, "120.50000000", rows[0].Spend.StringFixed(8))
}

func TestPublicTeamDirectoryNeverSelectsInviteCodeOrUserIdentity(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	mock.ExpectQuery(`(?s).*`).
		WithArgs(start, end, 20).
		WillReturnRows(sqlmock.NewRows([]string{"team_id", "team_name", "member_count", "is_recruiting", "monthly_spend"}).
			AddRow(int64(11), "星火战队", 8, true, "120.50000000"))

	rows, err := repo.ListPublicTeamDirectory(context.Background(), start, end, 20)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.True(t, rows[0].Recruiting)
	require.Equal(t, "120.50000000", rows[0].Spend.StringFixed(8))
}

func TestPublicTeamSeasonHistoryExcludesUnsettledSeasons(t *testing.T) {
	repo, mock := newPlayTeamRepositoryMock(t)

	mock.ExpectQuery(`(?is)FROM play_team_seasons s.*WHERE s\.status IN \('settled', 'legacy'\).*LIMIT \$1`).
		WithArgs(12).
		WillReturnRows(teamCompetitionSeasonRows())

	seasons, err := repo.ListPublicTeamSeasons(context.Background(), 12)

	require.NoError(t, err)
	require.Empty(t, seasons)
}

func TestTeamCompetitionSeasonSnapshotStaysSettlingUntilEveryPayoutIsPaid(t *testing.T) {
	db, mock, client := newTeamRewardRepositoryTestClient(t)
	repo := &playRepository{client: client, sql: db}
	periodStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	windowStart := time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, time.June, 30, 16, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?is)INSERT INTO play_team_seasons.*status.*active.*ON CONFLICT \(period_start\) DO NOTHING.*RETURNING`).
		WithArgs(periodStart.Format("2006-01-02"), windowStart, windowEnd, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "period_start", "window_start", "window_end", "rules_json", "status", "frozen_at", "settled_at"}))
	mock.ExpectQuery(`(?is)SELECT id, period_start, window_start, window_end.*FROM play_team_seasons.*WHERE period_start = \$1.*FOR UPDATE`).
		WithArgs(periodStart.Format("2006-01-02")).
		WillReturnRows(teamCompetitionSeasonRows().AddRow(
			int64(51), periodStart, windowStart, windowEnd,
			`{"team_reward":{"enabled":true,"cap":"250","tiers":[{"threshold":"20","rate":"0.02"}]}}`,
			"active", windowEnd, nil,
		))
	mock.ExpectExec(`(?is)UPDATE play_team_seasons.*status = 'settling'.*WHERE id = \$1`).
		WithArgs(int64(51)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?is)SELECT EXISTS.*FROM play_team_settlements s.*payout_status <> 'paid'`).
		WithArgs(periodStart.Format("2006-01-02")).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()

	published, err := repo.CreateTeamCompetitionSeasonSnapshot(context.Background(), periodStart, windowStart, windowEnd, map[string]any{
		"team_reward": map[string]any{"enabled": true},
	})

	require.NoError(t, err)
	require.False(t, published)
}

func TestTeamCompetitionSeasonSnapshotRebuildsAndPublishesOnlyAfterPayoutCompletion(t *testing.T) {
	db, mock, client := newTeamRewardRepositoryTestClient(t)
	repo := &playRepository{client: client, sql: db}
	periodStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	windowStart := time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, time.June, 30, 16, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?is)INSERT INTO play_team_seasons.*status.*active.*ON CONFLICT \(period_start\) DO NOTHING.*RETURNING`).
		WithArgs(periodStart.Format("2006-01-02"), windowStart, windowEnd, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "period_start", "window_start", "window_end", "rules_json", "status", "frozen_at", "settled_at"}))
	mock.ExpectQuery(`(?is)SELECT id, period_start, window_start, window_end.*FROM play_team_seasons.*WHERE period_start = \$1.*FOR UPDATE`).
		WithArgs(periodStart.Format("2006-01-02")).
		WillReturnRows(teamCompetitionSeasonRows().AddRow(
			int64(52), periodStart, windowStart, windowEnd,
			`{"team_reward":{"enabled":true,"cap":"250","tiers":[{"threshold":"20","rate":"0.02"}]}}`,
			"settling", windowEnd, nil,
		))
	mock.ExpectExec(`(?is)UPDATE play_team_seasons.*status = 'settling'.*WHERE id = \$1`).
		WithArgs(int64(52)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?is)SELECT EXISTS.*FROM play_team_settlements s.*payout_status <> 'paid'`).
		WithArgs(periodStart.Format("2006-01-02")).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?is)DELETE FROM play_team_season_rankings WHERE season_id = \$1`).
		WithArgs(int64(52)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`(?is)WITH ranked AS.*INSERT INTO play_team_season_rankings.*WHERE rank <= \$5`).
		WithArgs(periodStart.Format("2006-01-02"), windowStart, windowEnd, int64(52), 10).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?is)UPDATE play_team_seasons.*status = 'settled'.*WHERE id = \$1.*status = 'settling'`).
		WithArgs(int64(52)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	published, err := repo.CreateTeamCompetitionSeasonSnapshot(context.Background(), periodStart, windowStart, windowEnd, map[string]any{
		"team_reward": map[string]any{"enabled": true},
	})

	require.NoError(t, err)
	require.True(t, published)
}

func teamCompetitionSeasonRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"period_start",
		"window_start",
		"window_end",
		"rules_json",
		"status",
		"frozen_at",
		"settled_at",
	})
}
