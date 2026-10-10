package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestLedgerStorageErrorNeverBecomesAccountFailover(t *testing.T) {
	var service *OpenAIGatewayService
	err := service.handleOpenAIUpstreamTransportError(context.Background(), nil, nil, requestledger.ErrUnavailable, false)
	require.ErrorIs(t, err, requestledger.ErrUnavailable)
	var failover *UpstreamFailoverError
	require.NotErrorAs(t, err, &failover)
}

func TestLedgerStorageErrorDoesNotFailOverAnthropicOrGemini(t *testing.T) {
	for _, platform := range []string{"anthropic", "gemini"} {
		t.Run(platform, func(t *testing.T) {
			ctx := context.Background()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			account := &Account{ID: 101, Platform: platform, Type: AccountTypeAPIKey}
			var err error
			if platform == "anthropic" {
				err = (&GatewayService{}).handleUpstreamTransportError(ctx, c, account, requestledger.ErrUnavailable, OpsUpstreamErrorEvent{})
			} else {
				err = (&GeminiMessagesCompatService{}).handleUpstreamTransportError(ctx, c, account, requestledger.ErrUnavailable)
			}
			require.ErrorIs(t, err, requestledger.ErrUnavailable)
			var failover *UpstreamFailoverError
			require.NotErrorAs(t, err, &failover)
		})
	}
}
