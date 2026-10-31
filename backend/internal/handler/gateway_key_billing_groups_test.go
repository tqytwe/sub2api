package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMultiGroupBillingDeclarationOmitsSingleRate(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	entries := []keyBillingGroup{
		{GroupID: 7, GroupName: "first", Platform: "openai", Status: "active", keyBillingInfoResponse: keyBillingInfoResponse{Object: "sub2api.key_billing", SchemaVersion: 1, BillingScope: "token", EffectiveRateMultiplier: 0.5, ObservedAt: now}},
		{GroupID: 8, GroupName: "second", Platform: "openai", Status: "disabled", keyBillingInfoResponse: keyBillingInfoResponse{Object: "sub2api.key_billing", SchemaVersion: 1, BillingScope: "token", EffectiveRateMultiplier: 2, ObservedAt: now}},
	}
	body, err := json.Marshal(buildMultiGroupKeyBillingInfo(entries, now))
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	require.EqualValues(t, 2, got["schema_version"])
	require.Equal(t, "multi_group", got["billing_mode"])
	require.NotContains(t, got, "effective_rate_multiplier")
	require.NotContains(t, got, "resolved_rate_multiplier")
	groups, ok := got["groups"].([]any)
	require.True(t, ok)
	require.Len(t, groups, 2)
	disabled, ok := groups[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "disabled", disabled["status"])
}
