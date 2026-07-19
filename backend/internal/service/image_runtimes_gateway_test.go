package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageRuntimeHealthQueue struct {
	statsErr error
}

func (imageRuntimeHealthQueue) Submit(context.Context, *ImageTaskRecord, time.Duration, string) (string, bool, error) {
	return "", false, nil
}
func (imageRuntimeHealthQueue) Reserve(context.Context, time.Duration) (string, error) {
	return "", ErrImageTaskQueueEmpty
}
func (imageRuntimeHealthQueue) Ack(context.Context, string) error       { return nil }
func (imageRuntimeHealthQueue) Requeue(context.Context, string) error   { return nil }
func (imageRuntimeHealthQueue) Heartbeat(context.Context, string) error { return nil }
func (imageRuntimeHealthQueue) RecoverStaleActive(context.Context, time.Duration, int) (int, error) {
	return 0, nil
}
func (imageRuntimeHealthQueue) TryAcquireJobLock(context.Context, string, time.Duration) (ImageTaskJobLock, bool, error) {
	return nil, false, nil
}
func (imageRuntimeHealthQueue) Ping(context.Context) error { return nil }
func (q imageRuntimeHealthQueue) Stats(context.Context) (ImageTaskQueueStats, error) {
	if q.statsErr != nil {
		return ImageTaskQueueStats{}, q.statsErr
	}
	return ImageTaskQueueStats{
		Ready:  4,
		Active: 2,
		OldestTask: &ImageTaskRuntimeTask{
			ID:        "imgtask_oldest",
			Status:    ImageTaskStatusQueued,
			CreatedAt: time.Now().UTC().Add(-time.Hour),
		},
	}, nil
}

func TestImageRuntimesHealthGatewayAsyncReportsRedisWorkerAndBacklog(t *testing.T) {
	queue := imageRuntimeHealthQueue{}
	state := NewImageTaskRuntimeState(queue, true, true, true)
	state.SetWorkerRunning(true)
	tasks := &ImageTaskService{runtime: state}
	svc := &ImageRuntimesHealthService{
		cfg: &config.Config{
			ImageStorage: config.ImageStorageConfig{Enabled: true, Backend: "local"},
		},
		imageTask: tasks,
	}

	health := svc.gatewayAsyncHealth(context.Background())

	require.True(t, health.Enabled)
	require.True(t, health.Ready)
	require.Equal(t, "redis", health.Queue)
	require.True(t, health.QueueEnabled)
	require.True(t, health.RedisReady)
	require.True(t, health.WorkerRunning)
	require.EqualValues(t, 4, health.Backlog.Ready)
	require.EqualValues(t, 2, health.Backlog.Active)
	require.Equal(t, "imgtask_oldest", health.OldestTask.ID)
}

func TestImageRuntimesHealthGatewayAsyncReportsCurrentStatsFailure(t *testing.T) {
	queue := imageRuntimeHealthQueue{statsErr: errors.New("redis stats failed")}
	state := NewImageTaskRuntimeState(queue, true, true, true)
	state.SetWorkerRunning(true)
	tasks := &ImageTaskService{runtime: state}
	svc := &ImageRuntimesHealthService{
		cfg:       &config.Config{},
		imageTask: tasks,
	}

	health := svc.gatewayAsyncHealth(context.Background())

	require.False(t, health.Ready)
	require.NotNil(t, health.RecentError)
	require.Equal(t, "redis stats failed", health.RecentError.Message)
}
