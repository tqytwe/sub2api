//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesToolsCapabilityForwardDoesNotReplayOrChangeBilling(t *testing.T) {
	for _, mode := range []string{"chat", "legacy_chat", "responses", "changed_before_send"} {
		t.Run(mode, func(t *testing.T) {
			selected := turnAdmissionAccount()
			selected.Credentials["model_mapping"] = map[string]any{"public-alias": "gpt-6.1-sol"}
			selected.Extra = map[string]any{"openai_responses_mode": "force_responses"}
			if mode == "chat" || mode == "legacy_chat" {
				selected.Extra["openai_responses_mode"] = "force_chat_completions"
			}
			repo := &turnAdmissionRepo{account: selected}
			if mode == "changed_before_send" {
				repo.afterRead = func(n int, a *Account) {
					if n == 2 {
						latest := *a
						latest.Extra = map[string]any{"openai_responses_mode": "force_chat_completions"}
						repo.account = &latest
					}
				}
			}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"public-alias","input":"hello","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],"stream":false}`)
			if mode == "legacy_chat" {
				body = []byte(`{"model":"public-alias","messages":[{"role":"user","content":"hello"}],"functions":[{"name":"lookup","parameters":{"type":"object"}}],"stream":false}`)
			}
			ctx, pricingAt := WithGatewayTokenRequestPricing(context.Background())
			c := adaptiveProtocolTestContext("/v1/responses", body)
			c.Request = c.Request.WithContext(ctx)
			result, err := svc.Forward(ctx, c, selected, body)
			if mode != "responses" {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				var failover *UpstreamFailoverError
				require.False(t, errors.As(err, &failover))
				require.Nil(t, result)
				require.Empty(t, upstream.requests)
				require.False(t, c.Writer.Written())
				return
			}
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, "lookup", gjson.GetBytes(upstream.lastBody, "tools.0.name").String())
			require.Equal(t, "public-alias", result.Model)
			require.Equal(t, "gpt-6.1-sol", result.UpstreamModel)
			require.Equal(t, 3, result.Usage.InputTokens)
			require.Equal(t, 2, result.Usage.OutputTokens)
			require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
		})
	}
}

func TestResponsesToolsCapabilityKeepsFinalFallbackGuard(t *testing.T) {
	selected := turnAdmissionAccount()
	selected.Credentials["model_mapping"] = map[string]any{"public-alias": "gpt-6.1-sol"}
	svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
	upstream := &httpUpstreamRecorder{}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"public-alias","input":"hello","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`)
	c := adaptiveProtocolTestContext("/v1/responses", body)
	_, err := svc.forwardResponsesViaRawChatCompletions(context.Background(), c, selected, body)
	require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
	require.Equal(t, 400, c.Writer.Status())
	require.True(t, IsResponseCommitted(c))
	require.Empty(t, upstream.requests)
}
