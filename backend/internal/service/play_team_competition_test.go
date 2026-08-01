package service

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type teamCompetitionReadRepo struct {
	PlayRepository
	directory   []PlayTeamDirectoryBase
	leaderboard []PlayTeamPublicLeaderboardBase
	seasons     []PlayTeamSeason
	rankings    []PlayTeamSeasonRanking
}

type teamCompetitionSettingRepo struct {
	SettingRepository
	values map[string]string
}

func (r *teamCompetitionSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

type teamCompetitionApplicationEvent struct {
	ApplicationID int64
	TeamID        int64
	ActorUserID   *int64
	EventType     string
	FromStatus    string
	ToStatus      string
}

type teamCompetitionLifecycleTestRepo struct {
	PlayRepository

	application *PlayTeamJoinApplication
	updates     []PlayTeamJoinApplication
	events      []teamCompetitionApplicationEvent
	txCalls     int
}

func (r *teamCompetitionLifecycleTestRepo) WithTeamCompetitionAdmissionTx(ctx context.Context, fn func(context.Context) error) error {
	r.txCalls++
	return fn(ctx)
}

func (*teamCompetitionLifecycleTestRepo) CreateTeamWithInvite(context.Context, string, int64, PlayTeamInvite) (*PlayTeamDB, error) {
	return nil, nil
}

func (*teamCompetitionLifecycleTestRepo) LockTeamByInviteCode(context.Context, string) (*PlayTeamCompetitionTeam, error) {
	return nil, nil
}

func (*teamCompetitionLifecycleTestRepo) LockTeamCompetition(context.Context, int64) (*PlayTeamCompetitionTeam, error) {
	return nil, nil
}

func (*teamCompetitionLifecycleTestRepo) UpdateTeamInvite(context.Context, int64, PlayTeamInvite) error {
	return nil
}

func (*teamCompetitionLifecycleTestRepo) UpdateTeamRecruiting(context.Context, int64, bool) error {
	return nil
}

func (*teamCompetitionLifecycleTestRepo) GetLatestTeamLeftAt(context.Context, int64) (*time.Time, error) {
	return nil, nil
}

func (*teamCompetitionLifecycleTestRepo) JoinTeamWithEligibility(context.Context, int64, int64, time.Time, time.Time) error {
	return nil
}

func (*teamCompetitionLifecycleTestRepo) CreateTeamJoinApplication(context.Context, PlayTeamJoinApplication) (*PlayTeamJoinApplication, error) {
	return nil, nil
}

func (r *teamCompetitionLifecycleTestRepo) LockTeamJoinApplication(_ context.Context, applicationID int64) (*PlayTeamJoinApplication, error) {
	if r.application == nil || r.application.ID != applicationID {
		return nil, nil
	}
	return r.application, nil
}

func (r *teamCompetitionLifecycleTestRepo) UpdateTeamJoinApplication(_ context.Context, application PlayTeamJoinApplication) error {
	r.updates = append(r.updates, application)
	return nil
}

func (*teamCompetitionLifecycleTestRepo) ExpirePendingTeamJoinApplications(context.Context, int64, time.Time) ([]PlayTeamJoinApplication, error) {
	return nil, nil
}

func (*teamCompetitionLifecycleTestRepo) ListUserTeamJoinApplications(context.Context, int64, int) ([]PlayTeamJoinApplication, error) {
	return nil, nil
}

func (*teamCompetitionLifecycleTestRepo) ListCaptainTeamJoinApplications(context.Context, int64, int) ([]PlayTeamJoinApplication, error) {
	return nil, nil
}

func (r *teamCompetitionLifecycleTestRepo) RecordTeamJoinApplicationEvent(
	_ context.Context,
	applicationID, teamID int64,
	actorUserID *int64,
	eventType, fromStatus, toStatus string,
	_ map[string]any,
) error {
	r.events = append(r.events, teamCompetitionApplicationEvent{
		ApplicationID: applicationID,
		TeamID:        teamID,
		ActorUserID:   actorUserID,
		EventType:     eventType,
		FromStatus:    fromStatus,
		ToStatus:      toStatus,
	})
	return nil
}

func newTeamCompetitionLifecycleService(repo *teamCompetitionLifecycleTestRepo, now time.Time) *PlayService {
	settings := NewSettingService(&teamCompetitionSettingRepo{values: map[string]string{
		SettingKeyPlayAgentTeamEnabled: "true",
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)
	svc.now = func() time.Time { return now }
	return svc
}

func (r *teamCompetitionReadRepo) ListPublicTeamDirectory(context.Context, time.Time, time.Time, int) ([]PlayTeamDirectoryBase, error) {
	return append([]PlayTeamDirectoryBase(nil), r.directory...), nil
}

func (r *teamCompetitionReadRepo) ListPublicTeamLeaderboard(context.Context, time.Time, time.Time, int) ([]PlayTeamPublicLeaderboardBase, int, error) {
	return append([]PlayTeamPublicLeaderboardBase(nil), r.leaderboard...), len(r.leaderboard), nil
}

func (r *teamCompetitionReadRepo) ListPublicTeamSeasons(context.Context, int) ([]PlayTeamSeason, error) {
	return append([]PlayTeamSeason(nil), r.seasons...), nil
}

func (r *teamCompetitionReadRepo) GetPublicTeamSeason(context.Context, time.Time, int) (*PlayTeamSeason, []PlayTeamSeasonRanking, int, error) {
	if len(r.seasons) == 0 {
		return nil, nil, 0, nil
	}
	season := r.seasons[0]
	return &season, append([]PlayTeamSeasonRanking(nil), r.rankings...), len(r.rankings), nil
}

func TestPublicTeamCompetitionUsesShanghaiMonthAndDoesNotLeakPersonalData(t *testing.T) {
	repo := &teamCompetitionReadRepo{
		leaderboard: []PlayTeamPublicLeaderboardBase{{
			Rank:        1,
			TeamID:      11,
			TeamName:    "星火战队",
			MemberCount: 8,
			Spend:       decimal.RequireFromString("120.50000000"),
			GapToPrev:   decimal.Zero,
		}},
	}
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)
	svc.now = func() time.Time { return time.Date(2026, time.August, 1, 0, 30, 0, 0, time.UTC) }

	board, err := svc.PublicTeamLeaderboard(context.Background(), 500)

	require.NoError(t, err)
	require.Equal(t, "2026-08", board.Month)
	require.Equal(t, 1, board.TotalTeams)
	require.Len(t, board.Rows, 1)
	require.Equal(t, int64(11), board.Rows[0].TeamID)
	require.Equal(t, "120.50000000", board.Rows[0].Spend.StringFixed(8))
}

func TestTeamRewardEligibleAtMovesAfterShanghaiTwentyFifthToNextMonth(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	onCutoff := time.Date(2026, time.August, 25, 23, 59, 0, 0, location)
	afterCutoff := time.Date(2026, time.August, 26, 0, 0, 0, 0, location)

	require.Equal(t, onCutoff, teamRewardEligibleAt(onCutoff))
	require.Equal(t, time.Date(2026, time.September, 1, 0, 0, 0, 0, location), teamRewardEligibleAt(afterCutoff))
}

func TestGenerateTeamInviteCodeIsHighEntropyAndURLSafe(t *testing.T) {
	first, err := generateTeamInviteCode()
	require.NoError(t, err)
	second, err := generateTeamInviteCode()
	require.NoError(t, err)

	require.Len(t, first, 43)
	require.NotEqual(t, first, second)
	require.Regexp(t, `^[A-Za-z0-9_-]+$`, first)
}

func TestWithdrawTeamJoinApplicationTransitionsOnlyTheApplicantPendingRequest(t *testing.T) {
	now := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	repo := &teamCompetitionLifecycleTestRepo{application: &PlayTeamJoinApplication{
		ID:          51,
		TeamID:      11,
		ApplicantID: 7,
		Status:      PlayTeamJoinApplicationPending,
		ExpiresAt:   now.Add(time.Hour),
	}}
	svc := newTeamCompetitionLifecycleService(repo, now)

	application, err := svc.WithdrawTeamJoinApplication(context.Background(), 7, 51)

	require.NoError(t, err)
	require.NotNil(t, application)
	require.Equal(t, PlayTeamJoinApplicationWithdrawn, application.Status)
	require.Equal(t, now, *application.HandledAt)
	require.NotNil(t, application.HandledByID)
	require.Equal(t, int64(7), *application.HandledByID)
	require.Len(t, repo.updates, 1)
	require.Len(t, repo.events, 1)
	require.Equal(t, "withdrawn", repo.events[0].EventType)
	require.Equal(t, PlayTeamJoinApplicationPending, repo.events[0].FromStatus)
	require.Equal(t, PlayTeamJoinApplicationWithdrawn, repo.events[0].ToStatus)
	require.NotNil(t, repo.events[0].ActorUserID)
	require.Equal(t, int64(7), *repo.events[0].ActorUserID)
}

func TestWithdrawTeamJoinApplicationDoesNotRevealAnotherApplicantsRequest(t *testing.T) {
	now := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	repo := &teamCompetitionLifecycleTestRepo{application: &PlayTeamJoinApplication{
		ID:          51,
		TeamID:      11,
		ApplicantID: 7,
		Status:      PlayTeamJoinApplicationPending,
		ExpiresAt:   now.Add(time.Hour),
	}}
	svc := newTeamCompetitionLifecycleService(repo, now)

	application, err := svc.WithdrawTeamJoinApplication(context.Background(), 8, 51)

	require.ErrorIs(t, err, ErrPlayTeamApplicationNotFound)
	require.Nil(t, application)
	require.Empty(t, repo.updates)
	require.Empty(t, repo.events)
}

func TestWithdrawTeamJoinApplicationExpiresStalePendingRequestWithAuditEvent(t *testing.T) {
	now := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	repo := &teamCompetitionLifecycleTestRepo{application: &PlayTeamJoinApplication{
		ID:          51,
		TeamID:      11,
		ApplicantID: 7,
		Status:      PlayTeamJoinApplicationPending,
		ExpiresAt:   now,
	}}
	svc := newTeamCompetitionLifecycleService(repo, now)

	application, err := svc.WithdrawTeamJoinApplication(context.Background(), 7, 51)

	require.ErrorIs(t, err, ErrPlayTeamApplicationExpired)
	require.NotNil(t, application)
	require.Equal(t, PlayTeamJoinApplicationExpired, application.Status)
	require.Len(t, repo.updates, 1)
	require.Len(t, repo.events, 1)
	require.Equal(t, "expired", repo.events[0].EventType)
	require.Equal(t, PlayTeamJoinApplicationPending, repo.events[0].FromStatus)
	require.Equal(t, PlayTeamJoinApplicationExpired, repo.events[0].ToStatus)
}
