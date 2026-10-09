package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMandatoryUsageRecordStoppedPoolExecutesExactlyOnce(t *testing.T) {
	for _, imageResult := range []bool{false, true} {
		name := "mandatory"
		if imageResult {
			name = "image_result"
		}
		t.Run(name, func(t *testing.T) {
			pool := newStoppedUsageRecordPoolForTest()
			h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
			parent, cancel := context.WithCancel(context.Background())
			cancel()
			var calls atomic.Int32
			task := func(ctx context.Context) {
				require.NoError(t, ctx.Err(), "settlement must survive request cancellation")
				calls.Add(1)
			}
			if imageResult {
				h.submitOpenAIUsageRecordTask(parent, &service.OpenAIForwardResult{ImageCount: 1}, task)
			} else {
				h.submitMandatoryUsageRecordTask(parent, task)
			}
			require.EqualValues(t, 1, calls.Load(), "a stopped pool must settle inline exactly once")
		})
	}
}

func TestMandatoryUsageRecordQueueBoundariesExecuteExactlyOnce(t *testing.T) {
	for _, policy := range []string{"enqueued", config.UsageRecordOverflowPolicyDrop, config.UsageRecordOverflowPolicySync} {
		t.Run(policy, func(t *testing.T) {
			pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
				WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second,
				OverflowPolicy: policy,
			})
			t.Cleanup(pool.Stop)
			h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
			if policy != "enqueued" {
				started, release := make(chan struct{}), make(chan struct{})
				t.Cleanup(func() { close(release) })
				require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) {
					close(started)
					<-release
				}))
				<-started
				require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) {}))
			}
			var calls atomic.Int32
			h.submitMandatoryUsageRecordTask(context.Background(), func(context.Context) { calls.Add(1) })
			if policy == "enqueued" {
				pool.Stop()
			}
			require.EqualValues(t, 1, calls.Load())
		})
	}
}
