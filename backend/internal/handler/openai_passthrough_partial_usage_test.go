//go:build unit

package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type partialPassthroughUsageUpstream struct {
	service.HTTPUpstream
	calls int
	code  string
}

func (u *partialPassthroughUsageUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	if strings.HasPrefix(u.code, "image_") {
		payload := `data: {"type":"response.failed","response":{"id":"resp_image_partial","status":"failed","model":"gpt-5.1","error":{"code":"rate_limit_exceeded","message":"rate limit exceeded"},"output":[{"id":"image_complete","type":"image_generation_call","status":"completed","result":"synthetic-final-image","size":"1024x1024"}]}}` + "\n\n"
		if u.code == "image_after_output" {
			payload = `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n" + payload
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"partial_image_request"}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
	}
	if u.code == "admission" {
		payload := "data: " + `{"type":"response.in_progress","response":{"id":"resp_partial","model":"gpt-5.1","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}` + "\n\n"
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"partial_request"}}, Body: io.NopCloser(io.MultiReader(strings.NewReader(payload), partialUsageAdmissionReader{}))}, nil
	}
	if strings.HasSuffix(req.URL.Path, "/chat/completions") {
		payload := `data: {"id":"partial_cc","object":"chat.completion.chunk","model":"gpt-5.1","choices":[],"usage":{"prompt_tokens":9,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":4}}}` + "\n\n"
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"partial_request"}}, Body: io.NopCloser(io.MultiReader(strings.NewReader(payload), partialUsageFailureReader{}))}, nil
	}
	payload := fmt.Sprintf(`{"type":"response.failed","response":{"id":"resp_partial","status":"failed","model":"gpt-5.1","error":{"code":%q,"message":%q},"usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}`, u.code, u.code)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"partial_request"}}, Body: io.NopCloser(strings.NewReader("data: " + payload + "\n\n"))}, nil
}

func TestResponsesPassthroughMeteredFailureSettlesOnceWithoutFailover(t *testing.T) {
	testResponsesMeteredFailureSettlesOnceWithoutFailover(t, true, "rate_limit_exceeded")
}

func TestResponsesNativeMeteredFailureSettlesOnceWithoutFailover(t *testing.T) {
	testResponsesMeteredFailureSettlesOnceWithoutFailover(t, false, "rate_limit_exceeded")
}

func TestResponsesMeteredLocalAdmissionFailureSettlesBeforeGuard(t *testing.T) {
	for _, passthrough := range []bool{true, false} {
		t.Run(fmt.Sprintf("passthrough=%v", passthrough), func(t *testing.T) {
			testResponsesMeteredFailureSettlesOnceWithoutFailover(t, passthrough, "admission")
		})
	}
}

func TestResponsesCyberMeteredFailureSettlesOnce(t *testing.T) {
	for _, passthrough := range []bool{true, false} {
		t.Run(fmt.Sprintf("passthrough=%v", passthrough), func(t *testing.T) {
			testResponsesMeteredFailureSettlesOnceWithoutFailover(t, passthrough, "cyber_policy")
		})
	}
}

func TestResponsesMeteredImageFailurePreservesErrorResponse(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, code := range []string{"image_before_output", "image_after_output"} {
			t.Run(fmt.Sprintf("passthrough=%v/%s", passthrough, code), func(t *testing.T) {
				testResponsesMeteredFailureSettlesOnceWithoutFailover(t, passthrough, code)
			})
		}
	}
}

func testResponsesMeteredFailureSettlesOnceWithoutFailover(t *testing.T, passthrough bool, code string) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{true, false} {
		if (code == "admission" || code == "image_after_output") && !stream {
			continue
		}
		t.Run(fmt.Sprintf("stream=%v", stream), func(t *testing.T) {
			accounts := []service.Account{
				{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_passthrough": passthrough}},
				{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 1, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_passthrough": passthrough}},
			}
			repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: accounts}
			logs := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
			upstream := &partialPassthroughUsageUpstream{code: code}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			gateway := service.NewOpenAIGatewayService(repo, logs, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
			c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"gpt-5.1","stream":%v,"input":"hello"}`, stream)))
			h.Responses(c)
			if strings.HasPrefix(code, "image_") {
				if code == "image_after_output" {
					require.Equal(t, http.StatusOK, rec.Code)
					require.Contains(t, rec.Body.String(), "response.failed")
					require.Contains(t, rec.Body.String(), "partial")
				} else {
					require.Equal(t, http.StatusTooManyRequests, rec.Code, "a metered failed image must not turn an uncommitted error into empty 200")
					require.Contains(t, rec.Body.String(), "error")
				}
			}
			if code == "admission" {
				require.Contains(t, rec.Body.String(), "admission_unavailable")
				_, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
				require.False(t, hasUpstreamErrors, "a local admission error must not be classified as upstream health failure")
			}
			require.Equal(t, 1, upstream.calls, "metered failure must stop before same-account or cross-account replay")
			select {
			case log := <-logs.created:
				require.Equal(t, int64(1), log.AccountID)
				if strings.HasPrefix(code, "image_") {
					require.Equal(t, 1, log.ImageCount)
					require.Zero(t, log.InputTokens)
					require.Zero(t, log.OutputTokens)
					require.Zero(t, log.CacheReadTokens)
				} else {
					require.Equal(t, 5, log.InputTokens)
					require.Equal(t, 2, log.OutputTokens)
					require.Equal(t, 4, log.CacheReadTokens)
				}
				require.Equal(t, int64(100), log.UserID)
				require.Equal(t, int64(99), log.APIKeyID)
				if code == "cyber_policy" {
					require.Equal(t, service.RequestTypeCyberBlocked, log.RequestType)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("observed partial usage was not submitted for settlement")
			}
			require.Never(t, func() bool { return len(logs.created) > 0 }, 100*time.Millisecond, 10*time.Millisecond, "one upstream result must produce one usage record")
			require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
		})
	}
}

type partialUsageAdmissionReader struct{}

func (partialUsageAdmissionReader) Read([]byte) (int, error) {
	return 0, &service.OpenAITurnAdmissionError{Reason: "account_binding_changed"}
}

type partialUsageFailureReader struct{}

func (partialUsageFailureReader) Read([]byte) (int, error) {
	return 0, errors.New("synthetic upstream interruption")
}

func TestMessagesNativeMeteredFailureSettlesOnceWithoutFailover(t *testing.T) {
	for _, stream := range []bool{true, false} {
		for _, code := range []string{"rate_limit_exceeded", "cyber_policy"} {
			t.Run(fmt.Sprintf("stream=%v/%s", stream, code), func(t *testing.T) {
				testMessagesMeteredFailureSettlement(t, "force_responses", stream, code)
			})
		}
	}
}

func TestMessagesSharedCCMeteredFailureSettlesOnceWithoutFailover(t *testing.T) {
	testMessagesMeteredFailureSettlement(t, "force_chat_completions", true, "interrupted")
}

func testMessagesMeteredFailureSettlement(t *testing.T, mode string, stream bool, code string) {
	gin.SetMode(gin.TestMode)
	accounts := []service.Account{
		{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_responses_mode": mode}},
		{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 1, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_responses_mode": mode}},
	}
	repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: accounts}
	logs := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	upstream := &partialPassthroughUsageUpstream{code: code}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(repo, logs, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
	c.Request.URL.Path = "/v1/messages"
	c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"gpt-5.1","max_tokens":10,"stream":%v,"messages":[{"role":"user","content":"hello"}]}`, stream)))
	rawKey, _ := c.Get("api_key")
	rawKey.(*service.APIKey).Group.AllowMessagesDispatch = true
	h.Messages(c)
	if code == "admission" {
		require.Contains(t, rec.Body.String(), "admission_unavailable")
		_, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
		require.False(t, hasUpstreamErrors, "a local admission error must not be classified as upstream health failure")
	}
	require.Equal(t, 1, upstream.calls, "metered failure must never be replayed")
	select {
	case log := <-logs.created:
		require.Equal(t, int64(1), log.AccountID)
		require.Equal(t, 5, log.InputTokens)
		require.Equal(t, 2, log.OutputTokens)
		require.Equal(t, 4, log.CacheReadTokens)
		require.Equal(t, int64(100), log.UserID)
		require.Equal(t, int64(99), log.APIKeyID)
		if code == "cyber_policy" {
			require.Equal(t, service.RequestTypeCyberBlocked, log.RequestType)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("observed partial usage was not submitted for settlement")
	}
	require.Never(t, func() bool { return len(logs.created) > 0 }, 100*time.Millisecond, 10*time.Millisecond, "cyber policy must not create a second settlement")
	require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
}

func TestMessagesMeteredLocalAdmissionFailureSettlesBeforeGuard(t *testing.T) {
	testMessagesMeteredFailureSettlement(t, "force_responses", true, "admission")
}
