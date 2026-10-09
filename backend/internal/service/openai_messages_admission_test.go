//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIMessagesAdmissionCheckpoints(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, change := range []string{"initial_disabled", "final_disabled", "final_route", "rotated_secret", "canceled_drain"} {
			t.Run(fmt.Sprintf("raw=%v/%s", raw, change), func(t *testing.T) {
				a := chatAdmissionAccount(raw)
				repo := &turnAdmissionRepo{account: a}
				repo.afterRead = func(n int, current *Account) {
					if (change == "initial_disabled" && n == 1) || (n == 2 && change != "canceled_drain") {
						latest := *current
						latest.Credentials = maps.Clone(current.Credentials)
						switch change {
						case "initial_disabled", "final_disabled":
							latest.Status = "disabled"
						case "final_route":
							latest.Credentials["base_url"] = "http://changed.example"
						case "rotated_secret":
							latest.Credentials["api_key"] = "rotated-synthetic"
						}
						repo.account = &latest
					}
				}
				svc := newTurnAdmissionGateway(repo, false)
				upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(raw)}
				svc.httpUpstream = upstream
				body := []byte(`{"model":"gpt-5.4","max_tokens":64,"messages":[{"role":"user","content":"hello"}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if change == "canceled_drain" {
					cancel()
				}
				c := adaptiveProtocolTestContext("/v1/messages", body)
				result, err := svc.ForwardAsAnthropic(ctx, c, a, body, "", "")
				if strings.Contains(change, "disabled") || change == "final_route" {
					require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
					require.Nil(t, result)
					require.Empty(t, upstream.requests)
					var failover *UpstreamFailoverError
					require.False(t, errors.As(err, &failover))
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, upstream.requests, 1)
				require.Equal(t, 2, repo.reads)
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				if change == "rotated_secret" {
					require.Equal(t, "Bearer rotated-synthetic", upstream.lastReq.Header.Get("Authorization"))
				}
				require.Contains(t, string(upstream.lastBody), "lookup")
			})
		}
	}
}

func TestOpenAIMessagesAdmissionKeepsPublicAndOutboundModels(t *testing.T) {
	for _, raw := range []bool{false, true} {
		t.Run(fmt.Sprint(raw), func(t *testing.T) {
			a := chatAdmissionAccount(raw)
			a.GroupIDs = []int64{9}
			a.Groups = []*Group{{ID: 9, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-alias"}}}}
			repo := &turnAdmissionRepo{account: a}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(raw)}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","max_tokens":64,"messages":[{"role":"user","content":"hello"}]}`)
			parentBilling := &Group{ID: 20, Hydrated: true, Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 2}
			ctx, pricingAt := WithGatewayTokenRequestPricing(context.WithValue(context.Background(), ctxkey.Group, parentBilling))
			ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, "public-alias")
			c := adaptiveProtocolTestContext("/v1/messages", body)
			c.Request = c.Request.WithContext(ctx)
			groupID := int64(9)
			key := &APIKey{GroupID: &groupID, Group: &Group{ID: 9, RateMultiplier: 3}}
			c.Set("api_key", key)
			result, err := svc.ForwardAsAnthropic(ctx, c, a, body, "", "")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, 2, repo.reads)
			require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Same(t, key, getAPIKeyFromContext(c))
			require.Same(t, parentBilling, gatewayTokenRequestBillingGroupFromContext(c.Request.Context()))
			require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
		})
	}
}

func TestOpenAIChatAdmissionRotatedSetupTokenScopesIdentityOnce(t *testing.T) {
	a := chatAdmissionAccount(false)
	a.Type = AccountTypeSetupToken
	a.Credentials = map[string]any{"access_token": "old-synthetic"}
	repo := &turnAdmissionRepo{account: a}
	repo.afterRead = func(n int, current *Account) {
		if n == 2 {
			latest := *current
			latest.Credentials = map[string]any{"access_token": "new-synthetic"}
			repo.account = &latest
		}
	}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(false)}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","input":"hello","prompt_cache_key":"cache-seed","client_metadata":{"session_id":"session-seed"}}`)
	result, err := svc.ForwardAsChatCompletions(context.Background(), adaptiveProtocolTestContext("/v1/chat/completions", body), a, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Bearer new-synthetic", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, scopeCodexAccountIdentityValue(repo.account, 0, "session", "session-seed"), gjson.GetBytes(upstream.lastBody, "client_metadata.session_id").String())
	require.Equal(t, scopeCodexAccountIdentityValue(repo.account, 0, "prompt-cache", "cache-seed"), gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
}

func TestOpenAICompatProgressiveAggregateSurvivesDetailOnlyTerminal(t *testing.T) {
	for _, messages := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			for _, topLevel := range []bool{false, true} {
				t.Run(fmt.Sprintf("messages=%v/stream=%v/top_level=%v", messages, stream, topLevel), func(t *testing.T) {
					selected := chatAdmissionAccount(false)
					svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
					terminal := `{"type":"response.failed","response":{"id":"resp_progress","status":"failed","error":{"code":"rate_limit_exceeded","message":"rate limit exceeded"},"usage":{"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":3,"image_tokens":5}}}}`
					if topLevel {
						terminal = `{"type":"response.failed","response":{"id":"resp_progress","status":"failed","error":{"code":"rate_limit_exceeded","message":"rate limit exceeded"}},"usage":{"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":3,"image_tokens":5}}}`
					}
					payload := `data: {"type":"response.in_progress","response":{"id":"resp_progress","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}` + "\n\n" + "data: " + terminal + "\n\n"
					upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))}}
					svc.httpUpstream = upstream
					body := []byte(fmt.Sprintf(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":%v}`, stream))
					var result *OpenAIForwardResult
					var err error
					if messages {
						result, err = svc.ForwardAsAnthropic(context.Background(), adaptiveProtocolTestContext("/v1/messages", body), selected, body, "", "")
					} else {
						result, err = svc.ForwardAsChatCompletions(context.Background(), adaptiveProtocolTestContext("/v1/chat/completions", body), selected, body, "", "")
					}
					require.Error(t, err)
					require.NotNil(t, result, "a detail-only failed terminal must not erase aggregate expense")
					require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 2, CacheReadInputTokens: 4}, result.Usage)
					require.Len(t, upstream.requests, 1)
				})
			}
		}
	}
}
