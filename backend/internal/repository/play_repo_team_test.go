package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newPlayTeamRepositoryMock(t *testing.T) (*playRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return &playRepository{sql: db}, mock
}

func TestTeamMembershipQueriesOnlyReturnActiveRows(t *testing.T) {
	t.Run("user team filters historical memberships and archived teams", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectQuery(`(?s)FROM play_teams t.*JOIN play_team_members m.*m\.user_id = \$1.*m\.left_at IS NULL.*t\.archived_at IS NULL`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "captain_user_id", "invite_code"}).
				AddRow(int64(11), "Active", int64(7), "ACTIVE11"))

		team, err := repo.GetUserTeam(context.Background(), 7)
		require.NoError(t, err)
		require.Equal(t, int64(11), team.ID)
	})

	t.Run("invite lookup filters archived teams", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectQuery(`(?s)FROM play_teams.*invite_code = \$1.*archived_at IS NULL`).
			WithArgs("ACTIVE11").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "captain_user_id", "invite_code"}))

		team, err := repo.GetTeamByInviteCode(context.Background(), "ACTIVE11")
		require.NoError(t, err)
		require.Nil(t, team)
	})

	t.Run("member list filters historical rows and archived teams", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		now := time.Now()
		mock.ExpectQuery(`(?s)FROM play_team_members m.*JOIN play_teams t.*m\.team_id = \$1.*m\.left_at IS NULL.*t\.archived_at IS NULL`).
			WithArgs(int64(11)).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "avatar_url", "joined_at"}).
				AddRow(int64(7), "USER", "captain@example.com", "", now))

		members, err := repo.ListTeamMembers(context.Background(), 11)
		require.NoError(t, err)
		require.Len(t, members, 1)
		require.Equal(t, int64(7), members[0].UserID)
		require.Equal(t, "ca***@example.com", members[0].DisplayName)
	})
}

func TestTeamMembershipRepositoryMutationsUseDatabaseTimestamp(t *testing.T) {
	t.Run("leave preserves membership row", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectExec(`(?s)UPDATE play_team_members.*SET left_at = NOW\(\).*team_id = \$1.*user_id = \$2.*left_at IS NULL`).
			WithArgs(int64(11), int64(7)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, repo.LeaveTeam(context.Background(), 11, 7))
	})

	t.Run("archive uses the transaction timestamp", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectExec(`(?s)UPDATE play_teams.*SET archived_at = NOW\(\).*id = \$1.*archived_at IS NULL`).
			WithArgs(int64(11)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, repo.ArchiveTeam(context.Background(), 11))
	})

	t.Run("event detail is structured json", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectExec(`(?s)INSERT INTO play_team_events.*team_id.*actor_user_id.*subject_user_id.*event_type.*detail`).
			WithArgs(int64(11), int64(7), int64(9), service.PlayTeamEventCaptainTransferred, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		require.NoError(t, repo.InsertTeamEvent(context.Background(), service.PlayTeamEvent{
			TeamID:        11,
			ActorUserID:   7,
			SubjectUserID: 9,
			Type:          service.PlayTeamEventCaptainTransferred,
			Detail:        map[string]any{"previous_captain_user_id": int64(7)},
		}))
	})
}
