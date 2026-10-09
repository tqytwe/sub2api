package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIObservedTextUsageCannotDropWhenPoolOverflows(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second,
		OverflowPolicy: config.UsageRecordOverflowPolicyDrop,
	})
	t.Cleanup(pool.Stop)
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) {
		close(started)
		<-release
	}))
	<-started
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) {}))
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "private-settlement-id"))
	cancel()
	var calls atomic.Int32
	h.submitOpenAIUsageRecordTaskForAccount(parent, &service.Account{Platform: service.PlatformOpenAI}, &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9, OutputTokens: 2}}, func(ctx context.Context) {
		require.NoError(t, ctx.Err())
		require.Equal(t, "private-settlement-id", ctx.Value(ctxkey.UsageBillingRequestID))
		calls.Add(1)
	})
	require.EqualValues(t, 1, calls.Load(), "observed usage must settle once despite cancellation and queue overflow")
}

func TestOpenAIObservedUsageQueuePreservesGrokScope(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		for _, image := range []bool{false, true} {
			t.Run(platform+map[bool]string{false: "/text", true: "/image"}[image], func(t *testing.T) {
				pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second, OverflowPolicy: config.UsageRecordOverflowPolicyDrop})
				t.Cleanup(pool.Stop)
				started, release := make(chan struct{}), make(chan struct{})
				t.Cleanup(func() { close(release) })
				pool.Submit(func(context.Context) { close(started); <-release })
				<-started
				pool.Submit(func(context.Context) {})
				h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
				result := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9, OutputTokens: 2}}
				if image {
					result.ImageCount = 1
				}
				var calls atomic.Int32
				h.submitOpenAIUsageRecordTaskForAccount(context.Background(), &service.Account{Platform: platform}, result, func(context.Context) { calls.Add(1) })
				want := int32(0)
				if platform == service.PlatformOpenAI || image {
					want = 1
				}
				require.Equal(t, want, calls.Load())
			})
		}
	}
}

func TestOpenAIUsageRawTextRetainsConfiguredOverflow(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second, OverflowPolicy: config.UsageRecordOverflowPolicyDrop})
	t.Cleanup(pool.Stop)
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	pool.Submit(func(context.Context) { close(started); <-release })
	<-started
	pool.Submit(func(context.Context) {})
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
	var calls atomic.Int32
	h.submitOpenAIUsageRecordTask(context.Background(), &service.OpenAIForwardResult{Usage: service.OpenAIUsage{InputTokens: 9, OutputTokens: 2}}, func(context.Context) { calls.Add(1) })
	require.Zero(t, calls.Load(), "legacy raw callers must keep configured text overflow policy")
}
