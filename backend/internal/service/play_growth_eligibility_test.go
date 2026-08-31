package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvaluateGrowthEligibilityRequiresVerifiedEstablishedAccount(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	t.Run("unverified users remain in the exploration tier", func(t *testing.T) {
		result := EvaluateGrowthEligibility(PlayGrowthEligibilitySignals{
			EmailVerified:  true,
			CreatedAt:      now.AddDate(0, 0, -2),
			HasRecentUsage: true,
		}, now)

		require.Equal(t, PlayGrowthTierExplorer, result.Tier)
		require.Equal(t, PlayGrowthRewardEnergy, result.RewardMode)
		require.Equal(t, PlayGrowthEligibilityReasonAccountTooNew, result.PrimaryReason)
	})

	t.Run("established verified users unlock redeemable rewards through real use", func(t *testing.T) {
		result := EvaluateGrowthEligibility(PlayGrowthEligibilitySignals{
			EmailVerified:  true,
			CreatedAt:      now.AddDate(0, 0, -3),
			HasRecentUsage: true,
		}, now)

		require.Equal(t, PlayGrowthTierActive, result.Tier)
		require.Equal(t, PlayGrowthRewardRedeemable, result.RewardMode)
		require.Equal(t, PlayGrowthEligibilityReasonEligible, result.PrimaryReason)
	})

	t.Run("net balance recharge and active subscription are alternate real activity paths", func(t *testing.T) {
		for _, signals := range []PlayGrowthEligibilitySignals{
			{EmailVerified: true, CreatedAt: now.AddDate(0, 0, -3), NetBalanceRecharge30d: 10},
			{EmailVerified: true, CreatedAt: now.AddDate(0, 0, -3), HasActiveSubscription: true},
		} {
			result := EvaluateGrowthEligibility(signals, now)
			require.Equal(t, PlayGrowthTierActive, result.Tier)
			require.Equal(t, PlayGrowthRewardRedeemable, result.RewardMode)
		}
	})

	t.Run("a historical recharge outside the window cannot unlock recurring rewards", func(t *testing.T) {
		result := EvaluateGrowthEligibility(PlayGrowthEligibilitySignals{
			EmailVerified:         true,
			CreatedAt:             now.AddDate(0, 0, -10),
			NetBalanceRecharge30d: 9.99,
		}, now)

		require.Equal(t, PlayGrowthTierExplorer, result.Tier)
		require.Equal(t, PlayGrowthEligibilityReasonNoRecentActivity, result.PrimaryReason)
	})
}

func TestGrowthQualificationRequiredFailsClosedWhenRepositoryPortIsMissing(t *testing.T) {
	svc := NewPlayService(&legacyGrowthTestRepo{}, nil, nil, nil, nil, nil)
	svc.RequireGrowthQualification(true)

	_, err := svc.growthEligibility(context.Background(), 42, time.Date(2026, time.August, 29, 0, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, ErrPlayGrowthQualificationUnavailable)
}

func TestGrowthSnapshotAndEnergyWritesFailClosedWhenRepositoryPortIsMissing(t *testing.T) {
	svc := NewPlayService(&legacyGrowthTestRepo{}, nil, nil, nil, nil, nil)
	svc.RequireGrowthQualification(true)

	_, err := svc.createGrowthSnapshot(context.Background(), PlayGrowthEligibilitySnapshot{ActionID: "checkin:42:2026-08-29"})
	require.ErrorIs(t, err, ErrPlayGrowthQualificationUnavailable)
	err = svc.insertGrowthEnergy(context.Background(), PlayGrowthEnergyLedgerEntry{ActionID: "checkin:42:2026-08-29", Amount: 1})
	require.ErrorIs(t, err, ErrPlayGrowthQualificationUnavailable)
}

func TestGrowthSnapshotFailsClosedWhenRepositoryReturnsEmptyID(t *testing.T) {
	svc := NewPlayService(&zeroSnapshotGrowthRepo{}, nil, nil, nil, nil, nil)

	_, err := svc.createGrowthSnapshot(context.Background(), PlayGrowthEligibilitySnapshot{
		UserID:   42,
		Source:   PlayRewardSourceCheckin,
		ActionID: "checkin:42:2026-08-29",
	})
	require.ErrorIs(t, err, ErrPlayGrowthQualificationUnavailable)
}

// legacyGrowthTestRepo exercises the production fail-closed path without
// coupling this unit test to the concrete SQL repository implementation.
type legacyGrowthTestRepo struct{ PlayRepository }

type zeroSnapshotGrowthRepo struct {
	PlayRepository
	PlayGrowthQualificationRepository
}

func (*zeroSnapshotGrowthRepo) CreateGrowthEligibilitySnapshot(context.Context, PlayGrowthEligibilitySnapshot) (int64, error) {
	return 0, nil
}

func TestGrowthEvidenceHelpersRejectBlankActionIDs(t *testing.T) {
	svc := &PlayService{}

	_, err := svc.createGrowthSnapshot(context.Background(), PlayGrowthEligibilitySnapshot{ActionID: " \t "})
	require.ErrorContains(t, err, "action id is required")

	err = svc.insertGrowthEnergy(context.Background(), PlayGrowthEnergyLedgerEntry{ActionID: " \n ", Amount: 1})
	require.ErrorContains(t, err, "action id is required")
}

func TestRedeemableRewardIssuersRejectExplorerEligibility(t *testing.T) {
	svc := &PlayService{}
	explorer := PlayGrowthEligibility{
		Tier:          PlayGrowthTierExplorer,
		RewardMode:    PlayGrowthRewardEnergy,
		PrimaryReason: PlayGrowthEligibilityReasonNoRecentActivity,
	}

	_, err := svc.issueCouponRewardInTx(context.Background(), 42, CouponRewardActivityQuiz, "quiz:42:2026-08-29", "2026-08-29", time.Now(), explorer)
	require.ErrorIs(t, err, ErrPlayGrowthRewardIneligible)

	_, err = svc.issueRedeemCodeRewardInTx(context.Background(), 42, CouponRewardActivityQuiz, "quiz:42:2026-08-29", "2026-08-29", time.Now(), explorer)
	require.ErrorIs(t, err, ErrPlayGrowthRewardIneligible)
}
