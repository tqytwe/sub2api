package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type adminTeamRepairRepo struct {
	PlayRepository

	user             *PlayAdminTeamMemberCandidate
	membership       *PlayTeamMembershipDB
	teams            map[int64]*PlayTeamDB
	memberCounts     map[int64]int
	snapshot         bool
	overlap          bool
	captainChanged   bool
	otherMembership  bool
	callOrder        []string
	lockedTeamIDs    []int64
	joinedTeamID     int64
	joinedAt         time.Time
	closedMembership int64
	closedAt         time.Time
	archivedTeamID   int64
	events           []PlayTeamEvent
}

func (r *adminTeamRepairRepo) LockAdminTeamCandidateUser(context.Context, int64) (*PlayAdminTeamMemberCandidate, error) {
	r.callOrder = append(r.callOrder, "lock_user")
	return r.user, nil
}

func (r *adminTeamRepairRepo) GetActiveTeamMembership(context.Context, int64) (*PlayTeamMembershipDB, error) {
	r.callOrder = append(r.callOrder, "read_membership")
	return r.membership, nil
}

func (r *adminTeamRepairRepo) LockActiveTeamMembership(context.Context, int64) (*PlayTeamMembershipDB, error) {
	r.callOrder = append(r.callOrder, "lock_membership")
	return r.membership, nil
}

func (r *adminTeamRepairRepo) LockTeamForAdmin(_ context.Context, teamID int64) (*PlayTeamDB, error) {
	r.callOrder = append(r.callOrder, fmt.Sprintf("lock_team:%d", teamID))
	r.lockedTeamIDs = append(r.lockedTeamIDs, teamID)
	return r.teams[teamID], nil
}

func (r *adminTeamRepairRepo) CountActiveTeamMembers(_ context.Context, teamID int64) (int, error) {
	return r.memberCounts[teamID], nil
}

func (r *adminTeamRepairRepo) HasTeamRewardSnapshotAt(context.Context, []int64, time.Time) (bool, error) {
	return r.snapshot, nil
}

func (r *adminTeamRepairRepo) HasTeamMembershipOverlap(context.Context, int64, time.Time, int64) (bool, error) {
	return r.overlap, nil
}

func (r *adminTeamRepairRepo) HasTeamCaptainChangeAfter(context.Context, int64, time.Time) (bool, error) {
	return r.captainChanged, nil
}

func (r *adminTeamRepairRepo) HasOtherTeamMembershipAfter(context.Context, int64, int64, time.Time) (bool, error) {
	return r.otherMembership, nil
}

func (r *adminTeamRepairRepo) JoinTeamAt(_ context.Context, teamID, _ int64, joinedAt time.Time) error {
	r.joinedTeamID = teamID
	r.joinedAt = joinedAt
	return nil
}

func (r *adminTeamRepairRepo) CloseTeamMembershipAt(_ context.Context, membershipID int64, leftAt time.Time) error {
	r.closedMembership = membershipID
	r.closedAt = leftAt
	return nil
}

func (r *adminTeamRepairRepo) ArchiveTeamAt(_ context.Context, teamID int64, _ time.Time) error {
	r.archivedTeamID = teamID
	return nil
}

func (r *adminTeamRepairRepo) InsertTeamEvent(_ context.Context, event PlayTeamEvent) error {
	r.events = append(r.events, event)
	return nil
}

func newAdminTeamRepairService(t *testing.T, repo *adminTeamRepairRepo, now time.Time) (*PlayService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	svc := NewPlayService(repo, nil, nil, nil, nil, client)
	svc.now = func() time.Time { return now }
	return svc, mock
}

func activeRepairCandidate(userID int64) *PlayAdminTeamMemberCandidate {
	return &PlayAdminTeamMemberCandidate{
		UserID:      userID,
		Email:       "member@example.com",
		Username:    "member",
		DisplayName: "member",
		Status:      StatusActive,
	}
}

func TestAdminRepairTeamMemberAddsAtServerNowAndWritesTypedEvent(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.July, 20, 10, 30, 0, 0, shanghai)
	repo := &adminTeamRepairRepo{
		user: activeRepairCandidate(42),
		teams: map[int64]*PlayTeamDB{
			9: {ID: 9, Name: "Target", CaptainUserID: 7},
		},
		memberCounts: map[int64]int{9: 1},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID: 9,
		UserID:       42,
		ActorUserID:  99,
		Operation:    AdminTeamMemberOperationAdd,
		Reason:       "repair missing membership",
	})

	require.NoError(t, err)
	require.Equal(t, AdminTeamMemberRepairStatusAdded, result.Status)
	require.Equal(t, int64(9), repo.joinedTeamID)
	require.Equal(t, now, repo.joinedAt)
	require.Len(t, repo.events, 1)
	require.Equal(t, PlayTeamEventAdminMemberAdded, repo.events[0].Type)
	require.Equal(t, int64(99), repo.events[0].ActorUserID)
	require.Equal(t, int64(42), repo.events[0].SubjectUserID)
	require.NotContains(t, repo.events[0].Detail, "invite_code")
	require.NotContains(t, repo.events[0].Detail, "token")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberRedactsSensitiveReasonFromBusinessEvents(t *testing.T) {
	now := time.Date(2026, time.July, 20, 10, 30, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	repo := &adminTeamRepairRepo{
		user: activeRepairCandidate(42),
		teams: map[int64]*PlayTeamDB{
			9: {
				ID:            9,
				Name:          "Target",
				CaptainUserID: 7,
				InviteCode:    "TEAM-SECRET-123",
			},
		},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()
	reason := "ticket 12345 A1B2C3D4 token is abcdefghij invite_code=TEAM-SECRET-123 token=sk-supersecret1234567890 bearer ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID: 9,
		UserID:       42,
		ActorUserID:  99,
		Operation:    AdminTeamMemberOperationAdd,
		Reason:       reason,
	})

	require.NoError(t, err)
	require.Len(t, repo.events, 1)
	storedReason, ok := repo.events[0].Detail["reason"].(string)
	require.True(t, ok)
	require.Contains(t, storedReason, "ticket 12345")
	require.Contains(t, storedReason, "***")
	require.NotContains(t, storedReason, "A1B2C3D4")
	require.NotContains(t, storedReason, "abcdefghij")
	require.NotContains(t, storedReason, "TEAM-SECRET-123")
	require.NotContains(t, storedReason, "sk-supersecret1234567890")
	require.NotContains(t, storedReason, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	require.Equal(t, PlayTeamEventReasonAdminManualMembershipRepair, repo.events[0].Detail["reason_code"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberAlreadyInTargetIsNoOp(t *testing.T) {
	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	repo := &adminTeamRepairRepo{
		user: func() *PlayAdminTeamMemberCandidate {
			candidate := activeRepairCandidate(42)
			candidate.CreatedAt = now
			return candidate
		}(),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: 9, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			9: {ID: 9, Name: "Target", CaptainUserID: 7, CreatedAt: now},
		},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID: 9,
		UserID:       42,
		ActorUserID:  99,
		Operation:    AdminTeamMemberOperationAdd,
		Reason:       "confirm existing membership",
	})

	require.NoError(t, err)
	require.Equal(t, AdminTeamMemberRepairStatusNoOp, result.Status)
	require.Zero(t, repo.joinedTeamID)
	require.Zero(t, repo.closedMembership)
	require.Empty(t, repo.events)
	require.Equal(t, []int64{9}, repo.lockedTeamIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberRejectsEffectiveTimeBeforeEntityCreation(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	effectiveAt := now.Add(-48 * time.Hour)
	createdAfterEffectiveAt := effectiveAt.Add(time.Hour)

	tests := []struct {
		name          string
		userCreatedAt time.Time
		teamCreatedAt time.Time
		wantErr       error
	}{
		{
			name:          "target team",
			teamCreatedAt: createdAfterEffectiveAt,
			wantErr:       ErrPlayAdminTeamEffectiveBeforeTargetCreated,
		},
		{
			name:          "user",
			userCreatedAt: createdAfterEffectiveAt,
			wantErr:       ErrPlayAdminTeamEffectiveBeforeUserCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := activeRepairCandidate(42)
			candidate.CreatedAt = tt.userCreatedAt
			repo := &adminTeamRepairRepo{
				user: candidate,
				teams: map[int64]*PlayTeamDB{
					9: {
						ID:            9,
						Name:          "Target",
						CaptainUserID: 7,
						CreatedAt:     tt.teamCreatedAt,
					},
				},
			}
			svc, mock := newAdminTeamRepairService(t, repo, now)
			mock.ExpectBegin()
			mock.ExpectRollback()

			_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
				TargetTeamID: 9,
				UserID:       42,
				ActorUserID:  99,
				Operation:    AdminTeamMemberOperationAdd,
				EffectiveAt:  &effectiveAt,
				Reason:       "reject impossible creation history",
			})

			require.ErrorIs(t, err, tt.wantErr)
			require.Zero(t, repo.joinedTeamID)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAdminRepairTeamMemberAlreadyInTargetRejectsStaleExpectedSource(t *testing.T) {
	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	staleSourceID := int64(8)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: 9, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			9: {ID: 9, Name: "Target", CaptainUserID: 7},
		},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         9,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		Reason:               "reject stale source after concurrent move",
		ExpectedSourceTeamID: &staleSourceID,
	})

	require.ErrorIs(t, err, ErrPlayAdminTeamSourceConflict)
	require.Zero(t, repo.joinedTeamID)
	require.Zero(t, repo.closedMembership)
	require.Empty(t, repo.events)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberMoveRequiresSourceEvenWhenAlreadyInTarget(t *testing.T) {
	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: 9, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			9: {ID: 9, Name: "Target", CaptainUserID: 7},
		},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)

	_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID: 9,
		UserID:       42,
		ActorUserID:  99,
		Operation:    AdminTeamMemberOperationMove,
		Reason:       "require source snapshot for no-op move",
	})

	require.ErrorIs(t, err, ErrPlayAdminTeamMoveSourceRequired)
	require.Empty(t, repo.callOrder)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberNoOpRejectsArchivedTarget(t *testing.T) {
	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	archivedAt := now.Add(-time.Hour)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: 9, UserID: 42, JoinedAt: now.Add(-2 * time.Hour)},
		teams: map[int64]*PlayTeamDB{
			9: {ID: 9, Name: "Archived target", CaptainUserID: 7, ArchivedAt: &archivedAt},
		},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID: 9,
		UserID:       42,
		ActorUserID:  99,
		Operation:    AdminTeamMemberOperationAdd,
		Reason:       "confirm archived membership state",
	})

	require.ErrorIs(t, err, ErrPlayTeamNotFound)
	require.Equal(t, []int64{9}, repo.lockedTeamIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberMoveUsesOneTimestampAndDeterministicLocks(t *testing.T) {
	effectiveAt := time.Date(2026, time.July, 12, 4, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	sourceID := int64(20)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: sourceID, UserID: 42, JoinedAt: effectiveAt.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			10: {ID: 10, Name: "Target", CaptainUserID: 7},
			20: {ID: 20, Name: "Source", CaptainUserID: 8},
		},
		memberCounts: map[int64]int{20: 2},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         10,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		EffectiveAt:          &effectiveAt,
		Reason:               "move member to correct team",
		ExpectedSourceTeamID: &sourceID,
	})

	require.NoError(t, err)
	require.Equal(t, AdminTeamMemberRepairStatusMoved, result.Status)
	require.Equal(t, []int64{10, 20}, repo.lockedTeamIDs)
	require.Equal(t, int64(5), repo.closedMembership)
	require.Equal(t, effectiveAt, repo.closedAt)
	require.Equal(t, effectiveAt, repo.joinedAt)
	require.Len(t, repo.events, 2)
	require.Equal(t, PlayTeamEventAdminMemberMoved, repo.events[0].Type)
	require.Equal(t, "source", repo.events[0].Detail["side"])
	require.Equal(t, "move member to correct team", repo.events[0].Detail["reason"])
	require.Equal(t, PlayTeamEventAdminMemberMoved, repo.events[1].Type)
	require.Equal(t, "target", repo.events[1].Detail["side"])
	require.Equal(t, "move member to correct team", repo.events[1].Detail["reason"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberLocksTeamsBeforeMembershipToAvoidLifecycleDeadlocks(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	sourceID := int64(20)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: sourceID, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			10: {ID: 10, Name: "Target", CaptainUserID: 7},
			20: {ID: 20, Name: "Source", CaptainUserID: 8},
		},
		memberCounts: map[int64]int{20: 2},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()

	_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         10,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		Reason:               "move member without lifecycle deadlock",
		ExpectedSourceTeamID: &sourceID,
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"lock_user",
		"read_membership",
		"lock_team:10",
		"lock_team:20",
		"lock_membership",
	}, repo.callOrder)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberRejectsStaleSourceAndCaptainWithOtherMembers(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	sourceID := int64(20)
	staleSourceID := int64(19)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: sourceID, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			10: {ID: 10, Name: "Target", CaptainUserID: 7},
			20: {ID: 20, Name: "Source", CaptainUserID: 42},
		},
		memberCounts: map[int64]int{20: 2},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)

	mock.ExpectBegin()
	mock.ExpectRollback()
	_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         10,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		Reason:               "stale source team preview",
		ExpectedSourceTeamID: &staleSourceID,
	})
	require.ErrorIs(t, err, ErrPlayAdminTeamSourceConflict)

	mock.ExpectBegin()
	mock.ExpectRollback()
	repo.captainChanged = true
	_, err = svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         10,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		Reason:               "captain still has other members",
		ExpectedSourceTeamID: &sourceID,
	})
	require.ErrorIs(t, err, ErrPlayAdminTeamCaptainTransferRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberMovesSoleCaptainAndArchivesSource(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	sourceID := int64(20)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: sourceID, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			10: {ID: 10, Name: "Target", CaptainUserID: 7},
			20: {ID: 20, Name: "Source", CaptainUserID: 42},
		},
		memberCounts: map[int64]int{20: 1},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         10,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		Reason:               "move sole captain and archive",
		ExpectedSourceTeamID: &sourceID,
	})

	require.NoError(t, err)
	require.Equal(t, int64(20), repo.archivedTeamID)
	require.Contains(t, result.Warnings, PlayAdminTeamWarningSourceArchived)
	require.Len(t, repo.events, 3)
	require.Equal(t, PlayTeamEventArchived, repo.events[1].Type)
	require.Equal(t, "admin_moved_last_captain", repo.events[1].Detail["reason_code"])
	require.Equal(t, "move sole captain and archive", repo.events[1].Detail["reason"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberRejectsAmbiguousBackdatedSourceHistory(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	effectiveAt := now.Add(-48 * time.Hour)
	sourceID := int64(20)
	archivedAt := now.Add(-time.Hour)

	tests := []struct {
		name            string
		isCaptain       bool
		captainChanged  bool
		otherMembership bool
		sourceArchived  bool
	}{
		{
			name:           "captain changed after effective time",
			captainChanged: true,
		},
		{
			name:            "sole current captain had another membership interval after effective time",
			isCaptain:       true,
			otherMembership: true,
		},
		{
			name:            "archived source captain had another membership interval after effective time",
			isCaptain:       true,
			otherMembership: true,
			sourceArchived:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captainID := int64(8)
			if tt.isCaptain {
				captainID = 42
			}
			sourceTeam := &PlayTeamDB{ID: 20, Name: "Source", CaptainUserID: captainID}
			if tt.sourceArchived {
				sourceTeam.ArchivedAt = &archivedAt
			}
			repo := &adminTeamRepairRepo{
				user:       activeRepairCandidate(42),
				membership: &PlayTeamMembershipDB{ID: 5, TeamID: sourceID, UserID: 42, JoinedAt: effectiveAt.Add(-time.Hour)},
				teams: map[int64]*PlayTeamDB{
					10: {ID: 10, Name: "Target", CaptainUserID: 7},
					20: sourceTeam,
				},
				memberCounts:    map[int64]int{20: 1},
				captainChanged:  tt.captainChanged,
				otherMembership: tt.otherMembership,
			}
			svc, mock := newAdminTeamRepairService(t, repo, now)
			mock.ExpectBegin()
			mock.ExpectRollback()

			_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
				TargetTeamID:         10,
				UserID:               42,
				ActorUserID:          99,
				Operation:            AdminTeamMemberOperationMove,
				EffectiveAt:          &effectiveAt,
				Reason:               "reject ambiguous source team history",
				ExpectedSourceTeamID: &sourceID,
			})

			require.ErrorIs(t, err, ErrPlayAdminTeamSourceHistoryConflict)
			require.Zero(t, repo.closedMembership)
			require.Zero(t, repo.archivedTeamID)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAdminRepairTeamMemberRepairsArchivedSourceMembershipWithWarning(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	archivedAt := now.Add(-24 * time.Hour)
	sourceID := int64(20)
	repo := &adminTeamRepairRepo{
		user:       activeRepairCandidate(42),
		membership: &PlayTeamMembershipDB{ID: 5, TeamID: sourceID, UserID: 42, JoinedAt: now.Add(-time.Hour)},
		teams: map[int64]*PlayTeamDB{
			10: {ID: 10, Name: "Target", CaptainUserID: 7},
			20: {ID: 20, Name: "Archived source", CaptainUserID: 8, ArchivedAt: &archivedAt},
		},
	}
	svc, mock := newAdminTeamRepairService(t, repo, now)
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
		TargetTeamID:         10,
		UserID:               42,
		ActorUserID:          99,
		Operation:            AdminTeamMemberOperationMove,
		Reason:               "repair archived source membership",
		ExpectedSourceTeamID: &sourceID,
	})

	require.NoError(t, err)
	require.Contains(t, result.Warnings, PlayAdminTeamWarningArchivedMembershipRepair)
	require.Zero(t, repo.archivedTeamID)
	require.Equal(t, now, repo.closedAt)
	require.Equal(t, now, repo.joinedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminRepairTeamMemberRejectsInvalidEffectiveTimeSnapshotOverlapAndInactiveUser(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.July, 20, 10, 0, 0, 0, shanghai)
	target := &PlayTeamDB{ID: 10, Name: "Target", CaptainUserID: 7}

	tests := []struct {
		name        string
		effectiveAt time.Time
		configure   func(*adminTeamRepairRepo)
		wantErr     error
	}{
		{
			name:        "previous month",
			effectiveAt: time.Date(2026, time.June, 30, 23, 59, 0, 0, shanghai),
			wantErr:     ErrPlayAdminTeamEffectiveAtOutsideMonth,
		},
		{
			name:        "future",
			effectiveAt: now.Add(time.Minute),
			wantErr:     ErrPlayAdminTeamEffectiveAtFuture,
		},
		{
			name:        "immutable settlement snapshot",
			effectiveAt: now.Add(-time.Hour),
			configure:   func(r *adminTeamRepairRepo) { r.snapshot = true },
			wantErr:     ErrPlayAdminTeamSettlementSnapshotExists,
		},
		{
			name:        "historical membership overlap",
			effectiveAt: now.Add(-time.Hour),
			configure:   func(r *adminTeamRepairRepo) { r.overlap = true },
			wantErr:     ErrPlayAdminTeamMembershipOverlap,
		},
		{
			name:        "inactive user",
			effectiveAt: now,
			configure: func(r *adminTeamRepairRepo) {
				r.user.Status = StatusDisabled
			},
			wantErr: ErrPlayAdminTeamUserInactive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &adminTeamRepairRepo{
				user:  activeRepairCandidate(42),
				teams: map[int64]*PlayTeamDB{10: target},
			}
			if tt.configure != nil {
				tt.configure(repo)
			}
			svc, mock := newAdminTeamRepairService(t, repo, now)
			if tt.name != "previous month" && tt.name != "future" {
				mock.ExpectBegin()
				mock.ExpectRollback()
			}
			_, err := svc.RepairAdminTeamMember(context.Background(), AdminTeamMemberRepairInput{
				TargetTeamID: 10,
				UserID:       42,
				ActorUserID:  99,
				Operation:    AdminTeamMemberOperationAdd,
				EffectiveAt:  &tt.effectiveAt,
				Reason:       "repair historical membership",
			})
			require.ErrorIs(t, err, tt.wantErr)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAdminTeamMemberCandidatePreviewUsesRewardPolicy(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	repo := &adminTeamRepairRepo{}
	repo.PlayRepository = &adminTeamCandidatePreviewRepo{
		candidates:  []PlayAdminTeamMemberCandidate{*activeRepairCandidate(42)},
		targetSpend: decimal.RequireFromString("95"),
		userSpend:   decimal.RequireFromString("10"),
	}
	svc := &PlayService{repo: repo.PlayRepository}
	svc.now = func() time.Time { return now }

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "member@example.com",
		Operation:    AdminTeamMemberOperationAdd,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "95.00000000", result.Items[0].Impact.TargetSpendBefore.StringFixed(8))
	require.Equal(t, "105.00000000", result.Items[0].Impact.TargetSpendAfter.StringFixed(8))
	require.True(t, result.Items[0].Impact.TargetPoolAfter.GreaterThan(result.Items[0].Impact.TargetPoolBefore))
}

func TestAdminTeamMemberCandidatePreviewCapturesServerNowOnce(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	firstNow := time.Date(2026, time.July, 31, 23, 59, 30, 0, shanghai)
	secondNow := time.Date(2026, time.August, 1, 0, 0, 30, 0, shanghai)
	repo := &adminTeamCandidatePreviewRepo{
		candidates:  []PlayAdminTeamMemberCandidate{*activeRepairCandidate(42)},
		targetSpend: decimal.Zero,
		userSpend:   decimal.Zero,
	}
	svc := &PlayService{repo: repo}
	nowCalls := 0
	svc.now = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return firstNow
		}
		return secondNow
	}

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "member@example.com",
		Operation:    AdminTeamMemberOperationAdd,
	})

	require.NoError(t, err)
	require.Equal(t, 1, nowCalls)
	require.Equal(t, firstNow, result.EffectiveAt)
}

func TestAdminTeamMemberCandidatePreviewBlocksDeletedUser(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	deleted := *activeRepairCandidate(42)
	deleted.Status = "deleted"
	repo := &adminTeamCandidatePreviewRepo{
		candidates:  []PlayAdminTeamMemberCandidate{deleted},
		targetSpend: decimal.Zero,
		userSpend:   decimal.Zero,
	}
	svc := &PlayService{repo: repo}
	svc.now = func() time.Time { return now }

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "deleted@example.com",
		Operation:    AdminTeamMemberOperationAdd,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "deleted", result.Items[0].Status)
	require.Contains(t, result.Items[0].Blockers, PlayAdminTeamBlockerUserInactive)
}

func TestAdminTeamMemberCandidatePreviewSurfacesMoveAndIntegrityBlockers(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	joinedAt := now.Add(-2 * time.Hour)
	candidate := *activeRepairCandidate(42)
	candidate.CurrentMembershipID = 5
	candidate.CurrentJoinedAt = &joinedAt
	candidate.CurrentTeam = &PlayAdminTeamReference{ID: 20, Name: "Source"}
	candidate.IsCaptain = true
	repo := &adminTeamCandidatePreviewRepo{
		candidates:   []PlayAdminTeamMemberCandidate{candidate},
		targetSpend:  decimal.RequireFromString("95"),
		userSpend:    decimal.RequireFromString("10"),
		teamSpends:   map[int64]decimal.Decimal{10: decimal.RequireFromString("95"), 20: decimal.RequireFromString("40")},
		memberCounts: map[int64]int{20: 2},
		snapshot:     true,
		overlap:      true,
	}
	svc := &PlayService{repo: repo}
	svc.now = func() time.Time { return now }

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "member@example.com",
		Operation:    AdminTeamMemberOperationAdd,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.ElementsMatch(t, []string{
		PlayAdminTeamBlockerMoveRequired,
		PlayAdminTeamBlockerCaptainTransferRequired,
		PlayAdminTeamBlockerSettlementSnapshot,
		PlayAdminTeamBlockerMembershipOverlap,
	}, item.Blockers)
	require.Equal(t, "40.00000000", item.Impact.SourceSpendBefore.StringFixed(8))
	require.Equal(t, "30.00000000", item.Impact.SourceSpendAfter.StringFixed(8))
}

func TestAdminTeamMemberCandidatePreviewBlocksImpossibleEntityAndSourceHistory(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	effectiveAt := now.Add(-48 * time.Hour)
	createdAfterEffectiveAt := effectiveAt.Add(time.Hour)
	joinedAt := effectiveAt.Add(-time.Hour)
	candidate := *activeRepairCandidate(42)
	candidate.CreatedAt = createdAfterEffectiveAt
	candidate.CurrentMembershipID = 5
	candidate.CurrentJoinedAt = &joinedAt
	candidate.CurrentTeam = &PlayAdminTeamReference{ID: 20, Name: "Source"}
	repo := &adminTeamCandidatePreviewRepo{
		candidates:     []PlayAdminTeamMemberCandidate{candidate},
		targetTeam:     &PlayAdminTeamListItem{ID: 10, Name: "Target", CreatedAt: createdAfterEffectiveAt},
		captainChanged: true,
	}
	svc := &PlayService{repo: repo}
	svc.now = func() time.Time { return now }

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "member@example.com",
		Operation:    AdminTeamMemberOperationMove,
		EffectiveAt:  &effectiveAt,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.ElementsMatch(t, []string{
		PlayAdminTeamBlockerEffectiveBeforeTargetCreated,
		PlayAdminTeamBlockerEffectiveBeforeUserCreated,
		PlayAdminTeamBlockerSourceHistoryConflict,
	}, result.Items[0].Blockers)
}

func TestAdminTeamMemberCandidatePreviewPrefersCaptainBlockerOverArchiveHistoryConflict(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	joinedAt := now.Add(-time.Hour)
	candidate := *activeRepairCandidate(42)
	candidate.CurrentMembershipID = 5
	candidate.CurrentJoinedAt = &joinedAt
	candidate.CurrentTeam = &PlayAdminTeamReference{ID: 20, Name: "Source"}
	candidate.IsCaptain = true
	repo := &adminTeamCandidatePreviewRepo{
		candidates:      []PlayAdminTeamMemberCandidate{candidate},
		memberCounts:    map[int64]int{20: 2},
		otherMembership: true,
	}
	svc := &PlayService{repo: repo}
	svc.now = func() time.Time { return now }

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "member@example.com",
		Operation:    AdminTeamMemberOperationMove,
	})

	require.NoError(t, err)
	require.Contains(t, result.Items[0].Blockers, PlayAdminTeamBlockerCaptainTransferRequired)
	require.NotContains(t, result.Items[0].Blockers, PlayAdminTeamBlockerSourceHistoryConflict)
}

func TestAdminTeamMemberCandidatePreviewBlocksArchivedSourceCaptainHistoryConflict(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	joinedAt := now.Add(-2 * time.Hour)
	archivedAt := now.Add(-time.Hour)
	candidate := *activeRepairCandidate(42)
	candidate.CurrentMembershipID = 5
	candidate.CurrentJoinedAt = &joinedAt
	candidate.CurrentTeam = &PlayAdminTeamReference{
		ID:         20,
		Name:       "Archived source",
		ArchivedAt: &archivedAt,
	}
	candidate.IsCaptain = true
	repo := &adminTeamCandidatePreviewRepo{
		candidates:      []PlayAdminTeamMemberCandidate{candidate},
		memberCounts:    map[int64]int{20: 1},
		otherMembership: true,
	}
	svc := &PlayService{repo: repo}
	svc.now = func() time.Time { return now }

	result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
		TargetTeamID: 10,
		Query:        "member@example.com",
		Operation:    AdminTeamMemberOperationMove,
		EffectiveAt:  &joinedAt,
	})

	require.NoError(t, err)
	require.Contains(t, result.Items[0].Blockers, PlayAdminTeamBlockerSourceHistoryConflict)
}

func TestAdminTeamMemberCandidatePreviewSurfacesNoOpArchiveAndSourceWarnings(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)

	t.Run("already in target", func(t *testing.T) {
		joinedAt := now.Add(-time.Hour)
		candidate := *activeRepairCandidate(42)
		candidate.CurrentMembershipID = 5
		candidate.CurrentJoinedAt = &joinedAt
		candidate.CurrentTeam = &PlayAdminTeamReference{ID: 10, Name: "Target"}
		repo := &adminTeamCandidatePreviewRepo{
			candidates:  []PlayAdminTeamMemberCandidate{candidate},
			targetSpend: decimal.RequireFromString("95"),
			userSpend:   decimal.RequireFromString("10"),
			snapshot:    true,
			overlap:     true,
		}
		svc := &PlayService{repo: repo}
		svc.now = func() time.Time { return now }

		result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
			TargetTeamID: 10,
			Query:        "member@example.com",
			Operation:    AdminTeamMemberOperationAdd,
		})

		require.NoError(t, err)
		item := result.Items[0]
		require.Contains(t, item.Warnings, PlayAdminTeamWarningAlreadyInTarget)
		require.NotContains(t, item.Blockers, PlayAdminTeamBlockerSettlementSnapshot)
		require.NotContains(t, item.Blockers, PlayAdminTeamBlockerMembershipOverlap)
		require.Equal(t, item.Impact.TargetSpendBefore, item.Impact.TargetSpendAfter)
		require.Equal(t, item.Impact.TargetPoolBefore, item.Impact.TargetPoolAfter)
	})

	t.Run("move without source", func(t *testing.T) {
		repo := &adminTeamCandidatePreviewRepo{
			candidates: []PlayAdminTeamMemberCandidate{*activeRepairCandidate(42)},
		}
		svc := &PlayService{repo: repo}
		svc.now = func() time.Time { return now }

		result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
			TargetTeamID: 10,
			Query:        "member@example.com",
			Operation:    AdminTeamMemberOperationMove,
		})

		require.NoError(t, err)
		require.Contains(t, result.Items[0].Blockers, PlayAdminTeamBlockerNoSource)
	})

	t.Run("sole captain archives active source", func(t *testing.T) {
		joinedAt := now.Add(-2 * time.Hour)
		candidate := *activeRepairCandidate(42)
		candidate.CurrentMembershipID = 5
		candidate.CurrentJoinedAt = &joinedAt
		candidate.CurrentTeam = &PlayAdminTeamReference{ID: 20, Name: "Source"}
		candidate.IsCaptain = true
		repo := &adminTeamCandidatePreviewRepo{
			candidates:   []PlayAdminTeamMemberCandidate{candidate},
			memberCounts: map[int64]int{20: 1},
		}
		svc := &PlayService{repo: repo}
		svc.now = func() time.Time { return now }

		result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
			TargetTeamID: 10,
			Query:        "member@example.com",
			Operation:    AdminTeamMemberOperationMove,
		})

		require.NoError(t, err)
		require.Contains(t, result.Items[0].Warnings, PlayAdminTeamWarningSourceWillArchive)
	})

	t.Run("archived source and effective time before join", func(t *testing.T) {
		joinedAt := now.Add(-time.Hour)
		archivedAt := now.Add(-24 * time.Hour)
		effectiveAt := now.Add(-2 * time.Hour)
		candidate := *activeRepairCandidate(42)
		candidate.CurrentMembershipID = 5
		candidate.CurrentJoinedAt = &joinedAt
		candidate.CurrentTeam = &PlayAdminTeamReference{
			ID:         20,
			Name:       "Archived source",
			ArchivedAt: &archivedAt,
		}
		repo := &adminTeamCandidatePreviewRepo{
			candidates: []PlayAdminTeamMemberCandidate{candidate},
		}
		svc := &PlayService{repo: repo}
		svc.now = func() time.Time { return now }

		result, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
			TargetTeamID: 10,
			Query:        "member@example.com",
			Operation:    AdminTeamMemberOperationMove,
			EffectiveAt:  &effectiveAt,
		})

		require.NoError(t, err)
		require.Contains(t, result.Items[0].Warnings, PlayAdminTeamWarningArchivedMembershipRepair)
		require.Contains(t, result.Items[0].Blockers, PlayAdminTeamBlockerEffectiveBeforeJoined)
	})
}

func TestAdminTeamMemberCandidatePreviewRejectsMissingOrArchivedTarget(t *testing.T) {
	now := time.Date(2026, time.July, 20, 4, 0, 0, 0, time.UTC)
	archivedAt := now.Add(-time.Hour)
	tests := []struct {
		name       string
		targetTeam *PlayAdminTeamListItem
	}{
		{name: "missing target"},
		{
			name: "archived target",
			targetTeam: &PlayAdminTeamListItem{
				ID:         10,
				Name:       "Archived",
				ArchivedAt: &archivedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &adminTeamCandidatePreviewRepo{
				candidates:  []PlayAdminTeamMemberCandidate{*activeRepairCandidate(42)},
				targetSpend: decimal.Zero,
				userSpend:   decimal.Zero,
				targetTeam:  tt.targetTeam,
			}
			if tt.targetTeam == nil {
				repo.PlayRepository = missingAdminTeamMetaRepo{}
			}
			svc := &PlayService{repo: repo}
			svc.now = func() time.Time { return now }

			_, err := svc.ListAdminTeamMemberCandidates(context.Background(), AdminTeamMemberCandidateQuery{
				TargetTeamID: 10,
				Query:        "member@example.com",
				Operation:    AdminTeamMemberOperationAdd,
			})
			require.ErrorIs(t, err, ErrPlayTeamNotFound)
		})
	}
}

type missingAdminTeamMetaRepo struct {
	PlayRepository
}

func (missingAdminTeamMetaRepo) GetAdminTeamMeta(context.Context, int64) (*PlayAdminTeamListItem, error) {
	return nil, nil
}

type adminTeamCandidatePreviewRepo struct {
	PlayRepository
	candidates      []PlayAdminTeamMemberCandidate
	targetSpend     decimal.Decimal
	userSpend       decimal.Decimal
	targetTeam      *PlayAdminTeamListItem
	teamSpends      map[int64]decimal.Decimal
	memberCounts    map[int64]int
	snapshot        bool
	overlap         bool
	captainChanged  bool
	otherMembership bool
}

func (r *adminTeamCandidatePreviewRepo) ListAdminTeamMemberCandidates(context.Context, int64, string, int) ([]PlayAdminTeamMemberCandidate, error) {
	return append([]PlayAdminTeamMemberCandidate(nil), r.candidates...), nil
}

func (r *adminTeamCandidatePreviewRepo) GetAdminTeamMeta(context.Context, int64) (*PlayAdminTeamListItem, error) {
	if r.PlayRepository != nil {
		return r.PlayRepository.GetAdminTeamMeta(context.Background(), 0)
	}
	if r.targetTeam != nil {
		return r.targetTeam, nil
	}
	return &PlayAdminTeamListItem{ID: 10, Name: "Target"}, nil
}

func (r *adminTeamCandidatePreviewRepo) GetAdminTeamSpend(_ context.Context, teamID int64, _ time.Time, _ time.Time) (decimal.Decimal, error) {
	if r.teamSpends != nil {
		if spend, ok := r.teamSpends[teamID]; ok {
			return spend, nil
		}
	}
	return r.targetSpend, nil
}

func (r *adminTeamCandidatePreviewRepo) GetUserActualCost(context.Context, int64, time.Time, time.Time) (decimal.Decimal, error) {
	return r.userSpend, nil
}

func (r *adminTeamCandidatePreviewRepo) HasTeamRewardSnapshotAt(context.Context, []int64, time.Time) (bool, error) {
	return r.snapshot, nil
}

func (r *adminTeamCandidatePreviewRepo) HasTeamMembershipOverlap(context.Context, int64, time.Time, int64) (bool, error) {
	return r.overlap, nil
}

func (r *adminTeamCandidatePreviewRepo) HasTeamCaptainChangeAfter(context.Context, int64, time.Time) (bool, error) {
	return r.captainChanged, nil
}

func (r *adminTeamCandidatePreviewRepo) HasOtherTeamMembershipAfter(context.Context, int64, int64, time.Time) (bool, error) {
	return r.otherMembership, nil
}

func (r *adminTeamCandidatePreviewRepo) CountActiveTeamMembers(_ context.Context, teamID int64) (int, error) {
	return r.memberCounts[teamID], nil
}
