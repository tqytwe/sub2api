package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExcelBPSCapabilitiesProjectsOnlyUsableModels(t *testing.T) {
	caps, err := ParseExcelBPSCapabilities([]byte(`{
      "allowed": true,
      "model_catalog": {"restricted_models":["gpt-restricted"],"models":[
        {"id":"gpt-5.6-sol","label":"Sol","available":true,"enabled":true,"model_picker_enabled":true,"efforts":[{"value":"low"},{"value":"high"},{"value":"high"}]},
        {"id":"gpt-restricted","available":true},
        {"id":"gpt-disabled","enabled":false},
        {"id":"not-a-model","available":true},
        {"id":"gpt-unconfigured","policy":{"state":"unconfigured"}},
        {"id":"gpt-5.6-sol","available":true}
      ]}
    }`))
	require.NoError(t, err)
	require.True(t, caps.Allowed)
	require.Len(t, caps.Models, 1)
	require.Equal(t, "gpt-5.6-sol", caps.Models[0].ID)
	require.Equal(t, []string{"low", "high"}, caps.Models[0].ReasoningEfforts)
	require.True(t, caps.AllowsModel("gpt-5.6-sol"))
	require.False(t, caps.AllowsModel("gpt-disabled"))
}

func TestParseExcelBPSCapabilitiesDeniedAndMalformedAreSafe(t *testing.T) {
	denied, err := ParseExcelBPSCapabilities([]byte(`{"allowed":false}`))
	require.NoError(t, err)
	require.False(t, denied.Allowed)
	require.Empty(t, denied.Models)
	require.False(t, denied.AllowsModel("gpt-5.6-sol"))

	_, err = ParseExcelBPSCapabilities([]byte(`{"allowed":true}`))
	require.Error(t, err)
	_, err = ParseExcelBPSCapabilities([]byte(`{"allowed":"yes"}`))
	require.Error(t, err)
}
