package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type moderationUserContractRepo struct {
	ContentModerationRepository
	state            ModerationUserState
	manageCalls      int
	autoBan          ModerationAutoBanResult
	manageErr        error
	listCalls        int
	listState        string
	listUserID       int64
	listSince        time.Time
	listExcludeCyber bool
}

func (r *moderationUserContractRepo) GetModerationUser(context.Context, int64, time.Time, bool) (*ModerationUserState, error) {
	state := r.state
	return &state, nil
}

func (r *moderationUserContractRepo) ListModerationUsers(_ context.Context, _, _ int, filter string, userID int64, since time.Time, excludeCyber bool) ([]ModerationUserState, int64, error) {
	r.listCalls++
	r.listState = filter
	r.listUserID = userID
	r.listSince = since
	r.listExcludeCyber = excludeCyber
	return []ModerationUserState{r.state}, 1, nil
}

func (r *moderationUserContractRepo) ManageModerationUser(_ context.Context, _ int64, input ModerationUserAction) error {
	if r.manageErr != nil {
		return r.manageErr
	}
	if input.Action != "unban" || input.Version != r.state.Version {
		return errors.New("unexpected moderation action contract")
	}
	r.manageCalls++
	r.state.Status = StatusActive
	r.state.OwnedDisable = false
	r.state.Version++
	r.state.Reason = input.Reason
	return nil
}

type moderationUserFirstExemptionRepo struct {
	*moderationUserContractRepo
	manageInput ModerationUserAction
}

func (r *moderationUserFirstExemptionRepo) ManageModerationUser(_ context.Context, _ int64, input ModerationUserAction) error {
	if input.Action != "exemption" || input.Version != r.state.Version {
		return errors.New("unexpected first exemption contract")
	}
	r.manageInput = input
	r.manageCalls++
	if input.Exempt != nil {
		r.state.AutoBanExempt = *input.Exempt
	}
	r.state.Version++
	r.state.Reason = input.Reason
	return nil
}

func (r *moderationUserContractRepo) ApplyModerationAutoBan(context.Context, int64) (ModerationAutoBanResult, error) {
	return r.autoBan, nil
}

func (r *moderationUserContractRepo) ApplyModerationHit(context.Context, ModerationHit) (ModerationHitResult, error) {
	return ModerationHitResult{Accepted: true, Count: r.state.CurrentCount}, nil
}

type moderationUserContractCache struct{ userIDs []int64 }

func (c *moderationUserContractCache) InvalidateAuthCacheByKey(context.Context, string) {}

func (c *moderationUserContractCache) InvalidateAuthCacheByUserID(_ context.Context, id int64) {
	c.userIDs = append(c.userIDs, id)
}

func (c *moderationUserContractCache) InvalidateAuthCacheByGroupID(context.Context, int64) {}

// These synchronous user-management contracts do not exercise async moderation.
// Leave the repository unset during construction so the unrelated permanent
// cleanup worker cannot outlive the test and call methods absent from the stub.
func newModerationUserContractService(settings SettingRepository, repo ContentModerationRepository, cache APIKeyAuthCacheInvalidator) *ContentModerationService {
	svc := NewContentModerationService(settings, nil, nil, nil, nil, nil, cache, nil)
	svc.repo = repo
	return svc
}

func TestManageModerationUserRequiresVersionReasonAndConfirmation(t *testing.T) {
	repo := &moderationUserContractRepo{state: ModerationUserState{UserID: 27, Status: StatusDisabled, Version: 3, OwnedDisable: true}}
	svc := newModerationUserContractService(&contentModerationTestSettingRepo{}, repo, nil)

	for _, input := range []ModerationUserAction{
		{Action: "unban", Version: 2, Reason: "reviewed", Confirm: true, ActorID: 4},
		{Action: "unban", Version: 3, Confirm: true, ActorID: 4},
		{Action: "unban", Version: 3, Reason: "reviewed", ActorID: 4},
		{Action: "reset", Version: 3, Reason: "reviewed", Confirm: true, ActorID: 4},
	} {
		_, err := svc.ManageModerationUser(context.Background(), 27, input)
		require.Error(t, err)
	}
	require.Zero(t, repo.manageCalls)
}

func TestManageModerationUserUnbanReturnsPersistedStateWithoutResettingCount(t *testing.T) {
	repo := &moderationUserContractRepo{state: ModerationUserState{
		UserID: 27, Version: 3, Status: StatusDisabled, CurrentCount: 8, OwnedDisable: true,
	}}
	cache := &moderationUserContractCache{}
	svc := newModerationUserContractService(&contentModerationTestSettingRepo{}, repo, cache)

	state, err := svc.ManageModerationUser(context.Background(), 27, ModerationUserAction{
		Action: "unban", Version: 3, Reason: "Reviewed moderation source", Confirm: true, ActorID: 4,
	})

	require.NoError(t, err)
	require.Equal(t, StatusActive, state.Status)
	require.Equal(t, int64(4), state.Version)
	require.Equal(t, 8, state.CurrentCount)
	require.False(t, state.OwnedDisable)
	require.Equal(t, "Reviewed moderation source", state.Reason)
	require.Equal(t, []int64{27}, cache.userIDs)
}

func TestManageModerationUserDoesNotReportSuccessOnOwnershipConflict(t *testing.T) {
	repo := &moderationUserContractRepo{
		state:     ModerationUserState{UserID: 27, Version: 3, Status: StatusDisabled},
		manageErr: errors.New("MODERATION_DISABLE_NOT_OWNED"),
	}
	cache := &moderationUserContractCache{}
	svc := newModerationUserContractService(nil, repo, cache)

	_, err := svc.ManageModerationUser(context.Background(), 27, ModerationUserAction{
		Action: "unban", Version: 3, Reason: "reviewed", Confirm: true, ActorID: 4,
	})

	require.Error(t, err)
	require.Empty(t, cache.userIDs)
}

func TestListModerationUsersValidatesFilterAndReturnsUIDScopedState(t *testing.T) {
	repo := &moderationUserContractRepo{state: ModerationUserState{UserID: 27, Status: StatusDisabled}}
	svc := newModerationUserContractService(&contentModerationTestSettingRepo{}, repo, nil)

	items, total, err := svc.ListModerationUsers(context.Background(), 1, 20, "enforced", 27)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, int64(27), items[0].UserID)
	require.Zero(t, items[0].Version, "a user without a moderation record is returned as version zero for first-time configuration")
	require.Equal(t, "enforced", repo.listState)
	require.Equal(t, int64(27), repo.listUserID)

	_, _, err = svc.ListModerationUsers(context.Background(), 1, 20, "reset", 27)
	require.Error(t, err)
	require.Equal(t, 1, repo.listCalls)
}

func TestManageModerationUserAllowsFirstExemptionAtVersionZero(t *testing.T) {
	repo := &moderationUserFirstExemptionRepo{moderationUserContractRepo: &moderationUserContractRepo{state: ModerationUserState{UserID: 27, Status: StatusActive}}}
	cache := &moderationUserContractCache{}
	svc := newModerationUserContractService(&contentModerationTestSettingRepo{}, repo, cache)
	exempt := true

	state, err := svc.ManageModerationUser(context.Background(), 27, ModerationUserAction{
		Action: "exemption", Version: 0, Reason: "pre-approved account", Confirm: true, ActorID: 4, Exempt: &exempt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), state.Version)
	require.True(t, state.AutoBanExempt)
	require.Equal(t, "pre-approved account", state.Reason)
	require.Equal(t, int64(4), repo.manageInput.ActorID)
	require.Equal(t, []int64{27}, cache.userIDs)
}

func TestListModerationUsersPreservesConfiguredCountWindowAndCyberFilter(t *testing.T) {
	settings := &contentModerationTestSettingRepo{values: map[string]string{
		SettingKeyContentModerationConfig: `{"violation_window_hours":36,"cyber_policy_exclude_from_ban_count":true}`,
	}}
	repo := &moderationUserContractRepo{state: ModerationUserState{UserID: 27, Status: StatusDisabled}}
	svc := newModerationUserContractService(settings, repo, nil)
	started := time.Now()

	_, _, err := svc.ListModerationUsers(context.Background(), 2, 25, "auto_ban_exempt", 27)

	require.NoError(t, err)
	require.Equal(t, "auto_ban_exempt", repo.listState)
	require.Equal(t, int64(27), repo.listUserID)
	require.InDelta(t, float64(36*time.Hour), float64(time.Since(repo.listSince)), float64(time.Second))
	require.False(t, repo.listSince.Before(started.Add(-36*time.Hour-time.Second)))
	require.True(t, repo.listExcludeCyber)
}

func TestApplyModerationAutoBanUsesOwnershipRepository(t *testing.T) {
	repo := &moderationUserContractRepo{autoBan: ModerationAutoBanResult{Changed: true, Disabled: true}}
	cache := &moderationUserContractCache{}
	svc := newModerationUserContractService(&contentModerationTestSettingRepo{}, repo, cache)

	changed, disabled, err := svc.ApplyModerationAutoBan(context.Background(), 27)

	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, disabled)
	require.Equal(t, []int64{27}, cache.userIDs)
}
