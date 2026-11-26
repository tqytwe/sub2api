package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestOpenAI409OmissionPreservesRequestControlsAtSend(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]byte) ([]byte, error)
		refuse bool
	}{
		{name: "unchanged", mutate: func(b []byte) ([]byte, error) { return b, nil }},
		{name: "reordered schema", mutate: func(b []byte) ([]byte, error) {
			return sjson.SetRawBytes(b, "tools.0.parameters", []byte(`{"z":9007199254740993,"a":"keep"}`))
		}},
		{name: "tools precision", mutate: func(b []byte) ([]byte, error) {
			return sjson.SetRawBytes(b, "tools.0.parameters.z", []byte(`9007199254740992`))
		}, refuse: true},
		{name: "tool choice", mutate: func(b []byte) ([]byte, error) { return sjson.SetBytes(b, "tool_choice", "required") }, refuse: true},
		{name: "parallel tool calls", mutate: func(b []byte) ([]byte, error) { return sjson.SetBytes(b, "parallel_tool_calls", true) }, refuse: true},
		{name: "instructions", mutate: func(b []byte) ([]byte, error) { return sjson.DeleteBytes(b, "instructions") }, refuse: true},
		{name: "previous response", mutate: func(b []byte) ([]byte, error) { return sjson.SetBytes(b, "previous_response_id", "resp_unrelated") }, refuse: true},
		{name: "conversation", mutate: func(b []byte) ([]byte, error) { return sjson.SetBytes(b, "conversation.id", "conv_unrelated") }, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{cfg: &config.Config{}, settingService: NewSettingService(&codexPolicyMigrationRepoStub{values: map[string]string{SettingKeyOpenAI409OmitEncryptedHistoryEnabled: "true"}}, nil)}
			issue715LearnRoute(t, svc, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, issue715OpaqueBody)
			raw := []byte(strings.TrimSuffix(issue715OpaqueBody, "]}") + `,{"type":"message","role":"user","content":"continue"}],"tools":[{"type":"function","name":"record","parameters":{"a":"keep","z":9007199254740993}}],"tool_choice":"auto","parallel_tool_calls":false,"instructions":"retain these instructions"}`)
			c, _ := newTurnStateTestContext(t, 715, "fixture-session")
			c.Set("api_key", &APIKey{ID: 715, GroupID: int64Ptr(715)})
			ctx := svc.WithOpenAIAgentMessageRecovery(context.Background(), c, raw, "fixture-session", "gpt-6-astra")
			ctx, clean, changed := svc.TryOmitOpenAI409History(ctx, c, raw, "fixture-session", "gpt-6-astra", openAIAgentRecoveryError("owner_unavailable"))
			require.True(t, changed)
			wire, err := tc.mutate(clean)
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, "https://native.example/v1/responses", bytes.NewReader(wire))
			require.NoError(t, err)
			finish, err := svc.prepareOpenAIAgentRecoveryAttempt(ctx, &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, req)
			defer finish(false)
			if tc.refuse {
				var refusal *OpenAIAgentRecoveryError
				require.ErrorAs(t, err, &refusal)
				require.Equal(t, "wire_changed", refusal.Reason)
				diag, err := json.Marshal(refusal.Controls)
				require.NoError(t, err)
				require.NotNil(t, refusal.Controls)
				require.NotContains(t, string(diag), "retain these instructions")
				require.NotContains(t, string(diag), "resp_unrelated")
				require.NotContains(t, string(diag), "9007199254740993")
				require.NotContains(t, string(diag), "null")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
