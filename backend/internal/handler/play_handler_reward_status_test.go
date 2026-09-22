package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCheckinStatusDTOExposesGrowthGovernanceBlock(t *testing.T) {
	status := &service.PlayCheckinStatus{
		Enabled:                   true,
		CouponPoolReady:           false,
		GrowthGovernanceAvailable: false,
		GrowthGovernanceReason:    "not_approved",
	}

	payload, err := json.Marshal(toPlayCheckinStatusDTO(status))
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, false, decoded["growth_governance_available"])
	require.Equal(t, "not_approved", decoded["growth_governance_reason"])
}

func TestBlindboxStatusDTOExposesGrowthGovernanceBlock(t *testing.T) {
	status := &service.PlayBlindboxStatus{
		Enabled:                   true,
		CouponPoolReady:           false,
		GrowthGovernanceAvailable: false,
		GrowthGovernanceReason:    "budget_exhausted",
		GrowthEligibility: service.PlayGrowthEligibility{
			RewardMode: service.PlayGrowthRewardRedeemable,
		},
	}

	payload, err := json.Marshal(toPlayBlindboxStatusDTO(status, true))
	require.NoError(t, err)
	require.Contains(t, string(payload), `"growth_governance_available":false`)
	require.Contains(t, string(payload), `"growth_governance_reason":"budget_exhausted"`)
}
