package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestImageTaskStoreRoundTripAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_123",
		UserID:    7,
		APIKeyID:  9,
		Status:    service.ImageTaskStatusProcessing,
		CreatedAt: 100,
		ExpiresAt: 200,
	}

	require.NoError(t, store.Save(context.Background(), task, 24*time.Hour))
	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, task, got)
	require.Equal(t, 24*time.Hour, mr.TTL(imageTaskKey(task.ID)))
}

func TestImageTaskStoreMissing(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)

	_, err := store.Get(context.Background(), "imgtask_missing")
	require.ErrorIs(t, err, service.ErrImageTaskNotFound)
}

func TestImageTaskStoreTouchHeartbeatOnlyUpdatesProcessingTask(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)
	startedAt := time.Now().UTC().Add(-time.Minute).Unix()
	task := &service.ImageTaskRecord{
		ID:          "imgtask_heartbeat",
		Status:      service.ImageTaskStatusProcessing,
		StartedAt:   &startedAt,
		HeartbeatAt: &startedAt,
		CreatedAt:   startedAt,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Save(context.Background(), task, time.Hour))

	heartbeatAt := time.Now().UTC()
	require.NoError(t, store.TouchHeartbeat(context.Background(), task.ID, heartbeatAt))
	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, heartbeatAt.Unix(), *got.HeartbeatAt)

	got.Status = service.ImageTaskStatusCompleted
	require.NoError(t, store.Save(context.Background(), got, time.Hour))
	later := heartbeatAt.Add(time.Minute)
	require.NoError(t, store.TouchHeartbeat(context.Background(), task.ID, later))
	got, err = store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusCompleted, got.Status)
	require.Equal(t, heartbeatAt.Unix(), *got.HeartbeatAt)
}

func TestImageTaskStoreSaveIfStatusIsAtomic(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_cas",
		Status:    service.ImageTaskStatusProcessing,
		Request:   "encrypted-request",
		CreatedAt: time.Now().Add(-time.Minute).Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, store.Save(context.Background(), task, time.Hour))

	completed := *task
	completed.Status = service.ImageTaskStatusCompleted
	completed.Request = ""
	saved, err := store.SaveIfStatus(context.Background(), &completed, service.ImageTaskStatusProcessing, time.Hour)
	require.NoError(t, err)
	require.True(t, saved)

	failed := *task
	failed.Status = service.ImageTaskStatusFailed
	saved, err = store.SaveIfStatus(context.Background(), &failed, service.ImageTaskStatusProcessing, time.Hour)
	require.NoError(t, err)
	require.False(t, saved)

	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusCompleted, got.Status)
	require.Empty(t, got.Request)
}
