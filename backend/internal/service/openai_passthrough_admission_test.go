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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func passthroughAdmissionAccount() *Account {
	a := turnAdmissionAccount()
	a.Extra = map[string]any{"openai_passthrough": true}
	return a
}
func passthroughAdmissionResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"resp_admitted","object":"response","status":"completed","model":"gpt-5.4","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`))}
}

type passthroughAdmissionUpstream struct {
	*httpUpstreamRecorder
	afterFirst func()
}

func (u *passthroughAdmissionUpstream) Do(req *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
	response, err := u.httpUpstreamRecorder.Do(req, proxy, id, concurrency)
	if len(u.requests) == 1 && u.afterFirst != nil {
		u.afterFirst()
	}
	return response, err
}

func TestOpenAIPassthroughAdmissionRejectsBeforeFirstSend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, change := range []string{"disabled", "group", "route"} {
		t.Run(change, func(t *testing.T) {
			selected := passthroughAdmissionAccount()
			selected.GroupIDs = []int64{9}
			selected.Groups = []*Group{{ID: 9, Status: StatusActive}}
			repo := &turnAdmissionRepo{account: selected}
			repo.afterRead = func(n int, a *Account) {
				if n == 2 {
					next := *a
					switch change {
					case "disabled":
						next.Status = "disabled"
					case "group":
						next.GroupIDs = []int64{10}
					case "route":
						next.Credentials = maps.Clone(a.Credentials)
						next.Credentials["base_url"] = "http://changed.example"
					}
					repo.account = &next
				}
			}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`)
			c := adaptiveProtocolTestContext("/v1/responses", body)
			groupID := int64(9)
			c.Set("api_key", &APIKey{GroupID: &groupID})
			result, err := svc.Forward(context.Background(), c, selected, body)
			require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
			require.Nil(t, result)
			require.Empty(t, upstream.requests)
			var failover *UpstreamFailoverError
			require.False(t, errors.As(err, &failover))
		})
	}
}

func TestOpenAIPassthroughAdmissionRefreshesCredentials(t *testing.T) {
	selected := passthroughAdmissionAccount()
	repo := &turnAdmissionRepo{account: selected}
	repo.afterRead = func(n int, a *Account) {
		if n == 2 {
			next := *a
			next.Credentials = maps.Clone(a.Credentials)
			next.Credentials["api_key"] = "rotated-synthetic"
			repo.account = &next
		}
	}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`)
	original := string(body)
	ctx, pricingAt := WithGatewayTokenRequestPricing(context.Background())
	c := adaptiveProtocolTestContext("/v1/responses", body)
	c.Request = c.Request.WithContext(ctx)
	result, err := svc.Forward(ctx, c, selected, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer rotated-synthetic", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, 2, repo.reads)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "input").String())
	require.Equal(t, original, string(body))
	require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
}

func TestOpenAIPassthroughAdmissionRechecksRejectedFieldRetry(t *testing.T) {
	for _, change := range []string{"disabled", "mutable route", "cancelled"} {
		t.Run(change, func(t *testing.T) {
			selected := passthroughAdmissionAccount()
			repo := &turnAdmissionRepo{account: selected}
			if change == "disabled" {
				repo.afterRead = func(n int, a *Account) {
					if n == 3 {
						next := *a
						next.Schedulable = false
						repo.account = &next
					}
				}
			}
			firstBody := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: 'truncation'.","param":"truncation"}}`)}
			upstream := &passthroughAdmissionUpstream{httpUpstreamRecorder: &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: firstBody}, passthroughAdmissionResponse()}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if change == "cancelled" {
				upstream.afterFirst = cancel
			}
			// A reusable repository object must not let an in-place routing mutation
			// silently redefine this request's original binding on its next attempt.
			if change == "mutable route" {
				upstream.afterFirst = func() { selected.Credentials["base_url"] = "http://changed.example" }
			}
			svc := newTurnAdmissionGateway(repo, false)
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false,"truncation":"auto"}`)
			c := adaptiveProtocolTestContext("/v1/responses", body)
			c.Request = c.Request.WithContext(ctx)
			result, err := svc.Forward(ctx, c, selected, body)
			if change == "cancelled" {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
			}
			require.Nil(t, result)
			require.Len(t, upstream.requests, 1, "a local rejection must not send or replay a second upstream request")
			require.True(t, firstBody.closed, "the prior response is closed before retry admission")
			var failover *UpstreamFailoverError
			require.False(t, errors.As(err, &failover))
			_, hasUpstreamErrors := c.Get(OpsUpstreamErrorsKey)
			require.False(t, hasUpstreamErrors, "local admission must not create upstream health errors")
		})
	}
}

func TestOpenAIPassthroughAdmissionSuccessfulRetryKeepsPricing(t *testing.T) {
	selected := passthroughAdmissionAccount()
	repo := &turnAdmissionRepo{account: selected}
	repo.afterRead = func(n int, a *Account) {
		if n == 3 {
			next := *a
			next.Credentials = maps.Clone(a.Credentials)
			next.Credentials["api_key"] = "retry-rotated-synthetic"
			repo.account = &next
		}
	}
	firstBody := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: 'truncation'.","param":"truncation"}}`)}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: firstBody}, passthroughAdmissionResponse()}}
	svc := newTurnAdmissionGateway(repo, false)
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false,"truncation":"auto"}`)
	ctx, pricingAt := WithGatewayTokenRequestPricing(context.Background())
	c := adaptiveProtocolTestContext("/v1/responses", body)
	c.Request = c.Request.WithContext(ctx)
	result, err := svc.Forward(ctx, c, selected, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, 3, repo.reads)
	require.True(t, firstBody.closed)
	require.Equal(t, "Bearer sk-test", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "Bearer retry-rotated-synthetic", upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "auto", gjson.GetBytes(upstream.bodies[0], "truncation").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "truncation").Exists())
	require.Equal(t, "hello", gjson.GetBytes(upstream.bodies[1], "input").String())
	require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
}
