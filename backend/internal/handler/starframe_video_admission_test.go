//go:build unit

package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestStarframeActualAccountSlotKeepsVideoContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, acquired := range []bool{false, true} {
		t.Run(map[bool]string{false: "quick-acquire", true: "already-acquired"}[acquired], func(t *testing.T) {
			gw := &service.OpenAIGatewayService{}
			groupID := int64(50)
			cache := &profitCountingConcurrencyCache{}
			h := &OpenAIGatewayHandler{gatewayService: gw, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatClaude, 0)}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/videos", nil).WithContext(profitSlotTestContext(t, gw, groupID, false))
			requestCtx := service.WithStarframeVideoRequest(service.WithOpenAIProfitControlSuppressed(c.Request.Context()))
			account := profitSlotTestAccount(9, 0.8)
			account.Credentials = map[string]any{"video_protocol": "starframe", "base_url": "https://api.xzapi.vip", "api_key": "test-only", "openai_capabilities": []string{"starframe"}}
			selection := &service.AccountSelectionResult{Account: account, Acquired: acquired,
				WaitPlan: &service.AccountWaitPlan{AccountID: account.ID, MaxConcurrency: 2, Timeout: time.Second, MaxWaiting: 2}, ReleaseFunc: func() {}}
			streamStarted := false
			release, result := h.acquireStarframeVideoAccountSlot(c, requestCtx, &groupID, "", selection, &streamStarted, zap.NewNop())
			require.Equal(t, openAISlotAcquireOK, result)
			require.NotNil(t, release)
			require.True(t, service.IsStarframeVideoRequest(c.Request.Context()))
			release()
		})
	}
}
