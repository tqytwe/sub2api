package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestBuildPaymentRechargeQuoteUsesPreRechargeVIPTier(t *testing.T) {
	quote := buildPaymentRechargeQuote(50, 1, 90, defaultPlayVIPTiers(), paymentRechargeCampaignBonus{BonusPct: 5, CampaignIDs: []int64{7}})

	require.Equal(t, 1, quote.CurrentVIP.Tier)
	require.Equal(t, "V1", quote.CurrentVIP.Label)
	require.Equal(t, 2.0, quote.VIPBonusPct)
	require.Equal(t, 5.0, quote.CampaignBonusPct)
	require.Equal(t, []int64{7}, quote.CampaignIDs)
	require.Equal(t, 50.0, quote.BaseCredited)
	require.Equal(t, 53.5, quote.CreditedAmount)
	require.Equal(t, 140.0, quote.TotalRechargedAfterBase)
	require.True(t, quote.VIPUpgradeAppliesNextOrder)
}

func TestBuildPaymentRechargeQuoteAddsVIPAndCampaignBonusAfterBaseMultiplier(t *testing.T) {
	quote := buildPaymentRechargeQuote(33.335, 1.2, 200, defaultPlayVIPTiers(), paymentRechargeCampaignBonus{BonusPct: 12.5})

	require.Equal(t, 3, quote.CurrentVIP.Tier)
	require.Equal(t, 6.0, quote.VIPBonusPct)
	require.Equal(t, 12.5, quote.CampaignBonusPct)
	require.Equal(t, 40.0, quote.BaseCredited)
	require.Equal(t, 47.4, quote.CreditedAmount)
	require.Equal(t, 1.422, quote.EffectiveCreditMultiplier)
}

func TestPaymentOrderRechargeBaseCreditedUsesSnapshotForAffiliateBase(t *testing.T) {
	order := &dbent.PaymentOrder{
		OrderType: payment.OrderTypeBalance,
		Amount:    116,
		RechargeSnapshot: map[string]any{
			"base_credited": 100,
		},
	}

	got, ok := paymentOrderRechargeBaseCredited(order)

	require.True(t, ok)
	require.Equal(t, 100.0, got)
	require.Equal(t, 100.0, affiliateRebateBaseAmount(order))
}

func TestPaymentOrderRefundTotalRechargedDeltaUsesSnapshotRatio(t *testing.T) {
	order := &dbent.PaymentOrder{
		OrderType: payment.OrderTypeBalance,
		Amount:    110,
		RechargeSnapshot: map[string]any{
			"base_credited": 100,
		},
	}

	require.Equal(t, -50.0, paymentOrderRefundTotalRechargedDelta(order, 55))
	require.Equal(t, -100.0, paymentOrderRefundTotalRechargedDelta(order, 220))
}

func TestPaymentOrderRechargeBaseCreditedFallsBackForLegacyOrders(t *testing.T) {
	order := &dbent.PaymentOrder{OrderType: payment.OrderTypeBalance, Amount: 80}

	got, ok := paymentOrderRechargeBaseCredited(order)

	require.False(t, ok)
	require.Equal(t, 80.0, got)
	require.Equal(t, -20.0, paymentOrderRefundTotalRechargedDelta(order, 20))
}
