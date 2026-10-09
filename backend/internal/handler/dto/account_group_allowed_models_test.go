package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountFromServiceExposesGroupAllowedModels(t *testing.T) {
	var account service.Account
	require.NoError(t, json.Unmarshal([]byte(`{"ID":1,"AccountGroups":[{"AccountID":1,"GroupID":3},{"AccountID":1,"GroupID":5,"AllowedModels":["gpt-5.5"]}]}`), &account))
	out := AccountFromService(&account)
	require.NotNil(t, out)
	require.Len(t, out.AccountGroups, 2)
	body, err := json.Marshal(out.AccountGroups)
	require.NoError(t, err)
	var groups []map[string]any
	require.NoError(t, json.Unmarshal(body, &groups))
	require.NotContains(t, groups[0], "allowed_models")
	require.Equal(t, []any{"gpt-5.5"}, groups[1]["allowed_models"])
}
