//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestResolveUsageBillingRequestID_PrivateIdentitySeparatesCorrelation(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "reused")
	ctx = context.WithValue(ctx, ctxkey.RequestID, "also-reused")
	for _, id := range []string{"server-a", "server-b"} {
		request := context.WithValue(ctx, ctxkey.UsageBillingRequestID, id)
		require.Equal(t, "gateway:"+id, resolveUsageBillingRequestID(request, "upstream"))
		require.Equal(t, resolveUsageBillingRequestID(request, "upstream"), resolveUsageBillingRequestID(request, "upstream"))
		for _, forced := range []string{"web_search:x", "grok-video:x", "agnes-video:x", "starframe-video:9:x", "grok_audio:x", "grok_realtime:x", "openai_audio:x", "openai_realtime:x"} {
			require.Equal(t, forced, resolveUsageBillingRequestID(request, forced))
		}
	}
}

func TestOpenAIRecordUsagePrivateIdentityAndWSResponsePriority(t *testing.T) {
	for _, ws := range []bool{false, true} {
		usageRepo := &privateIdentityUsageRepo{}
		billingRepo := &privateIdentityBillingRepo{}
		cfg := &config.Config{}
		cfg.Default.RateMultiplier = 1
		svc := NewOpenAIGatewayService(
			nil, usageRepo, billingRepo, nil, nil, nil, nil, cfg,
			nil, nil, NewBillingService(cfg, nil), nil, &BillingCacheService{},
			nil, &DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
		)
		ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "same-correlation")
		ctx = context.WithValue(ctx, ctxkey.UsageBillingRequestID, "server-ingress")
		for _, responseID := range []string{"response-a", "response-b", "response-a", ""} {
			err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{RequestID: responseID, OpenAIWSMode: ws, Model: "gpt-5.1", Duration: time.Second, Usage: OpenAIUsage{InputTokens: 8, OutputTokens: 4}},
				APIKey: &APIKey{ID: 100}, User: &User{ID: 200}, Account: &Account{ID: 300},
			})
			require.NoError(t, err)
			expected := "gateway:server-ingress"
			// A private ingress ID identifies the WS connection. Without an
			// observed response ID, this slice deliberately retains its fallback;
			// per-turn fallback identity needs a separate admission snapshot.
			if ws && responseID != "" {
				expected = responseID
			}
			require.Equal(t, expected, billingRepo.lastCmd.RequestID)
			require.Equal(t, expected, usageRepo.lastLog.RequestID)
		}
	}
}

// Self-contained repositories keep these regressions runnable with all real
// production GoFiles, without compiling unrelated service test fixtures.
type privateIdentityUsageRepo struct {
	UsageLogRepository
	lastLog *UsageLog
}

func (r *privateIdentityUsageRepo) Create(_ context.Context, log *UsageLog) (bool, error) {
	r.lastLog = log
	return true, nil
}

type privateIdentityBillingRepo struct {
	UsageBillingRepository
	lastCmd *UsageBillingCommand
}

func (r *privateIdentityBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	cmd.Normalize()
	r.lastCmd = cmd
	return &UsageBillingApplyResult{Applied: true}, nil
}
