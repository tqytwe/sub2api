//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// A capability-only empty pool must stop before Forward and never record an
// upstream error, account switch, or synthetic usage on the Chat-only account.
func TestResponsesToolsCapabilityHandlerStopsBeforeForward(t *testing.T) {
	for _, mode := range []string{"force_chat_completions", "auto"} {
		for _, stream := range []string{"false", "true"} {
			t.Run(mode+"/"+stream, func(t *testing.T) {
				repo := &handlerAdmissionRejectedRepo{openAIImagesFailoverAccountRepo: openAIImagesFailoverAccountRepo{accounts: []service.Account{{
					ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
					Credentials: map[string]any{"api_key": "synthetic", "base_url": "http://unused.example", "model_mapping": map[string]any{"public-alias": "gpt-6.1-sol"}},
					Extra:       map[string]any{"openai_responses_mode": mode, "openai_responses_supported": false},
				}}}}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				upstream := &openAIResponsesFailoverCancelUpstream{}
				gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
				c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"public-alias","input":"hello","tools":[{"type":"function","name":"lookup"}],"stream":`+stream+`}`))
				h.Responses(c)
				require.Equal(t, http.StatusServiceUnavailable, rec.Code)
				require.Equal(t, "responses_tools_not_supported", gjson.Get(rec.Body.String(), "error.type").String())
				require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "Responses tool calls")
				require.True(t, gjson.Valid(rec.Body.String()))
				require.NotContains(t, rec.Body.String(), "event:")
				require.Zero(t, repo.reads, "must reject at selection, before authoritative Forward admission")
				require.Empty(t, upstream.calls())
				_, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
				require.False(t, hasUpstreamErrors)
				require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
			})
		}
	}
}
