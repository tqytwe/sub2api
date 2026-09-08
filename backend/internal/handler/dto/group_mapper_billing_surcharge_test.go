package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServiceAdminIncludesBillingSurcharge(t *testing.T) {
	group := &service.Group{
		ID:                              7,
		Name:                            "surcharge",
		Platform:                        service.PlatformOpenAI,
		Status:                          service.StatusActive,
		BillingSurchargeOverrideEnabled: true,
		BillingSurchargeEnabled:         true,
		BillingSurchargeMode:            service.BillingSurchargeModeAdditiveMultiplier,
		BillingSurchargeValue:           0.05,
	}

	admin := GroupFromServiceAdmin(group)
	require.True(t, admin.BillingSurchargeOverrideEnabled)
	require.True(t, admin.BillingSurchargeEnabled)
	require.Equal(t, service.BillingSurchargeModeAdditiveMultiplier, admin.BillingSurchargeMode)
	require.InDelta(t, 0.05, admin.BillingSurchargeValue, 1e-12)

	raw, err := json.Marshal(admin)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, true, body["billing_surcharge_override_enabled"])
	require.Equal(t, true, body["billing_surcharge_enabled"])
	require.Equal(t, service.BillingSurchargeModeAdditiveMultiplier, body["billing_surcharge_mode"])
	require.Equal(t, 0.05, body["billing_surcharge_value"])
}
