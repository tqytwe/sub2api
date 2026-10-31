package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAPIKeyGroupIDs(t *testing.T) {
	first := int64(11)
	got, err := normalizeAPIKeyGroupIDs(&first, []int64{11, 12})
	require.NoError(t, err)
	require.Equal(t, []int64{11, 12}, got)

	_, err = normalizeAPIKeyGroupIDs(&first, []int64{12, 11})
	require.Error(t, err)
	_, err = normalizeAPIKeyGroupIDs(nil, []int64{11, 11})
	require.Error(t, err)
	tooMany := make([]int64, maxAPIKeyGroups+1)
	for i := range tooMany { tooMany[i] = int64(i + 1) }
	_, err = normalizeAPIKeyGroupIDs(nil, tooMany)
	require.Error(t, err)
}

func TestParseMultiGroupBillingDeclaration(t *testing.T) {
	stamp := "2026-10-31T12:00:00Z"
	entry := map[string]any{"object": "sub2api.key_billing", "schema_version": 1, "billing_scope": "token", "group_id": 7, "group_name": "primary", "platform": "openai", "status": "active", "group_rate_multiplier": 1.1, "resolved_rate_multiplier": 1.1, "effective_rate_multiplier": 1.1, "peak_rate_enabled": false, "observed_at": stamp}
	body, err := json.Marshal(map[string]any{"object": "sub2api.key_billing", "schema_version": 2, "billing_scope": "token", "billing_mode": "multi_group", "observed_at": stamp, "groups": []any{entry}})
	require.NoError(t, err)
	got, err := parseMultiGroupUpstreamBillingResponse(body)
	require.NoError(t, err)
	require.Equal(t, 2, got["schema_version"])
	require.Equal(t, "multi_group", got["billing_mode"])
}

func TestAPIKeyConfiguredGroupIDsKeepsLegacyBinding(t *testing.T) {
	id := int64(7)
	key := &APIKey{GroupID: &id}
	require.Equal(t, []int64{7}, key.ConfiguredGroupIDs())
	key.GroupIDs = []int64{7, 8}
	require.Equal(t, []int64{7, 8}, key.ConfiguredGroupIDs())
}
