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
	"github.com/shopspring/decimal"
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
			DELETE FROM play_team_reward_allocations
			WHERE settlement_id IN (
				SELECT s.id
				FROM play_team_settlements s
				JOIN play_teams t ON t.id = s.team_id
				WHERE t.name LIKE $1
			)`, marker+"%")
		_, _ = integrationDB.ExecContext(context.Background(), `
			DELETE FROM play_team_settlements
			WHERE team_id IN (SELECT id FROM play_teams WHERE name LIKE $1)`, marker+"%")
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

func TestAdminTeamRepairConcurrentAddCreatesOneActiveMembership(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	actorID := f.user("repair-actor")
	captainA := f.user("repair-captain-a")
	captainB := f.user("repair-captain-b")
	memberID := f.user("repair-member")
	teamA := f.createTeam(captainA, "repair-add-a")
	teamB := f.createTeam(captainB, "repair-add-b")
	repairReason := "repair concurrent missing membership"

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, targetTeamID := range []int64{teamA.ID, teamB.ID} {
		wg.Add(1)
		go func(teamID int64) {
			defer wg.Done()
			<-start
			_, err := f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
				TargetTeamID: teamID,
				UserID:       memberID,
				ActorUserID:  actorID,
				Operation:    service.AdminTeamMemberOperationAdd,
				Reason:       repairReason,
			})
			errs <- err
		}(targetTeamID)
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, conflict int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrPlayAdminTeamMoveRequired):
			conflict++
		default:
			t.Fatalf("unexpected concurrent admin repair error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)

	var activeRows, repairEvents int
	var storedReason, reasonCode string
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_members
		WHERE user_id = $1 AND left_at IS NULL`, memberID).Scan(&activeRows))
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*), MIN(detail->>'reason'), MIN(detail->>'reason_code')
		FROM play_team_events
		WHERE subject_user_id = $1
		  AND event_type = $2`,
		memberID,
		service.PlayTeamEventAdminMemberAdded,
	).Scan(&repairEvents, &storedReason, &reasonCode))
	require.Equal(t, 1, activeRows)
	require.Equal(t, 1, repairEvents)
	require.Equal(t, repairReason, storedReason)
	require.Equal(t, service.PlayTeamEventReasonAdminManualMembershipRepair, reasonCode)
}

func TestAdminTeamRepairConcurrentCrossMoveUsesExactTimestampWithoutDeadlock(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	actorID := f.user("move-actor")
	captainA := f.user("move-captain-a")
	captainB := f.user("move-captain-b")
	memberA := f.user("move-member-a")
	memberB := f.user("move-member-b")
	teamA := f.createTeam(captainA, "repair-move-a")
	teamB := f.createTeam(captainB, "repair-move-b")
	_, err := f.service.JoinTeam(f.ctx, memberA, teamA.InviteCode)
	require.NoError(t, err)
	_, err = f.service.JoinTeam(f.ctx, memberB, teamB.InviteCode)
	require.NoError(t, err)

	effectiveAt := currentShanghaiAdminRepairTime(t)
	_, err = integrationDB.ExecContext(f.ctx, `
		UPDATE play_team_members
		SET joined_at = $2
		WHERE user_id IN ($1, $3)
		  AND left_at IS NULL`,
		memberA, effectiveAt.Add(-time.Hour), memberB)
	require.NoError(t, err)

	type move struct {
		userID   int64
		sourceID int64
		targetID int64
	}
	moves := []move{
		{userID: memberA, sourceID: teamA.ID, targetID: teamB.ID},
		{userID: memberB, sourceID: teamB.ID, targetID: teamA.ID},
	}
	repairReason := fmt.Sprintf(
		"repair concurrent cross team move %s token=super-secret-token invite_code=TEAM-SECRET-123",
		teamA.InviteCode,
	)
	safeReason := service.RedactAdminTeamRepairReason(repairReason)
	start := make(chan struct{})
	errs := make(chan error, len(moves))
	var wg sync.WaitGroup
	for _, item := range moves {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			sourceID := item.sourceID
			_, moveErr := f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
				TargetTeamID:         item.targetID,
				UserID:               item.userID,
				ActorUserID:          actorID,
				Operation:            service.AdminTeamMemberOperationMove,
				EffectiveAt:          &effectiveAt,
				Reason:               repairReason,
				ExpectedSourceTeamID: &sourceID,
			})
			errs <- moveErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for moveErr := range errs {
		require.NoError(t, moveErr)
	}

	for _, item := range moves {
		var oldLeftAt, newJoinedAt time.Time
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT
				MAX(left_at) FILTER (WHERE team_id = $2 AND left_at IS NOT NULL),
				MAX(joined_at) FILTER (WHERE team_id = $3 AND left_at IS NULL)
			FROM play_team_members
			WHERE user_id = $1`,
			item.userID, item.sourceID, item.targetID,
		).Scan(&oldLeftAt, &newJoinedAt))
		require.WithinDuration(t, effectiveAt, oldLeftAt, time.Microsecond)
		require.WithinDuration(t, effectiveAt, newJoinedAt, time.Microsecond)
		require.Truef(
			t,
			oldLeftAt.Equal(newJoinedAt),
			"source left_at %s must exactly equal target joined_at %s",
			oldLeftAt.Format(time.RFC3339Nano),
			newJoinedAt.Format(time.RFC3339Nano),
		)

		rows, queryErr := integrationDB.QueryContext(f.ctx, `
			SELECT team_id, detail->>'side', detail->>'reason', detail->>'reason_code'
			FROM play_team_events
			WHERE subject_user_id = $1
			  AND event_type = $2
			ORDER BY team_id`,
			item.userID,
			service.PlayTeamEventAdminMemberMoved,
		)
		require.NoError(t, queryErr)

		eventTeams := make(map[string]int64, 2)
		var eventCount int
		for rows.Next() {
			var teamID int64
			var side, storedReason, reasonCode string
			require.NoError(t, rows.Scan(&teamID, &side, &storedReason, &reasonCode))
			eventCount++
			eventTeams[side] = teamID
			require.Equal(t, safeReason, storedReason)
			require.Equal(t, service.PlayTeamEventReasonAdminManualMembershipRepair, reasonCode)
			require.NotContains(t, storedReason, teamA.InviteCode)
			require.NotContains(t, storedReason, "super-secret-token")
			require.NotContains(t, storedReason, "TEAM-SECRET-123")
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.Equal(t, 2, eventCount)
		require.Equal(t, item.sourceID, eventTeams["source"])
		require.Equal(t, item.targetID, eventTeams["target"])
	}
}

func TestAdminTeamRepairWaitsForSnapshotLockAndRejectsImmutableWindow(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	actorID := f.user("snapshot-actor")
	captainID := f.user("snapshot-captain")
	memberID := f.user("snapshot-member")
	team := f.createTeam(captainID, "snapshot-lock")
	effectiveAt := currentShanghaiAdminRepairTime(t)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	localEffective := effectiveAt.In(shanghai)
	windowStart := time.Date(localEffective.Year(), localEffective.Month(), 1, 0, 0, 0, 0, shanghai)
	windowEnd := windowStart.AddDate(0, 1, 0)
	periodStart := time.Date(localEffective.Year(), localEffective.Month(), 1, 0, 0, 0, 0, time.UTC)

	lockHeld := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	snapshotDone := make(chan error, 1)
	go func() {
		snapshotDone <- f.repo.WithTeamRewardSnapshotLock(f.ctx, team.ID, func(lockCtx context.Context) error {
			close(lockHeld)
			<-releaseSnapshot
			_, _, createErr := f.repo.CreateTeamRewardSnapshot(
				lockCtx,
				service.PlayTeamSettlement{
					TeamID:           team.ID,
					PeriodStart:      periodStart,
					WindowStart:      windowStart,
					WindowEnd:        windowEnd,
					TeamSpend:        decimal.NewFromInt(20),
					ReachedThreshold: decimal.NewFromInt(20),
					RewardRate:       decimal.RequireFromString("0.02"),
					PoolAmount:       decimal.RequireFromString("0.4"),
					CapAmount:        decimal.NewFromInt(250),
				},
				[]service.PlayTeamRewardAllocation{{
					UserID:         captainID,
					Contribution:   decimal.NewFromInt(20),
					Ratio:          decimal.NewFromInt(1),
					RewardAmount:   decimal.RequireFromString("0.4"),
					PayoutStatus:   service.PlayTeamRewardAllocationStatusPending,
					IdempotencyKey: fmt.Sprintf("team-repair-snapshot-lock:%s", f.marker),
				}},
			)
			return createErr
		})
	}()
	<-lockHeld

	repairDone := make(chan error, 1)
	go func() {
		_, repairErr := f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
			TargetTeamID: team.ID,
			UserID:       memberID,
			ActorUserID:  actorID,
			Operation:    service.AdminTeamMemberOperationAdd,
			EffectiveAt:  &effectiveAt,
			Reason:       "repair must wait for immutable snapshot",
		})
		repairDone <- repairErr
	}()

	select {
	case repairErr := <-repairDone:
		t.Fatalf("repair returned before snapshot lock released: %v", repairErr)
	case <-time.After(200 * time.Millisecond):
	}

	close(releaseSnapshot)
	require.NoError(t, <-snapshotDone)
	require.ErrorIs(t, <-repairDone, service.ErrPlayAdminTeamSettlementSnapshotExists)

	var activeMemberships int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_members
		WHERE user_id = $1 AND left_at IS NULL`, memberID).Scan(&activeMemberships))
	require.Zero(t, activeMemberships)
}

func TestAdminTeamRepairEventFailureRollsBackMembership(t *testing.T) {
	f := newTeamLifecycleFixture(t)
	actorID := f.user("rollback-repair-actor")
	captainID := f.user("rollback-repair-captain")
	memberID := f.user("rollback-repair-member")
	team := f.createTeam(captainID, "rollback-admin-repair")

	_, err := f.failingService(service.PlayTeamEventAdminMemberAdded).RepairAdminTeamMember(
		f.ctx,
		service.AdminTeamMemberRepairInput{
			TargetTeamID: team.ID,
			UserID:       memberID,
			ActorUserID:  actorID,
			Operation:    service.AdminTeamMemberOperationAdd,
			Reason:       "repair should roll back on event failure",
		},
	)
	require.Error(t, err)

	var memberships, events int
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_members
		WHERE user_id = $1`, memberID).Scan(&memberships))
	require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
		SELECT COUNT(*)
		FROM play_team_events
		WHERE subject_user_id = $1
		  AND event_type = $2`, memberID, service.PlayTeamEventAdminMemberAdded).Scan(&events))
	require.Zero(t, memberships)
	require.Zero(t, events)
}

func TestAdminTeamRepairRejectsEntityCreationTimeInconsistency(t *testing.T) {
	t.Run("target team created after effective time", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		actorID := f.user("created-target-actor")
		captainID := f.user("created-target-captain")
		memberID := f.user("created-target-member")
		team := f.createTeam(captainID, "created-target")
		effectiveAt := currentShanghaiAdminRepairTime(t)
		_, err := integrationDB.ExecContext(f.ctx, `
			UPDATE play_teams
			SET created_at = $2
			WHERE id = $1`, team.ID, effectiveAt.Add(time.Minute))
		require.NoError(t, err)

		_, err = f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
			TargetTeamID: team.ID,
			UserID:       memberID,
			ActorUserID:  actorID,
			Operation:    service.AdminTeamMemberOperationAdd,
			EffectiveAt:  &effectiveAt,
			Reason:       "reject target creation time inversion",
		})
		require.ErrorIs(t, err, service.ErrPlayAdminTeamEffectiveBeforeTargetCreated)

		var active int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*)
			FROM play_team_members
			WHERE user_id = $1 AND left_at IS NULL`, memberID).Scan(&active))
		require.Zero(t, active)
	})

	t.Run("user created after effective time", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		actorID := f.user("created-user-actor")
		captainID := f.user("created-user-captain")
		memberID := f.user("created-user-member")
		team := f.createTeam(captainID, "created-user")
		effectiveAt := currentShanghaiAdminRepairTime(t)
		_, err := integrationDB.ExecContext(f.ctx, `
			UPDATE users
			SET created_at = $2
			WHERE id = $1`, memberID, effectiveAt.Add(time.Minute))
		require.NoError(t, err)

		_, err = f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
			TargetTeamID: team.ID,
			UserID:       memberID,
			ActorUserID:  actorID,
			Operation:    service.AdminTeamMemberOperationAdd,
			EffectiveAt:  &effectiveAt,
			Reason:       "reject user creation time inversion",
		})
		require.ErrorIs(t, err, service.ErrPlayAdminTeamEffectiveBeforeUserCreated)

		var active int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*)
			FROM play_team_members
			WHERE user_id = $1 AND left_at IS NULL`, memberID).Scan(&active))
		require.Zero(t, active)
	})
}

func TestAdminTeamRepairRejectsAmbiguousSourceHistory(t *testing.T) {
	t.Run("captain changed after effective time", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		actorID := f.user("history-captain-actor")
		sourceCaptainID := f.user("history-source-captain")
		targetCaptainID := f.user("history-target-captain")
		memberID := f.user("history-moving-member")
		source := f.createTeam(sourceCaptainID, "history-captain-source")
		target := f.createTeam(targetCaptainID, "history-captain-target")
		_, err := f.service.JoinTeam(f.ctx, memberID, source.InviteCode)
		require.NoError(t, err)
		effectiveAt := currentShanghaiAdminRepairTime(t)
		time.Sleep(2 * time.Millisecond)
		require.NoError(t, f.repo.InsertTeamEvent(f.ctx, service.PlayTeamEvent{
			TeamID:        source.ID,
			ActorUserID:   sourceCaptainID,
			SubjectUserID: memberID,
			Type:          service.PlayTeamEventCaptainTransferred,
			Detail:        map[string]any{"previous_captain_user_id": sourceCaptainID},
		}))

		sourceID := source.ID
		_, err = f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
			TargetTeamID:         target.ID,
			UserID:               memberID,
			ActorUserID:          actorID,
			Operation:            service.AdminTeamMemberOperationMove,
			EffectiveAt:          &effectiveAt,
			Reason:               "reject ambiguous captain history",
			ExpectedSourceTeamID: &sourceID,
		})
		require.ErrorIs(t, err, service.ErrPlayAdminTeamSourceHistoryConflict)

		var sourceActive, targetActive int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`,
			source.ID, memberID,
		).Scan(&sourceActive))
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`,
			target.ID, memberID,
		).Scan(&targetActive))
		require.Equal(t, 1, sourceActive)
		require.Zero(t, targetActive)
	})

	t.Run("another membership extends after effective time", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		actorID := f.user("history-member-actor")
		movingCaptainID := f.user("history-moving-captain")
		targetCaptainID := f.user("history-member-target-captain")
		historicalMemberID := f.user("history-former-member")
		source := f.createTeam(movingCaptainID, "history-member-source")
		target := f.createTeam(targetCaptainID, "history-member-target")
		effectiveAt := currentShanghaiAdminRepairTime(t)
		time.Sleep(2 * time.Millisecond)
		_, err := integrationDB.ExecContext(f.ctx, `
			INSERT INTO play_team_members (team_id, user_id, joined_at, left_at)
			VALUES ($1, $2, $3, NOW())`,
			source.ID,
			historicalMemberID,
			effectiveAt.Add(-time.Hour),
		)
		require.NoError(t, err)

		sourceID := source.ID
		_, err = f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
			TargetTeamID:         target.ID,
			UserID:               movingCaptainID,
			ActorUserID:          actorID,
			Operation:            service.AdminTeamMemberOperationMove,
			EffectiveAt:          &effectiveAt,
			Reason:               "reject ambiguous member history",
			ExpectedSourceTeamID: &sourceID,
		})
		require.ErrorIs(t, err, service.ErrPlayAdminTeamSourceHistoryConflict)

		var sourceActive, archived int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`,
			source.ID, movingCaptainID,
		).Scan(&sourceActive))
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_teams
			WHERE id = $1 AND archived_at IS NOT NULL`,
			source.ID,
		).Scan(&archived))
		require.Equal(t, 1, sourceActive)
		require.Zero(t, archived)
	})

	t.Run("archived source still has membership history after effective time", func(t *testing.T) {
		f := newTeamLifecycleFixture(t)
		actorID := f.user("history-archived-actor")
		movingCaptainID := f.user("history-archived-moving-captain")
		targetCaptainID := f.user("history-archived-target-captain")
		historicalMemberID := f.user("history-archived-former-member")
		source := f.createTeam(movingCaptainID, "history-archived-source")
		target := f.createTeam(targetCaptainID, "history-archived-target")
		effectiveAt := currentShanghaiAdminRepairTime(t)
		time.Sleep(2 * time.Millisecond)
		_, err := integrationDB.ExecContext(f.ctx, `
			INSERT INTO play_team_members (team_id, user_id, joined_at, left_at)
			VALUES ($1, $2, $3, NOW())`,
			source.ID,
			historicalMemberID,
			effectiveAt.Add(-time.Hour),
		)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(f.ctx, `
			UPDATE play_teams
			SET archived_at = NOW()
			WHERE id = $1`,
			source.ID,
		)
		require.NoError(t, err)

		sourceID := source.ID
		_, err = f.service.RepairAdminTeamMember(f.ctx, service.AdminTeamMemberRepairInput{
			TargetTeamID:         target.ID,
			UserID:               movingCaptainID,
			ActorUserID:          actorID,
			Operation:            service.AdminTeamMemberOperationMove,
			EffectiveAt:          &effectiveAt,
			Reason:               "reject archived source member history",
			ExpectedSourceTeamID: &sourceID,
		})
		require.ErrorIs(t, err, service.ErrPlayAdminTeamSourceHistoryConflict)

		var sourceActive, targetActive int
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`,
			source.ID, movingCaptainID,
		).Scan(&sourceActive))
		require.NoError(t, integrationDB.QueryRowContext(f.ctx, `
			SELECT COUNT(*) FROM play_team_members
			WHERE team_id = $1 AND user_id = $2 AND left_at IS NULL`,
			target.ID, movingCaptainID,
		).Scan(&targetActive))
		require.Equal(t, 1, sourceActive)
		require.Zero(t, targetActive)
	})
}

func currentShanghaiAdminRepairTime(t *testing.T) time.Time {
	t.Helper()
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Now().In(shanghai)
	return now
}
