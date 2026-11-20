//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageMinimumChargeLeavesSupplierAndTokenCostIntact(t *testing.T) {
	cost := &CostBreakdown{BillingMode: string(BillingModeToken), InputCost: 0.002, OutputCost: 0.003, TotalCost: 0.005, ActualCost: 0.003}
	floor, adjustment := applyImageMinimumCharge(cost, 2, 0.025)
	require.InDelta(t, 0.05, floor, 1e-10)
	require.InDelta(t, 0.047, adjustment, 1e-10)
	require.InDelta(t, 0.05, cost.ActualCost, 1e-10)
	require.Equal(t, 0.005, cost.TotalCost)
	require.Equal(t, 0.002, cost.InputCost)
	require.Equal(t, 0.003, cost.OutputCost)

	floor, adjustment = applyImageMinimumCharge(cost, 0, 10)
	require.Zero(t, floor)
	require.Zero(t, adjustment)
	cost.BillingMode = string(BillingModeImage)
	floor, _ = applyImageMinimumCharge(cost, 2, 100)
	require.Zero(t, floor)
	require.Equal(t, 0.05, cost.ActualCost)
}

func TestGroupImageMinimumChargeValidation(t *testing.T) {
	for _, tc := range []struct {
		platform string
		mode     BillingMode
		price    float64
		valid    bool
	}{
		{PlatformOpenAI, BillingModeToken, 0, true},
		{PlatformOpenAI, BillingModeToken, 0.00123456, true},
		{PlatformOpenAI, BillingModeToken, 0.001234567, false},
		{PlatformOpenAI, BillingModeToken, math.Inf(1), false},
		{PlatformOpenAI, BillingModeToken, -1, false},
		{PlatformOpenAI, BillingModeToken, 1001, false},
		{PlatformOpenAI, BillingModeImage, 0.05, false},
		{PlatformGrok, BillingModeToken, 0.05, false},
	} {
		price := tc.price
		card := ChannelModelPricing{Models: []string{"test-image"}, BillingMode: tc.mode, ImageMinimumCharge: &price}
		out, err := normalizeGroupModelPricing(tc.platform, []ChannelModelPricing{card})
		if !tc.valid {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, price, *out[0].ImageMinimumCharge)
	}
}

func TestImageBillingReceiptRequiresVerifiedSettlement(t *testing.T) {
	receipt := ImageBillingReceipt{ActualCost: 0.01234567}
	_, err := receipt.EquivalentTokens()
	require.Error(t, err)
	receipt.Settled = true
	count, err := receipt.EquivalentTokens()
	require.NoError(t, err)
	require.EqualValues(t, 1234567, count)
}
