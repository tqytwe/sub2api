package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyBillingSurcharge_PreservesRateMultipliedActualCost(t *testing.T) {
	t.Parallel()

	cost := &CostBreakdown{TotalCost: 1, ActualCost: 0.25}
	tests := []struct {
		name          string
		config        BillingSurchargeConfig
		wantEnabled   bool
		wantSurcharge float64
		wantBilled    float64
	}{
		{
			name:       "disabled keeps the original group-multiplied charge",
			config:     BillingSurchargeConfig{},
			wantBilled: 0.25,
		},
		{
			name: "three per mille applies only to the multiplied charge",
			config: BillingSurchargeConfig{
				Enabled: true,
				Mode:    BillingSurchargeModePercentOnChargedCost,
				Value:   0.003,
			},
			wantEnabled:   true,
			wantSurcharge: 0.00075,
			wantBilled:    0.25075,
		},
		{
			name: "additive multiplier adds to but does not replace the group multiplier",
			config: BillingSurchargeConfig{
				Enabled: true,
				Mode:    BillingSurchargeModeAdditiveMultiplier,
				Value:   0.05,
			},
			wantEnabled:   true,
			wantSurcharge: 0.05,
			wantBilled:    0.30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ApplyBillingSurcharge(cost, tt.config)

			require.Equal(t, tt.wantEnabled, got.Enabled)
			require.InDelta(t, 0.25, cost.ActualCost, 1e-12)
			require.InDelta(t, 0.25, got.BaseCost, 1e-12)
			require.InDelta(t, tt.wantSurcharge, got.SurchargeCost, 1e-12)
			require.InDelta(t, tt.wantBilled, got.BilledCost, 1e-12)
		})
	}
}

func TestBuildUsageBillingCommand_ChargesFinalAmountExactlyOnce(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subscriptionID := int64(11)
	cost := &CostBreakdown{TotalCost: 1, ActualCost: 0.25}
	surcharge := ApplyBillingSurcharge(cost, BillingSurchargeConfig{
		Enabled: true,
		Mode:    BillingSurchargeModeAdditiveMultiplier,
		Value:   0.05,
	})

	tests := []struct {
		name             string
		subscription     bool
		wantBalance      float64
		wantSubscription float64
	}{
		{
			name:        "balance billing writes only the final charge",
			wantBalance: 0.30,
		},
		{
			name:             "subscription billing writes only the final charge",
			subscription:     true,
			wantSubscription: 0.30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			params := &postUsageBillingParams{
				Cost:               cost,
				User:               &User{ID: 1},
				APIKey:             &APIKey{ID: 2, GroupID: &groupID, Quota: 10},
				Account:            &Account{ID: 3},
				Subscription:       &UserSubscription{ID: subscriptionID},
				IsSubscriptionBill: tt.subscription,
				APIKeyService:      billingSurchargeQuotaUpdaterStub{},
				Surcharge:          surcharge,
			}

			cmd := buildUsageBillingCommandForContext(context.Background(), "req-once", nil, params)
			require.NotNil(t, cmd)
			require.InDelta(t, 0.25, cmd.ActualCost, 1e-12)
			require.InDelta(t, 0.05, cmd.BillingSurchargeCost, 1e-12)
			require.InDelta(t, 0.30, cmd.BilledCost, 1e-12)
			require.InDelta(t, tt.wantBalance, cmd.BalanceCost, 1e-12)
			require.InDelta(t, tt.wantSubscription, cmd.SubscriptionCost, 1e-12)
			require.InDelta(t, 0.30, cmd.APIKeyQuotaCost, 1e-12)
			require.Zero(t, cmd.BalanceCost*cmd.SubscriptionCost)
		})
	}
}

type billingSurchargeQuotaUpdaterStub struct{}

func (billingSurchargeQuotaUpdaterStub) UpdateQuotaUsed(context.Context, int64, float64) error {
	return nil
}

func (billingSurchargeQuotaUpdaterStub) UpdateRateLimitUsage(context.Context, int64, float64) error {
	return nil
}

func TestResolveGroupBillingSurcharge_DoesNotChangeGroupRateMultiplier(t *testing.T) {
	t.Parallel()

	group := &Group{
		RateMultiplier:                  0.25,
		BillingSurchargeOverrideEnabled: true,
		BillingSurchargeEnabled:         true,
		BillingSurchargeMode:            BillingSurchargeModeAdditiveMultiplier,
		BillingSurchargeValue:           0.05,
	}

	cfg := ResolveGroupBillingSurcharge(group, BillingSurchargeConfig{
		Enabled: true,
		Mode:    BillingSurchargeModePercentOnChargedCost,
		Value:   0.003,
	})

	require.InDelta(t, 0.25, group.RateMultiplier, 1e-12)
	require.True(t, cfg.Enabled)
	require.Equal(t, BillingSurchargeModeAdditiveMultiplier, cfg.Mode)
	require.InDelta(t, 0.05, cfg.Value, 1e-12)
}
