//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitServiceAccountOpsObserverQueuesClassifiedSignalOnce(t *testing.T) {
	ops := NewAccountOpsService(nil, nil, nil)
	cfg := defaultAccountOpsConfig()
	cfg.Enabled = true
	cfg.Recipient = "operations@example.test"
	ops.config.Store(cfg)

	svc := &RateLimitService{accountOps: ops}
	account := &Account{ID: 41, Name: "billing account", Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ctx := svc.observeAccountOps(context.Background(), account, http.StatusPaymentRequired, nil, []byte(`{"error":{"code":"insufficient_balance"}}`))
	svc.observeAccountOps(ctx, account, http.StatusPaymentRequired, nil, []byte(`{"error":{"code":"insufficient_balance"}}`))

	require.Len(t, ops.queue, 1)
	event := <-ops.queue
	require.Equal(t, int64(41), event.AccountID)
	require.Equal(t, "balance_low", event.Kind)
	require.Equal(t, "balance_error_code", event.Signal)
	require.Empty(t, event.Details, "raw upstream responses are not copied into the alert queue")
}

func TestRateLimitServiceAccountOpsObservesGatewayWeeklyLimit(t *testing.T) {
	for _, model := range []string{"gpt-5.3-codex-spark", ""} {
		t.Run("model="+model, func(t *testing.T) {
			ops := NewAccountOpsService(nil, nil, nil)
			cfg := defaultAccountOpsConfig()
			cfg.Enabled = true
			cfg.Recipient = "operations@example.test"
			ops.config.Store(cfg)

			repo := &oauth429RateLimitRepo{}
			limits := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			limits.accountOps = ops
			gateway := &OpenAIGatewayService{rateLimitService: limits}
			account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
			headers := make(http.Header)
			headers.Set("X-Codex-Primary-Window-Minutes", "10080")
			headers.Set("X-Codex-Primary-Used-Percent", "100")
			gateway.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, nil, model)

			require.Len(t, ops.queue, 1)
			event := <-ops.queue
			require.Equal(t, account.ID, event.AccountID)
			require.Equal(t, "weekly_quota", event.Kind)
			require.Equal(t, "codex_weekly_header", event.Signal)
			if model != "" {
				require.Equal(t, 1, repo.setModelRateLimitCalls, "Spark retains the existing model cooldown")
			}
		})
	}
}
