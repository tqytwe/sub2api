//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIResponsesFinalAdmissionRefreshesCredentials(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(map[bool]string{false: "first_send", true: "rejected_field_retry"}[retry], func(t *testing.T) {
			selected := turnAdmissionAccount()
			repo := &turnAdmissionRepo{account: selected}
			rotateAt := 2
			if retry {
				rotateAt = 3
			}
			repo.afterRead = func(n int, a *Account) {
				if n == rotateAt {
					next := *a
					next.Credentials = maps.Clone(a.Credentials)
					next.Credentials["api_key"] = "rotated-synthetic"
					repo.account = &next
				}
			}
			upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
			var rejectedBody *passthroughCloseTrackingReadCloser
			if retry {
				rejectedBody = &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: 'truncation'.","param":"truncation"}}`)}
				upstream.responses = []*http.Response{{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: rejectedBody}, passthroughAdmissionResponse()}
			}
			svc := newTurnAdmissionGateway(repo, false)
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false,"truncation":"auto"}`)
			ctx, pricingAt := WithGatewayTokenRequestPricing(context.Background())
			c := adaptiveProtocolTestContext("/v1/responses", body)
			c.Request = c.Request.WithContext(ctx)
			result, err := svc.Forward(ctx, c, selected, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, upstream.requests, rotateAt-1)
			require.Equal(t, rotateAt, repo.reads)
			require.Equal(t, "Bearer rotated-synthetic", upstream.lastReq.Header.Get("Authorization"))
			if retry {
				require.Equal(t, "Bearer sk-test", upstream.requests[0].Header.Get("Authorization"))
				require.True(t, rejectedBody.closed)
				require.False(t, gjson.GetBytes(upstream.lastBody, "truncation").Exists())
			}
			require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, "gpt-5.4", result.Model)
			require.Equal(t, 3, result.Usage.InputTokens)
			require.Equal(t, 2, result.Usage.OutputTokens)
			require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
		})
	}
}

func TestOpenAIResponsesFinalAdmissionOverridesStaleOAuthTokenCache(t *testing.T) {
	for _, tc := range []struct {
		name        string
		passthrough bool
		shadow      bool
	}{{name: "native"}, {name: "passthrough", passthrough: true}, {name: "native_shadow", shadow: true}, {name: "passthrough_shadow", passthrough: true, shadow: true}} {
		t.Run(tc.name, func(t *testing.T) {
			selected := turnAdmissionAccount()
			selected.Type = AccountTypeOAuth
			selected.Credentials = map[string]any{"access_token": "initial-synthetic", "chatgpt_account_id": "synthetic-account", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)}
			selected.Extra = map[string]any{"openai_passthrough": tc.passthrough}
			repo := &turnAdmissionRepo{account: selected}
			cacheAccount := selected
			if tc.shadow {
				parent := *selected
				parent.ID = 202
				selected.ParentAccountID = &parent.ID
				repo.parent = &parent
				cacheAccount = &parent
			}
			repo.afterRead = func(n int, a *Account) {
				if n == 2 {
					if tc.shadow {
						a = repo.parent
					}
					next := *a
					next.Credentials = maps.Clone(a.Credentials)
					next.Credentials["access_token"] = "rotated-synthetic"
					if tc.shadow {
						repo.parent = &next
					} else {
						repo.account = &next
					}
				}
			}
			svc := newTurnAdmissionGateway(repo, false)
			cache := newOpenAITokenCacheStub()
			cache.tokens[OpenAITokenCacheKey(cacheAccount)] = "stale-cache-synthetic"
			svc.openAITokenProvider = NewOpenAITokenProvider(repo, cache, nil)
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: " + `{"type":"response.completed","response":{"id":"resp_done","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2}}}` + "\n\n"))}}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","instructions":"hello","input":"hello","stream":true}`)
			_, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, 2, repo.reads)
			require.Equal(t, "Bearer rotated-synthetic", upstream.lastReq.Header.Get("Authorization"))
		})
	}
}

func TestOpenAIResponsesFinalAdmissionScopesRotatedSetupTokenIdentity(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "passthrough"}[passthrough], func(t *testing.T) {
			selected := turnAdmissionAccount()
			selected.Type = AccountTypeSetupToken
			selected.Credentials = map[string]any{"access_token": "initial-synthetic"}
			selected.Extra = map[string]any{"openai_passthrough": passthrough}
			repo := &turnAdmissionRepo{account: selected}
			repo.afterRead = func(n int, a *Account) {
				if n == 2 {
					next := *a
					next.Credentials = maps.Clone(a.Credentials)
					next.Credentials["access_token"] = "rotated-synthetic"
					repo.account = &next
				}
			}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: " + `{"type":"response.completed","response":{"id":"resp_done","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2}}}` + "\n\n"))}}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","instructions":"hello","input":"hello","stream":true,"client_metadata":{"installation_id":"client-installation"}}`)
			c := adaptiveProtocolTestContext("/v1/responses", body)
			c.Request.Header.Set("x-codex-installation-id", "client-installation")
			_, err := svc.Forward(context.Background(), c, selected, body)
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, "Bearer rotated-synthetic", upstream.lastReq.Header.Get("Authorization"))
			wantIdentity := scopeCodexAccountIdentityValue(repo.account, 0, "installation", "client-installation")
			require.Equal(t, wantIdentity, gjson.GetBytes(upstream.lastBody, "client_metadata.installation_id").String())
			require.Equal(t, wantIdentity, upstream.lastReq.Header.Get("x-codex-installation-id"))
		})
	}
}

func TestOpenAIResponsesFinalAdmissionRejectsMutableRouteRetry(t *testing.T) {
	selected := turnAdmissionAccount()
	repo := &turnAdmissionRepo{account: selected}
	firstBody := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: 'truncation'.","param":"truncation"}}`)}
	upstream := &passthroughAdmissionUpstream{
		httpUpstreamRecorder: &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: firstBody}, passthroughAdmissionResponse()}},
		afterFirst:           func() { selected.Credentials["base_url"] = "http://changed.example" },
	}
	svc := newTurnAdmissionGateway(repo, false)
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false,"truncation":"auto"}`)
	result, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
	require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
	require.Nil(t, result)
	require.Len(t, upstream.requests, 1)
	require.True(t, firstBody.closed)
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover))
}

func TestOpenAIResponsesFinalAdmissionPreservesFingerprintConvergence(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, mode := range []string{"device", "session", "full"} {
			t.Run(map[bool]string{false: "native", true: "passthrough"}[passthrough]+"/"+mode, func(t *testing.T) {
				selected := turnAdmissionAccount()
				selected.Type = AccountTypeOAuth
				selected.Credentials = map[string]any{"access_token": "synthetic-fingerprint-token", "chatgpt_account_id": "synthetic-fingerprint-account"}
				selected.Extra = map[string]any{"openai_passthrough": passthrough, codexFingerprintModeExtraKey: mode, codexFingerprintSeedExtraKey: testCodexFingerprintSeed}
				svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
				upstream := &httpUpstreamRecorder{responses: []*http.Response{
					{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: 'input[0].status'.","param":"input[0].status"}}`))},
					{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: " + `{"type":"response.completed","response":{"id":"resp_done","status":"completed","usage":{"input_tokens":3,"output_tokens":2}}}` + "\n\n"))},
				}}
				svc.httpUpstream = upstream
				body := []byte(`{"model":"gpt-5.4","instructions":"hi","input":[{"type":"message","role":"user","content":"hi","status":"completed"}],"stream":true,"prompt_cache_key":"client-session","client_metadata":{"x-codex-installation-id":"client-installation","session_id":"client-session","thread_id":"client-thread","turn_id":"client-turn","x-codex-window-id":"client-window"}}`)
				c := adaptiveProtocolTestContext("/v1/responses", body)
				c.Request.Header.Set("x-codex-installation-id", "client-installation")
				c.Request.Header.Set("session_id", "client-session")
				// Embedded turn metadata is rewritten only when supplied,
				// matching the deployed fingerprint header contract.
				c.Request.Header.Set("x-codex-turn-metadata", `{"installation_id":"client-installation","session_id":"client-session","thread_id":"client-thread","turn_id":"client-turn","window_id":"client-window"}`)
				_, err := svc.Forward(context.Background(), c, selected, body)
				require.NoError(t, err)
				require.Len(t, upstream.requests, 2)
				for i, req := range upstream.requests {
					require.Equal(t, req.Header.Get("x-codex-installation-id"), gjson.GetBytes(upstream.bodies[i], "client_metadata.x-codex-installation-id").String(), "attempt %d body and header installation", i)
					require.Equal(t, upstream.requests[0].Header.Get("x-codex-installation-id"), req.Header.Get("x-codex-installation-id"))
					if mode != "device" {
						require.NotEmpty(t, req.Header.Get("x-codex-turn-metadata"))
						require.Equal(t, req.Header.Get("session_id"), gjson.GetBytes(upstream.bodies[i], "client_metadata.session_id").String())
						require.Equal(t, req.Header.Get("thread-id"), gjson.GetBytes(upstream.bodies[i], "client_metadata.thread_id").String())
						require.Equal(t, gjson.Get(req.Header.Get("x-codex-turn-metadata"), "turn_id").String(), gjson.GetBytes(upstream.bodies[i], "client_metadata.turn_id").String())
						require.Equal(t, upstream.requests[0].Header.Get("x-codex-turn-metadata"), req.Header.Get("x-codex-turn-metadata"), "retry must reuse random turn IDs")
					}
				}
			})
		}
	}
}
