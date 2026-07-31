//go:build unit

package service

import (
	"context"
	"testing"
	"time"

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
	campaign   *ReferralCampaign
	secret     []byte
	enrollment *ReferralCampaignEnrollment
}

func (r *referralCampaignMemoryRepo) GetReferralCampaign(_ context.Context, _ int64) (*ReferralCampaign, error) {
	return r.campaign, nil
}

func (r *referralCampaignMemoryRepo) ClaimReferralReward(context.Context, ReferralClaimInput) (*ReferralReward, error) {
	return &ReferralReward{ID: 11, Status: ReferralRewardStatusClaimedFrozen}, nil
}

func (r *referralCampaignMemoryRepo) GetReferralCampaignSecret(context.Context, int64) ([]byte, error) {
	return r.secret, nil
}

func (r *referralCampaignMemoryRepo) GetReferralCampaignEnrollment(context.Context, int64, int64) (*ReferralCampaignEnrollment, error) {
	return r.enrollment, nil
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
