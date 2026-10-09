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
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func chatAdmissionResponse(raw bool) *http.Response {
	body := `data: {"type":"response.completed","response":{"id":"resp_admitted","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}` + "\n\ndata: [DONE]\n\n"
	contentType := "text/event-stream"
	if raw {
		body = `{"id":"chat_admitted","object":"chat.completion","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
		contentType = "application/json"
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}
func chatAdmissionAccount(raw bool) *Account {
	a := turnAdmissionAccount()
	if raw {
		a.Extra = forceChatResponsesFallbackAccount().Extra
	}
	return a
}
func TestOpenAIChatAdmissionCheckpoints(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, change := range []string{"initial_disabled", "final_disabled", "final_route", "rotated_secret", "canceled_drain"} {
			t.Run(map[bool]string{false: "responses", true: "raw"}[raw]+"/"+change, func(t *testing.T) {
				a := chatAdmissionAccount(raw)
				repo := &turnAdmissionRepo{account: a}
				repo.afterRead = func(n int, current *Account) {
					if (change == "initial_disabled" && n == 1) || (n == 2 && change != "canceled_drain") {
						next := *current
						next.Credentials = maps.Clone(current.Credentials)
						switch change {
						case "initial_disabled", "final_disabled":
							next.Status = "disabled"
						case "final_route":
							next.Credentials["base_url"] = "http://changed.example"
						case "rotated_secret":
							next.Credentials["api_key"] = "rotated-synthetic"
						}
						repo.account = &next
					}
				}
				svc := newTurnAdmissionGateway(repo, false)
				upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(raw)}
				svc.httpUpstream = upstream
				body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"reasoning_effort":"low"}`)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if change == "canceled_drain" {
					cancel()
				}
				c := adaptiveProtocolTestContext("/v1/chat/completions", body)
				result, err := svc.ForwardAsChatCompletions(ctx, c, a, body, "", "")
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

type blockingChatAdmissionRepo struct {
	AccountRepository
	deadline bool
}

func (r *blockingChatAdmissionRepo) GetOpenAITurnAdmission(ctx context.Context, _ int64) (*Account, *Account, error) {
	_, r.deadline = ctx.Deadline()
	<-ctx.Done()
	return nil, nil, ctx.Err()
}
func TestOpenAIChatAdmissionDetachedReadIsBounded(t *testing.T) {
	repo := &blockingChatAdmissionRepo{}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{err: errors.New("must not send")}
	svc.httpUpstream = upstream
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`)
	start := time.Now()
	_, err := svc.ForwardAsChatCompletions(ctx, adaptiveProtocolTestContext("/v1/chat/completions", body), chatAdmissionAccount(false), body, "", "")
	require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
	require.True(t, repo.deadline)
	require.Less(t, time.Since(start), 5*time.Second)
	require.Empty(t, upstream.requests)
}

func TestOpenAISharedCCAdmissionRefreshesBeforeHeaders(t *testing.T) {
	for _, deny := range []bool{false, true} {
		t.Run(map[bool]string{false: "rotate", true: "deny"}[deny], func(t *testing.T) {
			a := chatAdmissionAccount(true)
			fresh := *a
			fresh.Credentials = maps.Clone(a.Credentials)
			fresh.Credentials["api_key"] = "latest-synthetic"
			if deny {
				fresh.Status = "disabled"
			}
			repo := &turnAdmissionRepo{account: &fresh}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(true)}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`)
			resp, err := svc.sendCCUpstreamRequest(context.Background(), adaptiveProtocolTestContext("/v1/messages", body), a, "http://upstream.example/v1/chat/completions", body, false, "stale-synthetic", "", "", "gpt-5.4")
			if deny {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				require.Nil(t, resp)
				require.Empty(t, upstream.requests)
				return
			}
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, "Bearer latest-synthetic", upstream.lastReq.Header.Get("Authorization"))
			require.Equal(t, 1, repo.reads)
		})
	}
}

func TestOpenAIChatAdmissionPreservesPublicModelAndPricing(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, channelMapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("raw=%v/channel=%v", raw, channelMapped), func(t *testing.T) {
				a := chatAdmissionAccount(raw)
				a.Credentials["model_mapping"] = map[string]any{"public-alias": "gpt-5.4"}
				a.GroupIDs = []int64{9}
				a.Groups = []*Group{{ID: 9, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-alias"}}}}
				repo := &turnAdmissionRepo{account: a}
				svc := newTurnAdmissionGateway(repo, false)
				upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(raw)}
				svc.httpUpstream = upstream
				model := "public-alias"
				parentBilling := &Group{ID: 20, Hydrated: true, Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 2}
				ctx, pricingAt := WithGatewayTokenRequestPricing(context.WithValue(context.Background(), ctxkey.Group, parentBilling))
				if channelMapped {
					model = "gpt-5.4"
					ctx = context.WithValue(ctx, ctxkey.Model, "channel-alias")
					ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, "public-alias")
				}
				body := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello"}]}`, model))
				c := adaptiveProtocolTestContext("/v1/chat/completions", body)
				c.Request = c.Request.WithContext(ctx)
				groupID := int64(9)
				authGroup := &Group{ID: 9, RateMultiplier: 3}
				key := &APIKey{GroupID: &groupID, Group: authGroup}
				c.Set("api_key", key)
				result, err := svc.ForwardAsChatCompletions(ctx, c, a, body, "", "")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 2, repo.reads)
				require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
				require.Same(t, key, getAPIKeyFromContext(c))
				require.Same(t, authGroup, key.Group)
				require.Same(t, parentBilling, gatewayTokenRequestBillingGroupFromContext(c.Request.Context()))
				require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
			})
		}
	}
}

func TestOpenAIChatAdmissionChecksMappedModelAtFinalSend(t *testing.T) {
	for _, raw := range []bool{false, true} {
		t.Run(fmt.Sprint(raw), func(t *testing.T) {
			a := chatAdmissionAccount(raw)
			a.Credentials["model_mapping"] = map[string]any{"public-alias": "gpt-5.4"}
			repo := &turnAdmissionRepo{account: a}
			repo.afterRead = func(n int, current *Account) {
				if n == 2 {
					latest := *current
					latest.Extra = maps.Clone(current.Extra)
					if latest.Extra == nil {
						latest.Extra = make(map[string]any)
					}
					latest.Extra[modelRateLimitsKey] = map[string]any{"gpt-5.4": map[string]any{"rate_limit_reset_at": time.Now().Add(time.Minute).Format(time.RFC3339)}}
					repo.account = &latest
				}
			}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(raw)}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"public-alias","messages":[{"role":"user","content":"hello"}]}`)
			c := adaptiveProtocolTestContext("/v1/chat/completions", body)
			result, err := svc.ForwardAsChatCompletions(context.Background(), c, a, body, "", "")
			require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
			require.Nil(t, result)
			require.Empty(t, upstream.requests)
			require.False(t, c.Writer.Written(), "local admission must not start or misclassify the upstream response")
		})
	}
}

type chatAdmissionParentRepo struct {
	turnAdmissionRepo
	credentialReads int
}

func (r *chatAdmissionParentRepo) GetByID(context.Context, int64) (*Account, error) {
	r.credentialReads++
	return nil, errors.New("must use admitted parent snapshot")
}

func TestOpenAIChatAdmissionUsesAuthoritativeCredentialParent(t *testing.T) {
	parent := &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Credentials: map[string]any{"access_token": "initial-synthetic", "chatgpt_account_id": "synthetic-parent"}}
	a := chatAdmissionAccount(false)
	a.Type = AccountTypeOAuth
	a.ParentAccountID = &parent.ID
	a.Credentials = map[string]any{}
	repo := &chatAdmissionParentRepo{turnAdmissionRepo: turnAdmissionRepo{account: a, parent: parent}}
	repo.afterRead = func(n int, _ *Account) {
		if n == 2 {
			latest := *repo.parent
			latest.Credentials = maps.Clone(latest.Credentials)
			latest.Credentials["access_token"] = "rotated-synthetic"
			repo.parent = &latest
		}
	}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(false)}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`)
	result, err := svc.ForwardAsChatCompletions(context.Background(), adaptiveProtocolTestContext("/v1/chat/completions", body), a, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Zero(t, repo.credentialReads)
	require.Equal(t, 2, repo.reads)
	require.Equal(t, "Bearer rotated-synthetic", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "synthetic-parent", upstream.lastReq.Header.Get("Chatgpt-Account-Id"))
}

func TestOpenAIChatAdmissionRejectsCredentialParentBindingChange(t *testing.T) {
	parent := &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Credentials: map[string]any{"access_token": "initial-synthetic", "chatgpt_account_id": "synthetic-parent"}}
	a := chatAdmissionAccount(false)
	a.Type = AccountTypeOAuth
	a.ParentAccountID = &parent.ID
	a.Credentials = map[string]any{}
	repo := &chatAdmissionParentRepo{turnAdmissionRepo: turnAdmissionRepo{account: a, parent: parent}}
	repo.afterRead = func(n int, _ *Account) {
		if n == 2 {
			latest := *repo.parent
			latest.Credentials = maps.Clone(latest.Credentials)
			latest.Credentials["chatgpt_account_id"] = "different-parent"
			repo.parent = &latest
		}
	}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{resp: chatAdmissionResponse(false)}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`)
	result, err := svc.ForwardAsChatCompletions(context.Background(), adaptiveProtocolTestContext("/v1/chat/completions", body), a, body, "", "")
	require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
	require.Nil(t, result)
	require.Empty(t, upstream.requests)
}

func TestOpenAIChatAdmissionCredentialSnapshotDoesNotAliasRouteState(t *testing.T) {
	for _, mutate := range []struct {
		name  string
		apply func(*Account)
	}{
		{"proxy", func(a *Account) { a.Proxy.Host = "changed.example" }},
		{"nested_mapping", func(a *Account) { a.Credentials["model_mapping"].(map[string]any)["alias"] = "changed-model" }},
		{"typed_mapping", func(a *Account) { a.Credentials["typed_mapping"].(map[string]string)["alias"] = "changed-model" }},
		{"typed_slice", func(a *Account) { a.Credentials["typed_slice"].([]string)[0] = "changed-model" }},
		{"extra", func(a *Account) { a.Extra["openai_responses_mode"] = "force_chat_completions" }},
		{"parent_id", func(a *Account) { *a.ParentAccountID = 909 }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			parent := admittedOAuthTokenTestAccount()
			proxyID := int64(88)
			selected := &Account{
				ID: 902, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parent.ID,
				ProxyID: &proxyID, Proxy: &Proxy{ID: proxyID, Protocol: "http", Host: "original.example", Port: 8080},
				Credentials: map[string]any{
					"model_mapping": map[string]any{"alias": "original-model"},
					"typed_mapping": map[string]string{"alias": "original-model"},
					"typed_slice":   []string{"original-model"}, "numeric": int64(9007199254740993),
				},
				Extra: map[string]any{"openai_responses_mode": "force_responses"},
			}
			admitted := bindOpenAITurnCredentialParent(selected, parent)
			before := openAITurnRouteFingerprint(admitted)
			require.IsType(t, map[string]string{}, admitted.Credentials["typed_mapping"])
			require.IsType(t, []string{}, admitted.Credentials["typed_slice"])
			require.Equal(t, int64(9007199254740993), admitted.Credentials["numeric"])
			mutate.apply(selected)
			require.Equal(t, before, openAITurnRouteFingerprint(admitted), "admitted route must stay pinned when repository objects change in place")
			require.Equal(t, int64(901), admitted.openAITurnCredentialParent.ID)
		})
	}
	parent := admittedOAuthTokenTestAccount()
	parent.Proxy = &Proxy{Protocol: "http", Host: "original.example", Port: 8080}
	parent.Extra = map[string]any{codexFingerprintSeedExtraKey: "original-seed"}
	selected := &Account{ID: 902, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parent.ID}
	admitted := bindOpenAITurnCredentialParent(selected, parent)
	parent.Proxy.Host = "changed.example"
	parent.Extra[codexFingerprintSeedExtraKey] = "changed-seed"
	require.True(t, IsOpenAITurnAdmissionError(validateOpenAITurnCredentialParentBinding(admitted, parent)), "in-place mutation of the credential parent must be detected")
}
