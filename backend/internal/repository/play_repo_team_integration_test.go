//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type teamLifecycleSettingRepo struct {
	service.SettingRepository
}

func (teamLifecycleSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		switch key {
		case service.SettingKeyPlayAgentTeamEnabled:
			values[key] = "true"
		case service.SettingKeyPlayTeamAffiliateEnabled:
			values[key] = "false"
		}
	}
	return values, nil
}

type failingTeamEventRepository struct {
	service.PlayRepository
	failType string
}

func (r failingTeamEventRepository) InsertTeamEvent(ctx context.Context, event service.PlayTeamEvent) error {
	if r.failType == "" || r.failType == event.Type {
		return errors.New("forced team event failure")
	}
	return r.PlayRepository.InsertTeamEvent(ctx, event)
}

type teamLifecycleFixture struct {
	t       *testing.T
	ctx     context.Context
	marker  string
	repo    service.PlayRepository
	service *service.PlayService
}

func newTeamLifecycleFixture(t *testing.T) *teamLifecycleFixture {
	t.Helper()
	marker := fmt.Sprintf("team-lifecycle-%d", time.Now().UnixNano())
	repo := NewPlayRepository(testEntClient(t), integrationDB)
	settings := service.NewSettingService(teamLifecycleSettingRepo{}, nil)
	fixture := &teamLifecycleFixture{
		t:       t,
		ctx:     context.Background(),
		marker:  marker,
		repo:    repo,
		service: service.NewPlayService(repo, nil, nil, settings, nil, testEntClient(t)),
	}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `
			DELETE FROM play_team_events
			WHERE team_id IN (SELECT id FROM play_teams WHERE name LIKE $1)`, marker+"%")
		_, _ = integrationDB.ExecContext(context.Background(), `
			DELETE FROM play_team_members
			WHERE team_id IN (SELECT id FROM play_teams WHERE name LIKE $1)`, marker+"%")
		_, _ = integrationDB.ExecContext(context.Background(),
			`DELETE FROM play_teams WHERE name LIKE $1`, marker+"%")
		_, _ = integrationDB.ExecContext(context.Background(),
			`DELETE FROM users WHERE email LIKE $1`, marker+"%@example.com")
	})
	return fixture
}

func (f *teamLifecycleFixture) user(label string) int64 {
	f.t.Helper()
	return mustCreateUser(f.t, testEntClient(f.t), &service.User{
		Email:        fmt.Sprintf("%s-%s@example.com", f.marker, label),
		PasswordHash: "hash",
	}).ID
}

func (f *teamLifecycleFixture) createTeam(captainID int64, suffix string) *service.PlayTeamSummary {
	f.t.Helper()
	team, err := f.service.CreateTeam(f.ctx, captainID, f.marker+"-"+suffix)
	require.NoError(f.t, err)
	return team
}

func (f *teamLifecycleFixture) failingService(eventType ...string) *service.PlayService {
	f.t.Helper()
	settings := service.NewSettingService(teamLifecycleSettingRepo{}, nil)
	failType := ""
	if len(eventType) > 0 {
		failType = eventType[0]
	}
	return service.NewPlayService(
		failingTeamEventRepository{PlayRepository: f.repo, failType: failType},
		nil,
		nil,
		settings,
		nil,
		testEntClient(f.t),
	)
}

func TestTeamMembershipLifecyclePreservesHistoryAndAuthorization(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	captainID := f.user("captain")
	memberID := f.user("member")
	otherID := f.user("other")
	team := f.createTeam(captainID, "history")

	_, err := f.service.JoinTeam(f.ctx, memberID, team.InviteCode)
	require.NoError(t, err)

	err = f.service.LeaveTeam(f.ctx, captainID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainMustTransfer)
	err = f.service.RemoveTeamMember(f.ctx, captainID, captainID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainCannotRemoveSelf)
	err = f.service.TransferTeamCaptain(f.ctx, memberID, captainID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainRequired)
	err = f.service.RemoveTeamMember(f.ctx, memberID, captainID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainRequired)
	err = f.service.TransferTeamCaptain(f.ctx, captainID, otherID)
	require.ErrorIs(t, err, service.ErrPlayTeamMemberNotFound)
	err = f.service.TransferTeamCaptain(f.ctx, captainID, captainID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainTransferSelf)

	require.NoError(t, f.service.TransferTeamCaptain(f.ctx, captainID, memberID))
	require.NoError(t, f.service.LeaveTeam(f.ctx, captainID))

	active, err := f.repo.GetUserTeam(f.ctx, captainID)
	require.NoError(t, err)
	require.Nil(t, active)

	var historicalRows, closedRows int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE left_at IS NOT NULL)
		FROM play_team_members
		WHERE team_id = $1 AND user_id = $2`, team.ID, captainID).
		Scan(&historicalRows, &closedRows))
	require.Equal(t, 1, historicalRows)
	require.Equal(t, 1, closedRows)

	_, err = f.service.JoinTeam(f.ctx, captainID, team.InviteCode)
	require.NoError(t, err)
	require.NoError(t, f.service.RemoveTeamMember(f.ctx, memberID, captainID))

	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE left_at IS NOT NULL)
		FROM play_team_members
		WHERE team_id = $1 AND user_id = $2`, team.ID, captainID).
		Scan(&historicalRows, &closedRows))
	require.Equal(t, 2, historicalRows)
	require.Equal(t, 2, closedRows)

	members, err := f.repo.ListTeamMembers(f.ctx, team.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, memberID, members[0].UserID)
}

func TestTeamArchiveRejectsOldInviteAndClosesMembership(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	captainID := f.user("captain")
	joinerID := f.user("joiner")
	team := f.createTeam(captainID, "archive")

	require.NoError(t, f.service.LeaveTeam(f.ctx, captainID))

	var archived bool
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT archived_at IS NOT NULL
		FROM play_teams
		WHERE id = $1`, team.ID).Scan(&archived))
	require.True(t, archived)

	var left bool
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT left_at IS NOT NULL
		FROM play_team_members
		WHERE team_id = $1 AND user_id = $2`, team.ID, captainID).Scan(&left))
	require.True(t, left)

	_, err := f.service.JoinTeam(f.ctx, joinerID, team.InviteCode)
	require.ErrorIs(t, err, service.ErrPlayTeamNotFound)
}

func TestTeamConcurrentJoinCreatesOneActiveMembership(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	captainA := f.user("captain-a")
	captainB := f.user("captain-b")
	joinerID := f.user("joiner")
	teamA := f.createTeam(captainA, "concurrent-a")
	teamB := f.createTeam(captainB, "concurrent-b")

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, invite := range []string{teamA.InviteCode, teamB.InviteCode} {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			<-start
			_, err := f.service.JoinTeam(f.ctx, joinerID, code)
			errs <- err
		}(invite)
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, conflict int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrPlayTeamAlreadyJoined):
			conflict++
		default:
			t.Fatalf("unexpected concurrent join error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)

	var activeRows int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_members
		WHERE user_id = $1 AND left_at IS NULL`, joinerID).Scan(&activeRows))
	require.Equal(t, 1, activeRows)
}

func TestTeamArchiveRequiresCaptainAndOneMember(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	captainID := f.user("captain")
	memberID := f.user("member")
	team := f.createTeam(captainID, "archive-auth")
	_, err := f.service.JoinTeam(f.ctx, memberID, team.InviteCode)
	require.NoError(t, err)

	err = f.service.ArchiveTeam(f.ctx, memberID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainRequired)
	err = f.service.ArchiveTeam(f.ctx, captainID)
	require.ErrorIs(t, err, service.ErrPlayTeamCaptainMustTransfer)

	var activeRows, archivedRows int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_members
		WHERE team_id = $1 AND left_at IS NULL`, team.ID).Scan(&activeRows))
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_teams
		WHERE id = $1 AND archived_at IS NOT NULL`, team.ID).Scan(&archivedRows))
	require.Equal(t, 2, activeRows)
	require.Zero(t, archivedRows)
}

func TestTeamConcurrentCreateRollsBackLosingTeam(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	captainID := f.user("captain")

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, suffix := range []string{"concurrent-create-a", "concurrent-create-b"} {
		wg.Add(1)
		go func(teamSuffix string) {
			defer wg.Done()
			<-start
			_, err := f.service.CreateTeam(f.ctx, captainID, f.marker+"-"+teamSuffix)
			errs <- err
		}(suffix)
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, conflict int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrPlayTeamAlreadyJoined):
			conflict++
		default:
			t.Fatalf("unexpected concurrent create error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)

	var teams, activeRows int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_teams
		WHERE name IN ($1, $2)`,
		f.marker+"-concurrent-create-a",
		f.marker+"-concurrent-create-b",
	).Scan(&teams))
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_members
		WHERE user_id = $1 AND left_at IS NULL`, captainID).Scan(&activeRows))
	require.Equal(t, 1, teams)
	require.Equal(t, 1, activeRows)
}

func TestTeamLifecycleEventsAreTypedAndContainNoInviteCode(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	captainID := f.user("captain")
	memberID := f.user("member")
	team := f.createTeam(captainID, "events")
	_, err := f.service.JoinTeam(f.ctx, memberID, team.InviteCode)
	require.NoError(t, err)
	require.NoError(t, f.service.TransferTeamCaptain(f.ctx, captainID, memberID))
	require.NoError(t, f.service.RemoveTeamMember(f.ctx, memberID, captainID))

	rows, err := integrationDB.QueryContext(f.ctx, `
		SELECT event_type, detail::text
		FROM play_team_events
		WHERE team_id = $1
		ORDER BY id`, team.ID)
	require.NoError(t, err)
	defer rows.Close()

	var eventTypes []string
	for rows.Next() {
		var eventType, detail string
		require.NoError(t, rows.Scan(&eventType, &detail))
		eventTypes = append(eventTypes, eventType)
		require.NotContains(t, detail, team.InviteCode)
		require.NotContains(t, detail, "password")
		require.NotContains(t, detail, "secret")
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{
		service.PlayTeamEventCreated,
		service.PlayTeamEventMemberJoined,
		service.PlayTeamEventCaptainTransferred,
		service.PlayTeamEventMemberRemoved,
	}, eventTypes)
}

func TestTeamLifecycleEventFailureRollsBackEveryMutation(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		captainID := f.user("captain")
		_, err := f.failingService().CreateTeam(f.ctx, captainID, f.marker+"-rollback-create")
		require.Error(t, err)

		var teams, memberships int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx,
			`SELECT COUNT(*) FROM play_teams WHERE name = $1`, f.marker+"-rollback-create").Scan(&teams))
		require.NoError(t, integrationDB.QueryRowContext(f.ctx,
			`SELECT COUNT(*) FROM play_team_members WHERE user_id = $1`, captainID).Scan(&memberships))
		require.Zero(t, teams)
		require.Zero(t, memberships)
	})

	t.Run("join", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		captainID := f.user("captain")
		memberID := f.user("member")
		team := f.createTeam(captainID, "rollback-join")
		_, err := f.failingService().JoinTeam(f.ctx, memberID, team.InviteCode)
		require.Error(t, err)

		var active int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE user_id = $1 AND left_at IS NULL`, memberID).Scan(&active))
		require.Zero(t, active)
	})

	t.Run("leave", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		captainID := f.user("captain")
		memberID := f.user("member")
		team := f.createTeam(captainID, "rollback-leave")
		_, err := f.service.JoinTeam(f.ctx, memberID, team.InviteCode)
		require.NoError(t, err)
		err = f.failingService().LeaveTeam(f.ctx, memberID)
		require.Error(t, err)

		var active int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`, team.ID, memberID).Scan(&active))
		require.Equal(t, 1, active)
	})

	t.Run("transfer", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		captainID := f.user("captain")
		memberID := f.user("member")
		team := f.createTeam(captainID, "rollback-transfer")
		_, err := f.service.JoinTeam(f.ctx, memberID, team.InviteCode)
		require.NoError(t, err)
		err = f.failingService().TransferTeamCaptain(f.ctx, captainID, memberID)
		require.Error(t, err)

		var actualCaptain int64
		require.NoError(t, integrationDB.QueryRowContext(f.ctx,
			`SELECT captain_user_id FROM play_teams WHERE id = $1`, team.ID).Scan(&actualCaptain))
		require.Equal(t, captainID, actualCaptain)
	})

	t.Run("remove", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		captainID := f.user("captain")
		memberID := f.user("member")
		team := f.createTeam(captainID, "rollback-remove")
		_, err := f.service.JoinTeam(f.ctx, memberID, team.InviteCode)
		require.NoError(t, err)
		err = f.failingService().RemoveTeamMember(f.ctx, captainID, memberID)
		require.Error(t, err)

		var active int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`, team.ID, memberID).Scan(&active))
		require.Equal(t, 1, active)
	})

	t.Run("archive", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		captainID := f.user("captain")
		team := f.createTeam(captainID, "rollback-archive")
		err := f.failingService(service.PlayTeamEventArchived).LeaveTeam(f.ctx, captainID)
		require.Error(t, err)

		var active, archived int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`, team.ID, captainID).Scan(&active))
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_teams
			WHERE id = $1 AND archived_at IS NOT NULL`, team.ID).Scan(&archived))
		require.Equal(t, 1, active)
		require.Zero(t, archived)
	})
}
