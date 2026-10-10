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

type cacheFailureUpstream struct {
	service.HTTPUpstream
	calls   int
	metered bool
}

func (u *cacheFailureUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	usage := ""
	if u.metered {
		usage = `,"usage":{"input_tokens":9,"output_tokens":2}`
	}
	payload := `data: {"type":"response.failed","response":{"id":"synthetic_cache_failure","status":"failed","model":"gpt-5.1","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}],"error":{"code":"invalid_parameter","message":"prompt_cache_breakpoint is not supported on this model"}` + usage + `}}` + "\n\n"
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
}

func TestResponsesCachePartialFailureNeverReplaysOrInventsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			for _, metered := range []bool{false, true} {
				t.Run(fmt.Sprintf("passthrough=%v/stream=%v/metered=%v", passthrough, stream, metered), func(t *testing.T) {
					accounts := []service.Account{
						{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_passthrough": passthrough}},
						{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: 1, Credentials: map[string]any{"api_key": "synthetic", "base_url": "https://upstream.example"}, Extra: map[string]any{"openai_passthrough": passthrough}},
					}
					repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: accounts}
					logs := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
					upstream := &cacheFailureUpstream{metered: metered}
					cfg := &config.Config{RunMode: config.RunModeSimple}
					cfg.Default.RateMultiplier = 1
					billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
					t.Cleanup(billing.Stop)
					gateway := service.NewOpenAIGatewayService(repo, logs, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
					h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
					c, _ := newOpenAIResponsesFailoverTestContext(t, context.Background())
					c.Request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"gpt-5.1","stream":%v,"input":"hello"}`, stream)))
					h.Responses(c)
					require.Equal(t, 1, upstream.calls, "missing error type or usage does not authorize account replay")
					require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
					_, hasFailures := c.Get(service.OpsUpstreamErrorsKey)
					require.True(t, hasFailures, "the failed attempt must be retained independently of billing")
					if metered {
						select {
						case usageLog := <-logs.created:
							require.Equal(t, 9, usageLog.InputTokens)
							require.Equal(t, 2, usageLog.OutputTokens)
						case <-time.After(3 * time.Second):
							t.Fatal("observed failed-response usage was not retained for settlement")
						}
					}
					require.Never(t, func() bool { return len(logs.created) > 0 }, 100*time.Millisecond, 10*time.Millisecond, "no invented or duplicate usage records")
				})
			}
		}
	}
}
