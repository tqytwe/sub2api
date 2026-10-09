//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type handlerAdmissionRejectedRepo struct {
	openAIImagesFailoverAccountRepo
	reads int
}

func (r *handlerAdmissionRejectedRepo) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	r.reads++
	a, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	a.Status = "disabled"
	return a, nil, nil
}

func TestResponsesAdmissionRejectionIsLocalAndNotReplayed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &handlerAdmissionRejectedRepo{openAIImagesFailoverAccountRepo: openAIImagesFailoverAccountRepo{accounts: []service.Account{{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic", "base_url": "http://unused.example"}}}}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	upstream := &openAIResponsesFailoverCancelUpstream{}
	gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
	h.Responses(c)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "admission_unavailable", gjson.Get(rec.Body.String(), "error.type").String())
	require.Equal(t, 1, repo.reads, "local denial must not select/replay another account")
	require.Empty(t, upstream.calls())
	_, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
	require.False(t, hasUpstreamErrors)
	require.Zero(t, gateway.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
}

func TestResponsesAdmissionErrorStreamingAndNonAdmissionErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &OpenAIGatewayHandler{}
	for _, streamStarted := range []bool{false, true} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		if streamStarted {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.WriteHeader(http.StatusOK)
			_, err := c.Writer.WriteString("event: response.created\ndata: {}\n\n")
			require.NoError(t, err)
		}
		handled := h.handleOpenAITurnAdmissionError(c, &service.OpenAITurnAdmissionError{Reason: "account_binding_changed"}, streamStarted)
		require.True(t, handled)
		require.Contains(t, rec.Body.String(), "admission_unavailable")
		if streamStarted {
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), "response.failed")
		} else {
			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.False(t, h.handleOpenAITurnAdmissionError(c, errors.New("upstream failed"), false))
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}
