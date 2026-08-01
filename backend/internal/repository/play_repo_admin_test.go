package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListAdminTeamsUsesCurrentWindowScoreArguments(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &playRepository{sql: db}
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	mock.ExpectQuery(`(?is)SELECT COUNT\(\*\).*FROM play_teams t`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?is)GREATEST\(m\.joined_at, m\.reward_eligible_at, \$1\).*team_usage AS.*actual_cost > 0.*FROM play_teams t`).
		WithArgs(start, end, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "invite_code", "captain_user_id", "captain_username", "captain_email", "captain_avatar_url",
			"member_count", "token_sum", "team_spend", "created_at", "archived_at",
		}).AddRow(int64(7), "Team", "internal-code", int64(11), "captain", "captain@example.com", "", 2, int64(99), "30.00000000", start, nil))

	items, total, err := repo.ListAdminTeams(context.Background(), "active", "", start, end, 20, 0)

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "30.00000000", items[0].TeamSpend.StringFixed(8))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCountTeamRewardSettlementsNeedingAttentionUsesSettlementTable(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	mock.ExpectQuery(`(?is)FROM play_team_settlements\s+WHERE status IN \('pending', 'processing', 'partial', 'failed'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	got, err := repo.CountTeamRewardSettlementsNeedingAttention(context.Background())

	require.NoError(t, err)
	require.Equal(t, 3, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
