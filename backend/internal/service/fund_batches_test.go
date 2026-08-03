package service

import "testing"

import "github.com/stretchr/testify/require"

func TestClassifyFundBatchGrantKeepsRewardAndUnknownCreditsNonPaid(t *testing.T) {
	t.Parallel()

	for source, wantKind := range map[string]string{
		"affiliate_balance":                 FundSourceKindPromotionGift,
		PlayRewardSourceCheckin:             FundSourceKindPromotionGift,
		PlayRewardSourceCheckinMilestone:    FundSourceKindPromotionGift,
		PlayRewardSourceCheckinMakeup:       FundSourceKindPromotionGift,
		PlayRewardSourceBlindbox:            FundSourceKindPromotionGift,
		PlayRewardSourceQuiz:                FundSourceKindPromotionGift,
		PlayRewardSourceArenaSettlement:     FundSourceKindPromotionGift,
		PlayRewardSourceArenaDaily:          FundSourceKindPromotionGift,
		PlayRewardSourceTeamSharedReward:    FundSourceKindPromotionGift,
		"future_unrecognized_credit_source": FundSourceKindUnknown,
	} {
		source, wantKind := source, wantKind
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			got := classifyFundBatchGrant(source, "source-1", nil)
			require.True(t, got.Eligible)
			require.Equal(t, wantKind, got.SourceKind)
			require.False(t, got.Refundable)
			require.Nil(t, got.PaymentOrderID)
		})
	}
}
