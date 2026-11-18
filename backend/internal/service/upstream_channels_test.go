package service

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestUpstreamChannelEntityValidation(t *testing.T) {
	m := UpstreamChannelMutation{Confirm: true, Code: "relay-a", Name: "  上游A  ", Remark: " 说明 "}
	require.NoError(t, ValidateUpstreamChannelMutation(&m, true))
	require.Equal(t, "上游A", m.Name)
	for _, code := range []string{"", "a/b", "https://host", "备注", strings.Repeat("x", 65)} {
		m.Code = code
		require.Error(t, ValidateUpstreamChannelMutation(&m, true))
	}
	m = UpstreamChannelMutation{Name: "上游", Remark: strings.Repeat("😀", 500), Revision: new(int64(1)), Confirm: true}
	require.NoError(t, ValidateUpstreamChannelMutation(&m, false))
	m.Remark += "中"
	require.Error(t, ValidateUpstreamChannelMutation(&m, false))
	require.NoError(t, ValidateUpstreamChannelBindings(UpstreamChannelBindingsMutation{ChannelID: 5, Confirm: true, Bindings: []UpstreamChannelBinding{{AccountID: 1, Revision: 0}}}), "no URL or account platform is required")
	require.NoError(t, ValidateUpstreamChannelBindings(UpstreamChannelBindingsMutation{ChannelID: 0, Confirm: true, Bindings: []UpstreamChannelBinding{{AccountID: 1, ChannelID: 5, Revision: 1}}}))
	for _, m := range []UpstreamChannelBindingsMutation{{Confirm: false, Bindings: []UpstreamChannelBinding{{AccountID: 1}}}, {Confirm: true, Bindings: make([]UpstreamChannelBinding, 101)}, {Confirm: true, Bindings: []UpstreamChannelBinding{{AccountID: 1}, {AccountID: 1}}}} {
		require.Error(t, ValidateUpstreamChannelBindings(m))
	}
}
