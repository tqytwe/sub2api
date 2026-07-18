//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBatchImageWorkerRuntime_QueueDisabledDoesNotStart(t *testing.T) {
	queue := &blockingBatchImageRuntimeQueue{}
	runtime := NewBatchImageWorkerRuntime(
		NewBatchImageWorker(queue, &fakeBatchImageProcessor{}, BatchImageWorkerOptions{}),
		&config.Config{BatchImage: config.BatchImageConfig{QueueEnabled: false}},
	)

	runtime.Start()

	require.False(t, runtime.Running())
	require.Zero(t, queue.reserveCalls.Load())
	require.NotPanics(t, runtime.Stop)
}

func TestBatchImageWorkerRuntime_QueueEnabledStartsAndStops(t *testing.T) {
	queue := &blockingBatchImageRuntimeQueue{}
	processor := &fakeBatchImageProcessor{}
	cfg := &config.Config{BatchImage: config.BatchImageConfig{QueueEnabled: true}}
	state := NewBatchImageRuntimeState(queue, cfg)
	runtime := NewBatchImageWorkerRuntimeWithState(
		NewBatchImageWorker(queue, processor, BatchImageWorkerOptions{
			DelayedPollInterval: time.Hour,
			RecoveryInterval:    time.Hour,
		}),
		state,
		cfg,
	)

	runtime.Start()

	require.Eventually(t, func() bool {
		return runtime.Running() && queue.reserveCalls.Load() > 0
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, state.RequireReady(context.Background()))
	require.Empty(t, processor.processed)
	require.NotPanics(t, runtime.Stop)
	require.False(t, runtime.Running())
	require.ErrorIs(t, state.RequireReady(context.Background()), ErrBatchImageRuntimeNotReady)
	require.NotPanics(t, runtime.Stop)
}

func TestBatchImageRuntimeState_RequiresHealthyRedis(t *testing.T) {
	queue := &blockingBatchImageRuntimeQueue{}
	cfg := &config.Config{BatchImage: config.BatchImageConfig{QueueEnabled: true}}
	state := NewBatchImageRuntimeState(queue, cfg)
	state.SetWorkerRunning(true)
	queue.pingErr = errors.New("redis unavailable")

	err := state.RequireReady(context.Background())

	require.ErrorIs(t, err, ErrBatchImageRuntimeNotReady)
	require.Equal(t, "redis unavailable", state.LastError())

	queue.pingErr = nil
	require.NoError(t, state.RequireReady(context.Background()))
}

func TestBatchImageRuntimeState_SnapshotIncludesQueueState(t *testing.T) {
	queue := &blockingBatchImageRuntimeQueue{
		stats: BatchImageQueueStats{Ready: 3, Delayed: 2, Active: 1},
	}
	cfg := &config.Config{BatchImage: config.BatchImageConfig{
		Enabled:      true,
		QueueEnabled: true,
	}}
	state := NewBatchImageRuntimeState(queue, cfg)
	state.SetWorkerRunning(true)

	got := state.Snapshot(context.Background())

	require.True(t, got.Enabled)
	require.True(t, got.QueueEnabled)
	require.True(t, got.RedisReady)
	require.True(t, got.WorkerRunning)
	require.True(t, got.Ready)
	require.Equal(t, BatchImageQueueStats{Ready: 3, Delayed: 2, Active: 1}, got.Queue)
}

type blockingBatchImageRuntimeQueue struct {
	reserveCalls atomic.Int64
	pingErr      error
	stats        BatchImageQueueStats
}

func (q *blockingBatchImageRuntimeQueue) Enqueue(context.Context, string) error {
	return nil
}

func (q *blockingBatchImageRuntimeQueue) Reserve(ctx context.Context, _ time.Duration) (ReservedBatchImageJob, error) {
	q.reserveCalls.Add(1)
	<-ctx.Done()
	return ReservedBatchImageJob{}, ctx.Err()
}

func (q *blockingBatchImageRuntimeQueue) RequeueAfter(context.Context, string, time.Duration) error {
	return nil
}

func (q *blockingBatchImageRuntimeQueue) Ack(context.Context, string) error {
	return nil
}

func (q *blockingBatchImageRuntimeQueue) Heartbeat(context.Context, string) error {
	return nil
}

func (q *blockingBatchImageRuntimeQueue) MoveDueDelayedToReady(context.Context, int) (int, error) {
	return 0, nil
}

func (q *blockingBatchImageRuntimeQueue) RecoverStaleActive(context.Context, time.Duration, int) (int, error) {
	return 0, nil
}

func (q *blockingBatchImageRuntimeQueue) TryAcquireJobLock(context.Context, string, time.Duration) (BatchImageJobLock, bool, error) {
	return nil, false, nil
}

func (q *blockingBatchImageRuntimeQueue) Ping(context.Context) error {
	return q.pingErr
}

func (q *blockingBatchImageRuntimeQueue) Stats(context.Context) (BatchImageQueueStats, error) {
	return q.stats, q.pingErr
}
