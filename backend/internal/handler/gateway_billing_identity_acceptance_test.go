package handler

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayBillingIdentitySurvivesQueuedUsageCallbacks(t *testing.T) {
	for _, name := range []string{"gateway", "openai"} {
		t.Run(name, func(t *testing.T) {
			pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
				WorkerCount: 1, QueueSize: 8, TaskTimeout: 10 * time.Second,
			})
			t.Cleanup(pool.Stop)
			started, release := make(chan struct{}), make(chan struct{})
			released := false
			defer func() {
				if !released {
					close(release)
				}
			}()
			require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) {
				close(started)
				<-release
			}))
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("worker did not start")
			}
			gateway := &GatewayHandler{usageRecordWorkerPool: pool}
			openai := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
			submit := gateway.submitUsageRecordTask
			switch name {
			case "openai":
				submit = openai.submitUsageRecordTask
			}
			parent := context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "queued-server-request")
			parent, cancel := context.WithCancel(parent)
			cancel()
			type observation struct {
				identity string
				err      error
			}
			observations := make(chan observation, 2)
			for range 2 {
				submit(parent, func(ctx context.Context) {
					id, _ := ctx.Value(ctxkey.UsageBillingRequestID).(string)
					observations <- observation{identity: id, err: ctx.Err()}
				})
			}
			select {
			case <-observations:
				t.Fatal("callback ran inline instead of waiting in the worker queue")
			default:
			}
			close(release)
			released = true
			for range 2 {
				select {
				case got := <-observations:
					require.Equal(t, "queued-server-request", got.identity)
					require.NoError(t, got.err)
				case <-time.After(5 * time.Second):
					t.Fatal("queued callback did not execute")
				}
			}
			require.Zero(t, pool.Stats().SyncFallbackTasks)
		})
	}
}

type durableWorkerIdentityBillingRepo struct {
	service.UsageBillingRepository
	fingerprints map[string]string
	lastCommand  *service.UsageBillingCommand
	effects      int
}

func (r *durableWorkerIdentityBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	cmd.Normalize()
	r.lastCommand = cmd
	if previous, exists := r.fingerprints[cmd.RequestID]; exists {
		if previous != cmd.RequestFingerprint {
			return nil, service.ErrUsageBillingRequestConflict
		}
		return &service.UsageBillingApplyResult{}, nil
	}
	r.fingerprints[cmd.RequestID] = cmd.RequestFingerprint
	r.effects++
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

type durableWorkerIdentityUsageRepo struct{ service.UsageLogRepository }

func (*durableWorkerIdentityUsageRepo) Create(context.Context, *service.UsageLog) (bool, error) {
	return true, nil
}

func TestImageWorkerBillingIdentityPreservesExactSettlementKeys(t *testing.T) {
	billing := &durableWorkerIdentityBillingRepo{fingerprints: map[string]string{}}
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	svc := service.NewOpenAIGatewayService(
		nil, &durableWorkerIdentityUsageRepo{}, billing, nil, nil, nil, nil, cfg,
		nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{},
		nil, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
	)
	parent := context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "shared-submission-ingress")
	for _, durableID := range []string{"task-a", "image-studio:job-1:item-a", "image-studio:job-1:item-b", "task-a"} {
		ctx := usageRecordContext(imageWorkerBillingContext(parent, durableID), parent)
		input := &service.OpenAIRecordUsageInput{
			Result: &service.OpenAIForwardResult{RequestID: "upstream-result", Model: "gpt-5.1", Duration: time.Second, Usage: service.OpenAIUsage{InputTokens: 8, OutputTokens: 4}},
			APIKey: &service.APIKey{ID: 100}, User: &service.User{ID: 200}, Account: &service.Account{ID: 300},
		}
		for range 2 {
			require.NoError(t, svc.RecordUsage(ctx, input))
			require.Equal(t, "client:"+durableID, billing.lastCommand.RequestID)
			require.Greater(t, billing.lastCommand.BalanceCost, 0.0)
		}
	}
	require.Equal(t, 3, billing.effects)
	require.Len(t, billing.fingerprints, 3)
	require.Contains(t, billing.fingerprints, "client:task-a")
	require.Contains(t, billing.fingerprints, "client:image-studio:job-1:item-a")
	require.Contains(t, billing.fingerprints, "client:image-studio:job-1:item-b")
}
