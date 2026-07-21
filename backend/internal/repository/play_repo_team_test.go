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
			WillReturnRows(sqlmock.NewRows([]string{
				"user_id",
				"username",
				"email",
				"avatar_url",
				"joined_at",
				"latest_settlement_month",
				"latest_actual_reward",
				"latest_payout_status",
				"paid_at",
			}).AddRow(int64(7), "USER", "captain@example.com", "", now, "", "0.00000000", "", nil))

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

func TestAdminTeamRepairRepositoryUsesExplicitEffectiveTimestamp(t *testing.T) {
	effectiveAt := time.Date(2026, time.July, 10, 8, 0, 0, 0, time.UTC)

	t.Run("join", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectExec(`(?s)INSERT INTO play_team_members.*team_id.*user_id.*joined_at.*VALUES \(\$1, \$2, \$3\)`).
			WithArgs(int64(11), int64(7), effectiveAt).
			WillReturnResult(sqlmock.NewResult(1, 1))

		require.NoError(t, repo.JoinTeamAt(context.Background(), 11, 7, effectiveAt))
	})

	t.Run("close", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectExec(`(?s)UPDATE play_team_members.*SET left_at = \$2.*WHERE id = \$1.*left_at IS NULL`).
			WithArgs(int64(15), effectiveAt).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, repo.CloseTeamMembershipAt(context.Background(), 15, effectiveAt))
	})

	t.Run("archive", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectExec(`(?s)UPDATE play_teams.*SET archived_at = \$2.*WHERE id = \$1.*archived_at IS NULL`).
			WithArgs(int64(11), effectiveAt).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, repo.ArchiveTeamAt(context.Background(), 11, effectiveAt))
	})
}

func TestAdminTeamRepairRepositoryReadsArchivedSourceAndSafeEvents(t *testing.T) {
	t.Run("reads active membership before deterministic team locking", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		joinedAt := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
		mock.ExpectQuery(`(?s)SELECT id, team_id, user_id, joined_at.*FROM play_team_members.*user_id = \$1.*left_at IS NULL`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "team_id", "user_id", "joined_at"}).
				AddRow(int64(15), int64(11), int64(7), joinedAt))

		membership, err := repo.GetActiveTeamMembership(context.Background(), 7)
		require.NoError(t, err)
		require.Equal(t, int64(15), membership.ID)
		require.Equal(t, int64(11), membership.TeamID)
		require.Equal(t, joinedAt, membership.JoinedAt)
	})

	t.Run("searches candidates with membership and affiliate context", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		createdAt := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
		joinedAt := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
		mock.ExpectQuery(`(?s)FROM users u.*LEFT JOIN play_team_members m.*LEFT JOIN play_teams t.*LEFT JOIN user_affiliates ua.*LOWER\(COALESCE\(u\.email, ''\)\).*LIMIT \$2`).
			WithArgs("member@example.com", 20).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "username", "email", "status", "created_at", "membership_id", "joined_at",
				"team_id", "team_name", "captain_user_id", "archived_at",
				"inviter_id", "inviter_username", "inviter_email",
			}).AddRow(
				int64(7), "member", "member@example.com", service.StatusActive,
				createdAt,
				int64(15), joinedAt, int64(11), "Source", int64(7), nil,
				int64(99), "inviter", "inviter@example.com",
			))

		candidates, err := repo.ListAdminTeamMemberCandidates(context.Background(), 22, "MEMBER@example.com", 50)
		require.NoError(t, err)
		require.Len(t, candidates, 1)
		require.Equal(t, int64(7), candidates[0].UserID)
		require.Equal(t, createdAt, candidates[0].CreatedAt)
		require.Equal(t, int64(15), candidates[0].CurrentMembershipID)
		require.Equal(t, int64(11), candidates[0].CurrentTeam.ID)
		require.True(t, candidates[0].IsCaptain)
		require.Equal(t, int64(99), candidates[0].Affiliate.InviterUserID)
	})

	t.Run("keeps soft-deleted users visible as blocked repair candidates", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		createdAt := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
		mock.ExpectQuery(`(?s)SELECT u\.id.*CASE WHEN u\.deleted_at IS NOT NULL THEN 'deleted' ELSE u\.status END.*FROM users u.*WHERE.*u\.id::text = \$1`).
			WithArgs("deleted@example.com", 20).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "username", "email", "status", "created_at", "membership_id", "joined_at",
				"team_id", "team_name", "captain_user_id", "archived_at",
				"inviter_id", "inviter_username", "inviter_email",
			}).AddRow(
				int64(8), "deleted-user", "deleted@example.com", "deleted",
				createdAt,
				nil, nil, nil, nil, nil, nil, nil, "", "",
			))

		candidates, err := repo.ListAdminTeamMemberCandidates(context.Background(), 22, "deleted@example.com", 20)
		require.NoError(t, err)
		require.Len(t, candidates, 1)
		require.Equal(t, "deleted", candidates[0].Status)
	})

	t.Run("locks archived team without filtering it out", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		createdAt := time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC)
		archivedAt := time.Date(2026, time.July, 5, 8, 0, 0, 0, time.UTC)
		mock.ExpectQuery(`(?s)FROM play_teams.*WHERE id = \$1.*FOR UPDATE`).
			WithArgs(int64(11)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "captain_user_id", "invite_code", "created_at", "archived_at"}).
				AddRow(int64(11), "Archived", int64(7), "SECRET-CODE", createdAt, archivedAt))

		team, err := repo.LockTeamForAdmin(context.Background(), 11)
		require.NoError(t, err)
		require.Equal(t, createdAt, team.CreatedAt)
		require.NotNil(t, team.ArchivedAt)
	})

	t.Run("lists typed event data without invite codes", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		createdAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
		mock.ExpectQuery(`(?s)FROM play_team_events e.*JOIN users actor.*LEFT JOIN users subject.*e\.team_id = \$1`).
			WithArgs(int64(11), 100).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "team_id", "actor_user_id", "actor_username", "actor_email",
				"subject_user_id", "subject_username", "subject_email", "event_type", "detail", "created_at",
			}).AddRow(
				int64(1), int64(11), int64(99), "admin", "admin@example.com",
				int64(7), "member", "member@example.com", service.PlayTeamEventAdminMemberAdded,
				[]byte(`{
					"reason":"manual repair token=SECRET-TOKEN-123456",
					"invite_code":"SECRET-CODE",
					"token":"SECRET-TOKEN",
					"operation":{"token":"NESTED-SECRET-TOKEN"},
					"target_team_id":{"invite_code":"NESTED-SECRET-CODE"}
				}`), createdAt,
			))

		events, err := repo.ListAdminTeamEvents(context.Background(), 11, 100)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, "admin", events[0].ActorDisplayName)
		require.Equal(t, "member", events[0].SubjectDisplayName)
		require.NotContains(t, events[0].Detail, "invite_code")
		require.NotContains(t, events[0].Detail, "token")
		require.NotContains(t, events[0].Detail, "operation")
		require.NotContains(t, events[0].Detail, "target_team_id")
		require.Equal(t, "manual repair token=***", events[0].Detail["reason"])
	})
}

func TestAdminTeamRepairRepositoryDetectsSnapshotsAndHistoryOverlap(t *testing.T) {
	effectiveAt := time.Date(2026, time.July, 10, 8, 0, 0, 0, time.UTC)

	t.Run("settlement snapshot", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectQuery(`(?s)FROM play_team_settlements.*team_id = ANY\(\$1\).*window_start <= \$2.*window_end > \$2`).
			WithArgs(sqlmock.AnyArg(), effectiveAt).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := repo.HasTeamRewardSnapshotAt(context.Background(), []int64{10, 20}, effectiveAt)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("membership overlap", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectQuery(`(?s)FROM play_team_members.*user_id = \$1.*left_at IS NOT NULL.*left_at > \$2.*id <> \$3`).
			WithArgs(int64(7), effectiveAt, int64(15)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := repo.HasTeamMembershipOverlap(context.Background(), 7, effectiveAt, 15)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("captain change after effective time", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectQuery(`(?s)FROM play_team_events.*team_id = \$1.*event_type = \$2.*created_at > \$3`).
			WithArgs(int64(20), service.PlayTeamEventCaptainTransferred, effectiveAt).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := repo.HasTeamCaptainChangeAfter(context.Background(), 20, effectiveAt)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("other membership extends after effective time", func(t *testing.T) {
		repo, mock := newPlayTeamRepositoryMock(t)
		mock.ExpectQuery(`(?s)FROM play_team_members.*team_id = \$1.*id <> \$2.*left_at IS NULL OR left_at > \$3`).
			WithArgs(int64(20), int64(15), effectiveAt).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		exists, err := repo.HasOtherTeamMembershipAfter(context.Background(), 20, 15, effectiveAt)
		require.NoError(t, err)
		require.True(t, exists)
	})
}
