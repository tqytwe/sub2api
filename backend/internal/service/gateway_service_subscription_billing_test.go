//go:build unit

package service

import (
	"context"
	"math"
	"testing"
)

// TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier locks in the fix
// that subscription-mode billing honours the group (and any user-specific) rate
// multiplier — i.e. cmd.SubscriptionCost tracks ActualCost (= TotalCost *
// RateMultiplier), not raw TotalCost.
func TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subID := int64(42)

	tests := []struct {
		name           string
		totalCost      float64
		actualCost     float64
		isSubscription bool
		wantSub        float64
		wantBalance    float64
	}{
		{
			name:           "subscription with 2x multiplier consumes 2x quota",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			wantSub:        2.0,
			wantBalance:    0,
		},
		{
			name:           "subscription with 0.5x multiplier consumes 0.5x quota",
			totalCost:      1.0,
			actualCost:     0.5,
			isSubscription: true,
			wantSub:        0.5,
			wantBalance:    0,
		},
		{
			name:           "free subscription (multiplier 0) consumes no quota",
			totalCost:      1.0,
			actualCost:     0,
			isSubscription: true,
			wantSub:        0,
			wantBalance:    0,
		},
		{
			name:           "balance billing keeps using ActualCost (regression)",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: false,
			wantSub:        0,
			wantBalance:    2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &postUsageBillingParams{
				Cost:               &CostBreakdown{TotalCost: tt.totalCost, ActualCost: tt.actualCost},
				User:               &User{ID: 1},
				APIKey:             &APIKey{ID: 2, GroupID: &groupID},
				Account:            &Account{ID: 3},
				Subscription:       &UserSubscription{ID: subID},
				IsSubscriptionBill: tt.isSubscription,
			}

			cmd := buildUsageBillingCommandForContext(context.Background(), "req-1", nil, p)
			if cmd == nil {
				t.Fatal("buildUsageBillingCommandForContext returned nil")
			}
			if cmd.SubscriptionCost != tt.wantSub {
				t.Errorf("SubscriptionCost = %v, want %v", cmd.SubscriptionCost, tt.wantSub)
			}
			if cmd.BalanceCost != tt.wantBalance {
				t.Errorf("BalanceCost = %v, want %v", cmd.BalanceCost, tt.wantBalance)
			}
		})
	}
}

func TestBuildUsageBillingCommand_SurchargeBilledCostDoesNotPolluteActualCost(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{TotalCost: 1.0, ActualCost: 0.25},
		User: &User{ID: 1},
		APIKey: &APIKey{
			ID:      2,
			GroupID: &groupID,
			Group: &Group{
				ID:   groupID,
				Name: "paid-group",
			},
			Quota: 10,
		},
		Account:       &Account{ID: 3},
		APIKeyService: apiKeyQuotaUpdaterStub{},
		Surcharge: ApplyBillingSurcharge(
			&CostBreakdown{TotalCost: 1.0, ActualCost: 0.25},
			BillingSurchargeConfig{
				Enabled: true,
				Mode:    BillingSurchargeModeAdditiveMultiplier,
				Value:   0.05,
			},
		),
	}

	cmd := buildUsageBillingCommandForContext(context.Background(), "req-surcharge", nil, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommandForContext returned nil")
	}
	if cmd.ActualCost != 0.25 {
		t.Fatalf("ActualCost = %v, want original 0.25", cmd.ActualCost)
	}
	if math.Abs(cmd.BillingSurchargeCost-0.05) > 1e-9 {
		t.Fatalf("BillingSurchargeCost = %v, want 0.05", cmd.BillingSurchargeCost)
	}
	if math.Abs(cmd.BilledCost-0.30) > 1e-9 {
		t.Fatalf("BilledCost = %v, want 0.30", cmd.BilledCost)
	}
	if math.Abs(cmd.BalanceCost-0.30) > 1e-9 {
		t.Fatalf("BalanceCost = %v, want 0.30", cmd.BalanceCost)
	}
	if math.Abs(cmd.APIKeyQuotaCost-0.30) > 1e-9 {
		t.Fatalf("APIKeyQuotaCost = %v, want 0.30", cmd.APIKeyQuotaCost)
	}
	if cmd.APIKeyGroupID == nil || *cmd.APIKeyGroupID != groupID {
		t.Fatalf("APIKeyGroupID = %v, want %d", cmd.APIKeyGroupID, groupID)
	}
	if cmd.APIKeyGroupName != "paid-group" {
		t.Fatalf("APIKeyGroupName = %q, want paid-group", cmd.APIKeyGroupName)
	}
}

type apiKeyQuotaUpdaterStub struct{}

func (apiKeyQuotaUpdaterStub) UpdateQuotaUsed(context.Context, int64, float64) error {
	return nil
}

func (apiKeyQuotaUpdaterStub) UpdateRateLimitUsage(context.Context, int64, float64) error {
	return nil
}
