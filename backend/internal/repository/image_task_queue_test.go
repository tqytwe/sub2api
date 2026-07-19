package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newImageTaskQueueTestHarness(t *testing.T) (*miniredis.Miniredis, *redis.Client, service.ImageTaskQueue) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueReadyKey:        "test:image:ready",
		QueueActiveKey:       "test:image:active",
		IdempotencyKeyPrefix: "test:image:idem:",
		JobLockKeyPrefix:     "test:image:lock:",
	}}
	return mr, rdb, NewImageTaskQueue(rdb, cfg)
}

func TestImageTaskQueueSubmitReserveHeartbeatAndAck(t *testing.T) {
	_, _, queue := newImageTaskQueueTestHarness(t)
	task := &service.ImageTaskRecord{
		ID:          "imgtask_queue1",
		UserID:      7,
		APIKeyID:    9,
		Status:      service.ImageTaskStatusQueued,
		RequestHash: "hash-1",
		CreatedAt:   time.Now().Unix(),
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}

	taskID, created, err := queue.Submit(context.Background(), task, time.Hour, "7:9:request-1")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, task.ID, taskID)

	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Ready)

	reserved, err := queue.Reserve(context.Background(), time.Second)
	require.NoError(t, err)
	require.Equal(t, task.ID, reserved)
	require.NoError(t, queue.Heartbeat(context.Background(), task.ID))

	stats, err = queue.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 0, stats.Ready)
	require.EqualValues(t, 1, stats.Active)

	require.NoError(t, queue.Ack(context.Background(), task.ID))
	stats, err = queue.Stats(context.Background())
	require.NoError(t, err)
	require.Zero(t, stats.Active)
}

func TestImageTaskQueueIdempotencyReplayAndConflict(t *testing.T) {
	_, _, queue := newImageTaskQueueTestHarness(t)
	first := &service.ImageTaskRecord{
		ID:          "imgtask_first",
		Status:      service.ImageTaskStatusQueued,
		RequestHash: "same-hash",
	}
	taskID, created, err := queue.Submit(context.Background(), first, time.Hour, "7:9:idem")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, first.ID, taskID)

	replay := *first
	replay.ID = "imgtask_replay"
	taskID, created, err = queue.Submit(context.Background(), &replay, time.Hour, "7:9:idem")
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, taskID)

	conflict := replay
	conflict.ID = "imgtask_conflict"
	conflict.RequestHash = "different-hash"
	_, _, err = queue.Submit(context.Background(), &conflict, time.Hour, "7:9:idem")
	require.ErrorIs(t, err, service.ErrImageTaskIdempotency)

	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Ready)
}

func TestImageTaskQueueIdempotencyRecreatesExpiredTaskAndClearsStaleActiveEntry(t *testing.T) {
	_, rdb, queue := newImageTaskQueueTestHarness(t)
	first := &service.ImageTaskRecord{
		ID:          "imgtask_expired_first",
		Status:      service.ImageTaskStatusQueued,
		RequestHash: "same-hash",
	}
	taskID, created, err := queue.Submit(context.Background(), first, time.Hour, "7:9:expired-idem")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, first.ID, taskID)

	reserved, err := queue.Reserve(context.Background(), time.Second)
	require.NoError(t, err)
	require.Equal(t, first.ID, reserved)
	require.NoError(t, rdb.Del(context.Background(), imageTaskKey(first.ID)).Err())

	replacement := *first
	replacement.ID = "imgtask_expired_replacement"
	taskID, created, err = queue.Submit(context.Background(), &replacement, time.Hour, "7:9:expired-idem")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, replacement.ID, taskID)

	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Ready)
	require.Zero(t, stats.Active)

	reserved, err = queue.Reserve(context.Background(), time.Second)
	require.NoError(t, err)
	require.Equal(t, replacement.ID, reserved)
}

func TestImageTaskQueueRecoversStaleActive(t *testing.T) {
	_, rdb, queue := newImageTaskQueueTestHarness(t)
	task := &service.ImageTaskRecord{ID: "imgtask_stale", Status: service.ImageTaskStatusQueued, RequestHash: "hash"}
	_, _, err := queue.Submit(context.Background(), task, time.Hour, "")
	require.NoError(t, err)
	_, err = queue.Reserve(context.Background(), time.Second)
	require.NoError(t, err)

	require.NoError(t, rdb.ZAdd(context.Background(), "test:image:active", redis.Z{
		Score:  float64(time.Now().Add(-11 * time.Minute).UnixMilli()),
		Member: task.ID,
	}).Err())
	recovered, err := queue.RecoverStaleActive(context.Background(), 10*time.Minute, 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)

	reserved, err := queue.Reserve(context.Background(), time.Second)
	require.NoError(t, err)
	require.Equal(t, task.ID, reserved)
}

func TestImageTaskQueueReportsLostLease(t *testing.T) {
	_, rdb, queue := newImageTaskQueueTestHarness(t)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_lost_lease",
		Status:    service.ImageTaskStatusQueued,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	_, _, err := queue.Submit(context.Background(), task, time.Hour, "")
	require.NoError(t, err)
	_, err = queue.Reserve(context.Background(), time.Second)
	require.NoError(t, err)

	require.NoError(t, rdb.ZRem(context.Background(), "test:image:active", task.ID).Err())
	require.ErrorIs(t, queue.Heartbeat(context.Background(), task.ID), service.ErrImageTaskLeaseLost)

	lock, acquired, err := queue.TryAcquireJobLock(context.Background(), task.ID, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, rdb.Del(context.Background(), "test:image:lock:"+task.ID).Err())
	require.ErrorIs(t, lock.Refresh(context.Background(), time.Minute), service.ErrImageTaskLeaseLost)
}

func TestImageTaskQueueStaleLockCannotWriteAckOrRequeue(t *testing.T) {
	_, rdb, queue := newImageTaskQueueTestHarness(t)
	ctx := context.Background()
	task := &service.ImageTaskRecord{
		ID:        "imgtask_stale_fence",
		Status:    service.ImageTaskStatusQueued,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	_, created, err := queue.Submit(ctx, task, time.Hour, "")
	require.NoError(t, err)
	require.True(t, created)
	_, err = queue.Reserve(ctx, time.Second)
	require.NoError(t, err)
	lock, acquired, err := queue.TryAcquireJobLock(ctx, task.ID, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	require.NoError(t, rdb.Set(ctx, "test:image:lock:"+task.ID, "replacement-token", time.Minute).Err())
	terminal := *task
	terminal.Status = service.ImageTaskStatusCompleted
	terminal.Result = json.RawMessage(`{"data":[]}`)

	saved, err := lock.SaveIfStatus(ctx, &terminal, service.ImageTaskStatusQueued, time.Hour)
	require.ErrorIs(t, err, service.ErrImageTaskLeaseLost)
	require.False(t, saved)
	require.ErrorIs(t, lock.Ack(ctx), service.ErrImageTaskLeaseLost)
	require.ErrorIs(t, lock.Requeue(ctx), service.ErrImageTaskLeaseLost)

	got, err := NewImageTaskStore(rdb).Get(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusQueued, got.Status)
	require.NoError(t, rdb.ZScore(ctx, "test:image:active", task.ID).Err())
	require.Zero(t, rdb.LLen(ctx, "test:image:ready").Val())
}

func TestImageTaskQueueStatsIncludesOldestTask(t *testing.T) {
	_, _, queue := newImageTaskQueueTestHarness(t)
	older := &service.ImageTaskRecord{
		ID:        "imgtask_oldest",
		Status:    service.ImageTaskStatusQueued,
		CreatedAt: time.Now().Add(-time.Hour).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	newer := &service.ImageTaskRecord{
		ID:        "imgtask_newer",
		Status:    service.ImageTaskStatusQueued,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	_, _, err := queue.Submit(context.Background(), older, time.Hour, "")
	require.NoError(t, err)
	_, _, err = queue.Submit(context.Background(), newer, time.Hour, "")
	require.NoError(t, err)

	stats, err := queue.Stats(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stats.OldestTask)
	require.Equal(t, older.ID, stats.OldestTask.ID)
	require.Equal(t, service.ImageTaskStatusQueued, stats.OldestTask.Status)
	require.Equal(t, time.Unix(older.CreatedAt, 0).UTC(), stats.OldestTask.CreatedAt)
}

func TestImageTaskQueueFailsLegacyProcessingAfterRestart(t *testing.T) {
	_, rdb, queue := newImageTaskQueueTestHarness(t)
	store := NewImageTaskStore(rdb)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_legacy",
		Status:    service.ImageTaskStatusProcessing,
		CreatedAt: time.Now().Add(-time.Hour).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Save(context.Background(), task, time.Hour))

	recovery, ok := queue.(service.ImageTaskLegacyRecovery)
	require.True(t, ok)
	failed, err := recovery.FailUnrecoverableProcessing(context.Background(), time.Now().Add(-time.Minute), 10)
	require.NoError(t, err)
	require.Equal(t, 1, failed)

	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusFailed, got.Status)
	require.Contains(t, string(got.Error), "could not be safely recovered")
}
