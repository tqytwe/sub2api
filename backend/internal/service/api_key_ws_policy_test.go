package service

import (
	"context"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Exercise real client WS frames through the passthrough ingress: the second
// turn must not restore the client's model or effort after the initial request.
func TestAPIKeyWSPolicyAppliesToFirstAndSubsequentFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(context.Canceled)
	key := &APIKey{ModelMapping: map[string]string{"gpt-6.1-sol": "gpt-5.6-sol"}, ReasoningEffort: "high"}
	upstream := newStagedPassthroughConn()
	hooks := &OpenAIWSIngressHooks{
		ForcedReasoningEffort: key.ReasoningEffort,
		APIKeyModelMapping:    true,
		MaxReasoningEffort:    "medium",
		MapRequestModel:       func(_ int, model string) (string, error) { return MapAPIKeyModel(key, model), nil },
	}
	server, _ := startPassthroughHookRecordingServer(t, ctx, newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount(), hooks)
	defer server.Close()
	client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-6.1-sol","stream":false}`)
	defer func() { _ = client.CloseNow() }()

	first := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
	require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(first, "model").String())
	require.Equal(t, "medium", gjson.GetBytes(first, "reasoning.effort").String())
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_key_1","model":"gpt-5.6-sol","usage":{"input_tokens":1,"output_tokens":1}}}`)
	_, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
	require.NoError(t, err)

	writeCtx, cancelWrite := context.WithTimeout(ctx, 3*time.Second)
	err = client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-6.1-sol","Model":"gpt-6.1-sol","reasoning":{"effort":"ultra"}}`))
	cancelWrite()
	require.NoError(t, err)
	second := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
	require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(second, "model").String())
	require.Equal(t, "medium", gjson.GetBytes(second, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(second, "Model").Exists())
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_key_2","model":"gpt-5.6-sol","usage":{"input_tokens":1,"output_tokens":1}}}`)
	_, err = readPassthroughLifecycleFrame(t, client, 3*time.Second)
	require.NoError(t, err)
}

func TestAPIKeyWSPolicyStateMutationsRespectContextRestriction(t *testing.T) {
	large := strings.Repeat("context ", 300)
	for _, tc := range []struct {
		name, payload string
		key           *APIKey
		blocked       bool
	}{
		{"restricted session update", `{"type":"session.update","session":{"instructions":"small"}}`, &APIKey{AllowLongContext: false}, true},
		{"explicit ceiling session update", `{"type":"session.update","session":{"instructions":"small"}}`, &APIKey{AllowLongContext: true, ContextTokenLimit: 100}, true},
		{"restricted conversation create", `{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + large + `"}]}}`, &APIKey{AllowLongContext: true, ContextTokenLimit: 100}, true},
		{"restricted conversation mutation", `{"type":"conversation.item.update","item":{"content":"` + large + `"}}`, &APIKey{AllowLongContext: true, ContextTokenLimit: 100}, true},
		// Even individually small appends can accumulate invisible server-side
		// history; the first append must be denied rather than starting a counter.
		{"restricted small conversation create", `{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}`, &APIKey{AllowLongContext: true, ContextTokenLimit: 100}, true},
		{"legacy restricted small conversation create", `{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}`, &APIKey{AllowLongContext: false}, true},
		{"restricted small conversation mutation", `{"type":"conversation.item.update","item":{"content":"hi"}}`, &APIKey{AllowLongContext: true, ContextTokenLimit: 100}, true},
		{"unrestricted session update", `{"type":"session.update","session":{"instructions":"small"}}`, &APIKey{AllowLongContext: true}, false},
		{"unrestricted conversation create", `{"type":"conversation.item.create","item":{"content":"` + large + `"}}`, &APIKey{AllowLongContext: true}, false},
		{"restricted cancel", `{"type":"response.cancel","response_id":"resp_active"}`, &APIKey{AllowLongContext: true, ContextTokenLimit: 100}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(context.Canceled)
			upstream := newStagedPassthroughConn()
			svc := newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream)
			server, serverErr := startPassthroughHookRecordingServer(t, ctx, svc, passthroughLifecycleAccount(), nil, tc.key)
			defer server.Close()
			client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-5.4","input":"hi"}`)
			defer func() { _ = client.CloseNow() }()
			requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
			if gjson.Get(tc.payload, "type").String() == "response.cancel" {
				upstream.Send(`{"type":"response.created","response":{"id":"resp_context_policy","model":"gpt-5.4"}}`)
			} else {
				upstream.Send(`{"type":"response.completed","response":{"id":"resp_context_policy","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":1}}}`)
			}
			_, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
			require.NoError(t, err)
			writeCtx, cancelWrite := context.WithTimeout(ctx, 3*time.Second)
			err = client.Write(writeCtx, coderws.MessageText, []byte(tc.payload))
			cancelWrite()
			require.NoError(t, err)
			if !tc.blocked {
				require.JSONEq(t, tc.payload, string(requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)))
				return
			}
			type clientRead struct {
				payload []byte
				err     error
			}
			clientResult := make(chan clientRead, 1)
			go func() {
				payload, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
				clientResult <- clientRead{payload: payload, err: err}
			}()
			select {
			case result := <-clientResult:
				require.NoError(t, result.err)
				require.Equal(t, "response.failed", gjson.GetBytes(result.payload, "type").String())
				require.Equal(t, APIKeyContextLengthExceededCode, gjson.GetBytes(result.payload, "response.error.code").String())
			case sent := <-upstream.writes:
				t.Fatalf("restricted mutation reached upstream: %s", sent)
			case <-time.After(3 * time.Second):
				t.Fatal("restricted mutation did not produce a public failure event")
			}
			_ = client.CloseNow()
			select {
			case err := <-serverErr:
				var policyErr *APIKeyLongContextError
				require.ErrorAs(t, err, &policyErr)
			case sent := <-upstream.writes:
				t.Fatalf("restricted mutation reached upstream: %s", sent)
			case <-time.After(3 * time.Second):
				t.Fatal("restricted mutation was not rejected")
			}
			select {
			case sent := <-upstream.writes:
				t.Fatalf("restricted mutation reached upstream: %s", sent)
			default:
			}
		})
	}
}
