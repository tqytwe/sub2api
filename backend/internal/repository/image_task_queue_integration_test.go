//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImageTaskQueueAndStoreUseRealRedisLeasesAndCAS(t *testing.T) {
	ctx := context.Background()
	rdb := integrationRedis
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	prefix := "it:image_task:" + suffix + ":"
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueReadyKey:        prefix + "ready",
		QueueActiveKey:       prefix + "active",
		IdempotencyKeyPrefix: prefix + "idem:",
		JobLockKeyPrefix:     prefix + "lock:",
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	store := NewImageTaskStore(rdb)

	older := &service.ImageTaskRecord{
		ID:        "imgtask_integration_older_" + suffix,
		Status:    service.ImageTaskStatusQueued,
		Request:   "encrypted-request",
		CreatedAt: time.Now().UTC().Add(-time.Hour).Unix(),
		ExpiresAt: time.Now().UTC().Add(time.Hour).Unix(),
	}
	newer := &service.ImageTaskRecord{
		ID:        "imgtask_integration_newer_" + suffix,
		Status:    service.ImageTaskStatusQueued,
		Request:   "encrypted-request",
		CreatedAt: time.Now().UTC().Unix(),
		ExpiresAt: time.Now().UTC().Add(time.Hour).Unix(),
	}
	t.Cleanup(func() {
		_ = rdb.Unlink(
			context.Background(),
			cfg.ImageAsync.QueueReadyKey,
			cfg.ImageAsync.QueueActiveKey,
			imageTaskKey(older.ID),
			imageTaskKey(newer.ID),
		).Err()
	})
	for _, task := range []*service.ImageTaskRecord{older, newer} {
		_, created, err := queue.Submit(ctx, task, time.Hour, "")
		require.NoError(t, err)
		require.True(t, created)
	}

	stats, err := queue.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.Ready)
	require.NotNil(t, stats.OldestTask)
	require.Equal(t, older.ID, stats.OldestTask.ID)

	taskID, err := queue.Reserve(ctx, time.Second)
	require.NoError(t, err)
	require.Equal(t, older.ID, taskID)
	require.NoError(t, queue.Heartbeat(ctx, taskID))

	processing := *older
	processing.Status = service.ImageTaskStatusProcessing
	saved, err := store.SaveIfStatus(ctx, &processing, service.ImageTaskStatusQueued, time.Hour)
	require.NoError(t, err)
	require.True(t, saved)

	start := make(chan struct{})
	results := make(chan bool, 2)
	var wg sync.WaitGroup
	for _, status := range []string{service.ImageTaskStatusCompleted, service.ImageTaskStatusFailed} {
		status := status
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			terminal := processing
			terminal.Status = status
			terminal.Request = ""
			if status == service.ImageTaskStatusCompleted {
				terminal.Result = json.RawMessage(`{"data":[]}`)
			} else {
				terminal.Error = json.RawMessage(`{"type":"api_error"}`)
			}
			won, saveErr := store.SaveIfStatus(ctx, &terminal, service.ImageTaskStatusProcessing, time.Hour)
			require.NoError(t, saveErr)
			results <- won
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	winners := 0
	for won := range results {
		if won {
			winners++
		}
	}
	require.Equal(t, 1, winners)
	got, err := store.Get(ctx, older.ID)
	require.NoError(t, err)
	require.Contains(t, []string{service.ImageTaskStatusCompleted, service.ImageTaskStatusFailed}, got.Status)
	require.Empty(t, got.Request)

	require.NoError(t, rdb.ZRem(ctx, cfg.ImageAsync.QueueActiveKey, older.ID).Err())
	require.ErrorIs(t, queue.Heartbeat(ctx, older.ID), service.ErrImageTaskLeaseLost)
}

func TestImageTaskQueueRealRedisReplacesExpiredIdempotentTask(t *testing.T) {
	ctx := context.Background()
	rdb := integrationRedis
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	prefix := "it:image_task_idem:" + suffix + ":"
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueReadyKey:        prefix + "ready",
		QueueActiveKey:       prefix + "active",
		IdempotencyKeyPrefix: prefix + "idem:",
		JobLockKeyPrefix:     prefix + "lock:",
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	first := &service.ImageTaskRecord{
		ID:          "imgtask_integration_expired_" + suffix,
		Status:      service.ImageTaskStatusQueued,
		RequestHash: "same-request",
		CreatedAt:   time.Now().UTC().Unix(),
		ExpiresAt:   time.Now().UTC().Add(time.Hour).Unix(),
	}
	replacement := *first
	replacement.ID = "imgtask_integration_replacement_" + suffix
	idempotencyKey := "owner:request"
	t.Cleanup(func() {
		_ = rdb.Unlink(
			context.Background(),
			cfg.ImageAsync.QueueReadyKey,
			cfg.ImageAsync.QueueActiveKey,
			cfg.ImageAsync.IdempotencyKeyPrefix+idempotencyKey,
			imageTaskKey(first.ID),
			imageTaskKey(replacement.ID),
		).Err()
	})

	taskID, created, err := queue.Submit(ctx, first, time.Hour, idempotencyKey)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, first.ID, taskID)
	reserved, err := queue.Reserve(ctx, time.Second)
	require.NoError(t, err)
	require.Equal(t, first.ID, reserved)
	require.NoError(t, rdb.Del(ctx, imageTaskKey(first.ID)).Err())

	taskID, created, err = queue.Submit(ctx, &replacement, time.Hour, idempotencyKey)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, replacement.ID, taskID)
	stats, err := queue.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Ready)
	require.Zero(t, stats.Active)
}
