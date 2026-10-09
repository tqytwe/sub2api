//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMessagesAdmissionRejectionIsLocalAndNotReplayed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"force_chat_completions", "force_responses"} {
		t.Run(mode, func(t *testing.T) {
			repo := &handlerAdmissionRejectedRepo{openAIImagesFailoverAccountRepo: openAIImagesFailoverAccountRepo{accounts: []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic", "base_url": "http://unused.example"}, Extra: map[string]any{"openai_responses_mode": mode}}}}}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			upstream := &openAIResponsesFailoverCancelUpstream{}
			gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
			c.Request.URL.Path = "/v1/messages"
			c.Request.Body = io.NopCloser(strings.NewReader(`{"model":"gpt-5.1","max_tokens":10,"messages":[{"role":"user","content":"hello"}]}`))
			rawKey, _ := c.Get("api_key")
			rawKey.(*service.APIKey).Group.AllowMessagesDispatch = true
			h.Messages(c)
			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			require.Equal(t, "admission_unavailable", gjson.Get(rec.Body.String(), "error.type").String())
			require.Equal(t, "error", gjson.Get(rec.Body.String(), "type").String())
			require.Equal(t, 1, repo.reads)
			require.Empty(t, upstream.calls())
			_, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
			require.False(t, hasUpstreamErrors)
			require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
		})
	}
}
