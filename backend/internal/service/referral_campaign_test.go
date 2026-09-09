//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestReferralCampaignValidationRejectsRunningWithoutBudgetAndWindows(t *testing.T) {
	campaign := ReferralCampaign{
		Name:             "August invite",
		Status:           ReferralCampaignStatusRunning,
		RegistrationFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		RegistrationTo:   time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		StartsAt:         time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		EndsAt:           time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		QualificationTo:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		ClaimDeadline:    time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		BudgetTotal:      0,
		MaxEnrollments:   100,
		PayThreshold:     50,
		UsageThreshold:   1,
		RiskHoldHours:    168,
		Version:          1,
	}
	err := ValidateReferralCampaign(&campaign)
	require.Error(t, err)
	require.Contains(t, err.Error(), "budget")
}

func TestReferralCampaignTokenRejectsTamperingAndExpiry(t *testing.T) {
	secret := []byte("referral-campaign-test-secret")
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	token, err := SignReferralCampaignToken(secret, ReferralCampaignTokenPayload{
		CampaignID: 9,
		InviterID:  42,
		Nonce:      "nonce-1",
		ExpiresAt:  now.Add(time.Hour),
	})
	require.NoError(t, err)

	payload, err := VerifyReferralCampaignToken(secret, token, now)
	require.NoError(t, err)
	require.Equal(t, int64(42), payload.InviterID)

	_, err = VerifyReferralCampaignToken(secret, token+"x", now)
	require.Error(t, err)
	_, err = VerifyReferralCampaignToken(secret, token, now.Add(2*time.Hour))
	require.Error(t, err)
}

func TestReferralCampaignValidationEnforcesSevenDayRiskHold(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	campaign := ReferralCampaign{
		Name: "August invite", Status: ReferralCampaignStatusDraft, RegistrationFrom: now, RegistrationTo: now.Add(24 * time.Hour),
		StartsAt: now, EndsAt: now.Add(7 * 24 * time.Hour), QualificationTo: now.Add(8 * 24 * time.Hour),
		ClaimDeadline: now.Add(14 * 24 * time.Hour), BudgetTotal: 1000, MaxEnrollments: 100, RiskHoldHours: 167,
	}

	err := ValidateReferralCampaign(&campaign)

	require.ErrorContains(t, err, "168")
	campaign.RiskHoldHours = 168
	require.NoError(t, ValidateReferralCampaign(&campaign))
}

func TestReferralQualificationRequiresNetPaymentUsageAndRiskApproval(t *testing.T) {
	base := ReferralQualificationInput{NetPaid: 100, ActualCost: 5, RiskStatus: ReferralRiskApproved}
	require.True(t, ReferralQualificationMeetsThresholds(base, 50, 1))

	base.RiskStatus = ReferralRiskPending
	require.False(t, ReferralQualificationMeetsThresholds(base, 50, 1))
	base.RiskStatus = ReferralRiskApproved
	base.NetPaid = 49.99
	require.False(t, ReferralQualificationMeetsThresholds(base, 50, 1))
	base.NetPaid = 100
	base.ActualCost = 0
	require.False(t, ReferralQualificationMeetsThresholds(base, 50, 1))
}

func TestReferralCampaignServiceRejectsClaimWhenCampaignVersionChanged(t *testing.T) {
	repo := &referralCampaignMemoryRepo{campaign: &ReferralCampaign{ID: 7, Version: 2, Status: ReferralCampaignStatusRunning}}
	svc := NewReferralCampaignService(repo)
	_, err := svc.ClaimReward(context.Background(), ReferralClaimInput{CampaignID: 7, UserID: 42, RewardID: 11, CampaignVersion: 1})
	require.ErrorIs(t, err, ErrReferralCampaignVersionConflict)
}

type referralCampaignMemoryRepo struct {
	ReferralCampaignRepository
	campaign         *ReferralCampaign
	secret           []byte
	enrollment       *ReferralCampaignEnrollment
	pendingFinancial *ReferralCampaignFinancialVersion
	reviewedVersion  int64
	earlyCloseID     int64
	earlyCloseVer    int64
	earlyCloseActor  int64
	earlyCloseReason string
	earlyCloseErr    error
	claimErr         error
	attribution      *ReferralAttribution
	recomputeCalled  bool
}

func (r *referralCampaignMemoryRepo) GetReferralCampaign(_ context.Context, _ int64) (*ReferralCampaign, error) {
	return r.campaign, nil
}

func (r *referralCampaignMemoryRepo) ClaimReferralReward(context.Context, ReferralClaimInput) (*ReferralReward, error) {
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	return &ReferralReward{ID: 11, Status: ReferralRewardStatusClaimedFrozen}, nil
}

func (r *referralCampaignMemoryRepo) GetReferralCampaignEarlyClosePreview(context.Context, int64) (*ReferralCampaignEarlyClosePreview, error) {
	return &ReferralCampaignEarlyClosePreview{}, nil
}

func (r *referralCampaignMemoryRepo) EarlyCloseReferralCampaign(_ context.Context, id, version, actorID int64, reason string) (*ReferralCampaign, error) {
	r.earlyCloseID = id
	r.earlyCloseVer = version
	r.earlyCloseActor = actorID
	r.earlyCloseReason = reason
	if r.earlyCloseErr != nil {
		return nil, r.earlyCloseErr
	}
	return r.campaign, nil
}

func (r *referralCampaignMemoryRepo) GetReferralCampaignSecret(context.Context, int64) ([]byte, error) {
	return r.secret, nil
}

func (r *referralCampaignMemoryRepo) GetReferralCampaignEnrollment(context.Context, int64, int64) (*ReferralCampaignEnrollment, error) {
	return r.enrollment, nil
}

func (r *referralCampaignMemoryRepo) GetReferralAttribution(context.Context, int64, int64) (*ReferralAttribution, error) {
	return r.attribution, nil
}

func (r *referralCampaignMemoryRepo) RecomputeReferralQualification(context.Context, int64, int64, time.Time) (*ReferralQualification, error) {
	r.recomputeCalled = true
	return &ReferralQualification{}, nil
}

func (r *referralCampaignMemoryRepo) GetPendingReferralCampaignFinancialVersion(context.Context, int64) (*ReferralCampaignFinancialVersion, error) {
	return r.pendingFinancial, nil
}

func (r *referralCampaignMemoryRepo) ReviewReferralCampaign(_ context.Context, _ int64, version int64, _ string, _ string, _ int64, _ string) (*ReferralCampaign, error) {
	r.reviewedVersion = version
	return r.campaign, nil
}

func TestReferralCampaignServiceReviewsPendingFinancialVersionWithoutPausingRunningCampaign(t *testing.T) {
	repo := &referralCampaignMemoryRepo{
		campaign:         &ReferralCampaign{ID: 7, Version: 4, Status: ReferralCampaignStatusRunning, RulesVersion: 3},
		pendingFinancial: &ReferralCampaignFinancialVersion{RulesVersion: 4, BaseRulesVersion: 3, Status: "review"},
	}
	svc := NewReferralCampaignService(repo)

	_, err := svc.Review(context.Background(), 7, 4, "finance", "approved", 42, "funds checked")

	require.NoError(t, err)
	require.Equal(t, int64(4), repo.reviewedVersion)
	require.Equal(t, ReferralCampaignStatusRunning, repo.campaign.Status)
}

func TestReferralCampaignValidateInviteTokenBeforeRegistration(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	secret := []byte("referral-campaign-validation-secret")
	token, err := SignReferralCampaignToken(secret, ReferralCampaignTokenPayload{
		CampaignID: 7, InviterID: 42, Nonce: "registration-preflight", ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	repo := &referralCampaignMemoryRepo{
		campaign: &ReferralCampaign{ID: 7, Status: ReferralCampaignStatusRunning, RegistrationFrom: now.Add(-time.Hour), RegistrationTo: now.Add(time.Hour)},
		secret:   secret, enrollment: &ReferralCampaignEnrollment{CampaignID: 7, UserID: 42},
	}
	svc := NewReferralCampaignService(repo)

	inviterID, err := svc.ValidateInviteToken(context.Background(), token, now)

	require.NoError(t, err)
	require.Equal(t, int64(42), inviterID)
	_, err = svc.ValidateInviteToken(context.Background(), token+"tampered", now)
	require.ErrorIs(t, err, ErrReferralCampaignTokenInvalid)
}

func TestReferralCampaignEarlyCloseRequiresSettlingWindowReasonAndCurrentVersion(t *testing.T) {
	now := time.Now().UTC()
	repo := &referralCampaignMemoryRepo{campaign: &ReferralCampaign{
		ID: 7, Version: 3, Status: ReferralCampaignStatusSettling, ClaimDeadline: now.Add(time.Hour),
	}}
	svc := NewReferralCampaignService(repo)

	_, err := svc.EarlyClose(context.Background(), 7, 3, 42, " \t ")
	require.ErrorIs(t, err, ErrReferralCampaignEarlyCloseReason)
	_, err = svc.EarlyClose(context.Background(), 7, 3, 42, strings.Repeat("x", 501))
	require.ErrorIs(t, err, ErrReferralCampaignEarlyCloseReason)
	_, err = svc.EarlyClose(context.Background(), 7, 2, 42, "operations reviewed")
	require.ErrorIs(t, err, ErrReferralCampaignVersionConflict)

	repo.campaign.Status = ReferralCampaignStatusRunning
	_, err = svc.EarlyClose(context.Background(), 7, 3, 42, "operations reviewed")
	require.ErrorIs(t, err, ErrReferralCampaignInvalidState)
	repo.campaign.Status = ReferralCampaignStatusSettling
	repo.campaign.ClaimDeadline = now.Add(-time.Minute)
	_, err = svc.EarlyClose(context.Background(), 7, 3, 42, "operations reviewed")
	require.ErrorIs(t, err, ErrReferralCampaignInvalidState)
}

func TestReferralCampaignEarlyCloseDelegatesTrimmedReasonOnceValidated(t *testing.T) {
	repo := &referralCampaignMemoryRepo{campaign: &ReferralCampaign{
		ID: 7, Version: 3, Status: ReferralCampaignStatusSettling, ClaimDeadline: time.Now().UTC().Add(time.Hour),
	}}
	svc := NewReferralCampaignService(repo)

	_, err := svc.EarlyClose(context.Background(), 7, 3, 42, "  finance and operations approved  ")

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.earlyCloseID)
	require.Equal(t, int64(3), repo.earlyCloseVer)
	require.Equal(t, int64(42), repo.earlyCloseActor)
	require.Equal(t, "finance and operations approved", repo.earlyCloseReason)
}

func TestReferralCampaignClaimWrapsUnexpectedRepositoryFailureWithoutLeakingCause(t *testing.T) {
	dbErr := errors.New("duplicate key value violates unique constraint user_affiliate_ledger_reward_key")
	repo := &referralCampaignMemoryRepo{
		campaign: &ReferralCampaign{ID: 7, Version: 3, Status: ReferralCampaignStatusSettling},
		claimErr: dbErr,
	}
	svc := NewReferralCampaignService(repo)

	_, err := svc.ClaimReward(context.Background(), ReferralClaimInput{CampaignID: 7, UserID: 42, RewardID: 11, CampaignVersion: 3})

	require.Equal(t, "REFERRAL_REWARD_CLAIM_FAILED", infraerrors.Reason(err))
	require.Equal(t, 500, infraerrors.Code(err))
	require.NotContains(t, infraerrors.Message(err), "duplicate key")
	require.ErrorIs(t, err, dbErr)
}

func TestReferralCampaignClaimPreservesKnownDomainFailures(t *testing.T) {
	repo := &referralCampaignMemoryRepo{
		campaign: &ReferralCampaign{ID: 7, Version: 3, Status: ReferralCampaignStatusSettling},
		claimErr: ErrReferralRewardNotClaimable,
	}
	svc := NewReferralCampaignService(repo)

	_, err := svc.ClaimReward(context.Background(), ReferralClaimInput{CampaignID: 7, UserID: 42, RewardID: 11, CampaignVersion: 3})

	require.ErrorIs(t, err, ErrReferralRewardNotClaimable)
}

func TestReferralCampaignSettlingStopsNewQualificationRecomputation(t *testing.T) {
	repo := &referralCampaignMemoryRepo{
		campaign:    &ReferralCampaign{ID: 7, Status: ReferralCampaignStatusSettling},
		attribution: &ReferralAttribution{CampaignID: 7, InviteeID: 42, QualificationToSnapshot: time.Now().UTC().Add(time.Hour)},
	}
	svc := NewReferralCampaignService(repo)

	_, err := svc.RecomputeQualification(context.Background(), 7, 42, time.Now().UTC())

	require.ErrorIs(t, err, ErrReferralCampaignNotOpen)
	require.False(t, repo.recomputeCalled)
}
