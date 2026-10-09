//go:build unit

package handler

import (
	"context"
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

func TestChatMeteredFailureSettlesOnceWithoutFailover(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, code := range []string{"rate_limit_exceeded", "cyber_policy", "admission"} {
			t.Run(fmt.Sprintf("stream=%v/%s", stream, code), func(t *testing.T) {
				testChatMeteredFailureSettlement(t, "force_responses", stream, code)
			})
		}
	}
}

func TestChatMeteredLocalAdmissionAfterOutputKeepsSSEFraming(t *testing.T) {
	testChatMeteredFailureSettlement(t, "force_responses", true, "admission_after_output")
}

type chatMeteredFailureUpstream struct {
	*partialPassthroughUsageUpstream
}

func (u *chatMeteredFailureUpstream) Do(req *http.Request, proxyURL string, accountID int64, concurrency int) (*http.Response, error) {
	if u.code != "admission_after_output" {
		return u.partialPassthroughUsageUpstream.Do(req, proxyURL, accountID, concurrency)
	}
	u.calls++
	payload := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_partial","model":"gpt-5.1","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		"",
		`data: {"type":"response.in_progress","response":{"id":"resp_partial","model":"gpt-5.1","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}`,
		"", "",
	}, "\n")
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"partial_request"}}, Body: io.NopCloser(io.MultiReader(strings.NewReader(payload), partialUsageAdmissionReader{}))}, nil
}

func TestChatSharedCCMeteredFailureSettlesOnceWithoutFailover(t *testing.T) {
	testChatMeteredFailureSettlement(t, "force_chat_completions", true, "interrupted")
}

func testChatMeteredFailureSettlement(t *testing.T, mode string, stream bool, code string) {
	gin.SetMode(gin.TestMode)
	accounts := []service.Account{
		{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_responses_mode": mode}},
		{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 1, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_responses_mode": mode}},
	}
	repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: accounts}
	logs := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	upstream := &chatMeteredFailureUpstream{partialPassthroughUsageUpstream: &partialPassthroughUsageUpstream{code: code}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(repo, logs, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
	c.Request.URL.Path = "/v1/chat/completions"
	c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"gpt-5.1","stream":%v,"messages":[{"role":"user","content":"hello"}]}`, stream)))
	h.ChatCompletions(c)
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
	require.Never(t, func() bool { return len(logs.created) > 0 }, 100*time.Millisecond, 10*time.Millisecond, "one upstream result must produce one usage record")
	require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
	if code == "admission_after_output" {
		require.Equal(t, http.StatusOK, rec.Code, "already-started SSE retains its original status")
		require.Contains(t, rec.Body.String(), `"content":"partial"`)
		require.Contains(t, rec.Body.String(), "event: error\ndata: ", "local admission error after output must remain SSE-framed")
		require.Contains(t, rec.Body.String(), "admission_unavailable")
		for _, line := range strings.Split(rec.Body.String(), "\n") {
			if strings.TrimSpace(line) != "" {
				require.True(t, strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, ":"), "raw JSON must not be appended to SSE: %s", line)
			}
		}
	}
	if strings.HasPrefix(code, "admission") {
		_, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
		require.False(t, hasUpstreamErrors, "local errors must not acquire upstream-health side effects during read classification")
	}
}
