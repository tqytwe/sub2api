package repository

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type imageTaskWorkerTestEncryptor struct{}

func (imageTaskWorkerTestEncryptor) Encrypt(value string) (string, error) { return value, nil }
func (imageTaskWorkerTestEncryptor) Decrypt(value string) (string, error) { return value, nil }

type boundedImageTaskProcessor struct {
	tasks   *service.ImageTaskService
	release chan struct{}
	started chan string

	mu        sync.Mutex
	active    int
	maxActive int
	completed int
}

type unexpectedImageTaskProcessor struct {
	mu     sync.Mutex
	called int
}

type cancelAwareImageTaskProcessor struct {
	started  chan struct{}
	canceled chan struct{}
}

func (p *cancelAwareImageTaskProcessor) ProcessImageTask(ctx context.Context, _ string) error {
	close(p.started)
	<-ctx.Done()
	close(p.canceled)
	return ctx.Err()
}

func (p *unexpectedImageTaskProcessor) ProcessImageTask(context.Context, string) error {
	p.mu.Lock()
	p.called++
	p.mu.Unlock()
	return nil
}

func (p *boundedImageTaskProcessor) ProcessImageTask(ctx context.Context, taskID string) error {
	p.mu.Lock()
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.mu.Unlock()
	p.started <- taskID
	<-p.release
	err := p.tasks.Complete(ctx, taskID, 200, json.RawMessage(`{"data":[]}`))
	p.mu.Lock()
	p.active--
	p.completed++
	p.mu.Unlock()
	return err
}

func TestImageTaskWorkerRuntimeBoundsConcurrencyAndCompletesQueuedJobs(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueEnabled:            true,
		WorkerCount:             2,
		QueueReadyKey:           "worker:image:ready",
		QueueActiveKey:          "worker:image:active",
		IdempotencyKeyPrefix:    "worker:image:idem:",
		JobLockKeyPrefix:        "worker:image:lock:",
		ReserveTimeoutSeconds:   1,
		JobLockTTLSeconds:       60,
		HeartbeatSeconds:        1,
		StaleActiveAfterSeconds: 30,
		RecoveryIntervalSeconds: 30,
		RecoverLimit:            100,
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	store := NewImageTaskStore(rdb)
	state := service.NewImageTaskRuntimeState(queue, true, true, true)
	tasks := service.NewQueuedImageTaskService(
		store,
		queue,
		nil,
		imageTaskWorkerTestEncryptor{},
		state,
		time.Hour,
		time.Minute,
	)
	processor := &boundedImageTaskProcessor{
		tasks:   tasks,
		release: make(chan struct{}, 4),
		started: make(chan string, 4),
	}
	runtime := service.NewImageTaskWorkerRuntime(queue, tasks, processor, state, cfg)

	for i := 0; i < 4; i++ {
		task := &service.ImageTaskRecord{
			ID:        "imgtask_worker_" + string(rune('a'+i)),
			Status:    service.ImageTaskStatusQueued,
			CreatedAt: time.Now().Unix(),
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}
		_, created, err := queue.Submit(context.Background(), task, time.Hour, "")
		require.NoError(t, err)
		require.True(t, created)
	}

	runtime.Start()
	t.Cleanup(runtime.Stop)

	<-processor.started
	<-processor.started
	select {
	case taskID := <-processor.started:
		t.Fatalf("third task %s started before a worker slot was released", taskID)
	case <-time.After(150 * time.Millisecond):
	}

	processor.release <- struct{}{}
	processor.release <- struct{}{}
	<-processor.started
	<-processor.started
	processor.release <- struct{}{}
	processor.release <- struct{}{}

	require.Eventually(t, func() bool {
		processor.mu.Lock()
		defer processor.mu.Unlock()
		return processor.completed == 4
	}, 3*time.Second, 20*time.Millisecond)
	processor.mu.Lock()
	require.Equal(t, 2, processor.maxActive)
	processor.mu.Unlock()

	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.Zero(t, stats.Ready)
	require.Zero(t, stats.Active)
}

func TestImageTaskWorkerRuntimeFailsRecoveredProcessingWithoutReplayingUpstream(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueEnabled:            true,
		WorkerCount:             1,
		QueueReadyKey:           "worker:image:unsafe:ready",
		QueueActiveKey:          "worker:image:unsafe:active",
		IdempotencyKeyPrefix:    "worker:image:unsafe:idem:",
		JobLockKeyPrefix:        "worker:image:unsafe:lock:",
		ReserveTimeoutSeconds:   1,
		JobLockTTLSeconds:       60,
		HeartbeatSeconds:        1,
		StaleActiveAfterSeconds: 30,
		RecoveryIntervalSeconds: 30,
		RecoverLimit:            100,
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	store := NewImageTaskStore(rdb)
	state := service.NewImageTaskRuntimeState(queue, true, true, true)
	tasks := service.NewQueuedImageTaskService(
		store,
		queue,
		nil,
		imageTaskWorkerTestEncryptor{},
		state,
		time.Hour,
		time.Minute,
	)
	now := time.Now().UTC().Unix()
	task := &service.ImageTaskRecord{
		ID:          "imgtask_processing_after_restart",
		Status:      service.ImageTaskStatusProcessing,
		Request:     `{"method":"POST","path":"/v1/images/generations","content_type":"application/json","body":"e30="}`,
		StartedAt:   &now,
		HeartbeatAt: &now,
		CreatedAt:   now - 60,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}
	_, created, err := queue.Submit(context.Background(), task, time.Hour, "")
	require.NoError(t, err)
	require.True(t, created)

	processor := &unexpectedImageTaskProcessor{}
	runtime := service.NewImageTaskWorkerRuntime(queue, tasks, processor, state, cfg)
	runtime.Start()
	t.Cleanup(runtime.Stop)

	require.Eventually(t, func() bool {
		got, getErr := store.Get(context.Background(), task.ID)
		return getErr == nil && got.Status == service.ImageTaskStatusFailed
	}, 3*time.Second, 20*time.Millisecond)
	processor.mu.Lock()
	require.Zero(t, processor.called)
	processor.mu.Unlock()
	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Contains(t, string(got.Error), "IMAGE_TASK_RECOVERY_UNAVAILABLE")
}

func TestImageTaskWorkerRuntimeAcksDuplicateTerminalMessageWithoutReplayingUpstream(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueEnabled:            true,
		WorkerCount:             1,
		QueueReadyKey:           "worker:image:terminal:ready",
		QueueActiveKey:          "worker:image:terminal:active",
		IdempotencyKeyPrefix:    "worker:image:terminal:idem:",
		JobLockKeyPrefix:        "worker:image:terminal:lock:",
		ReserveTimeoutSeconds:   1,
		JobLockTTLSeconds:       60,
		HeartbeatSeconds:        1,
		StaleActiveAfterSeconds: 30,
		RecoveryIntervalSeconds: 30,
		RecoverLimit:            100,
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	store := NewImageTaskStore(rdb)
	state := service.NewImageTaskRuntimeState(queue, true, true, true)
	tasks := service.NewQueuedImageTaskService(
		store,
		queue,
		nil,
		imageTaskWorkerTestEncryptor{},
		state,
		time.Hour,
		time.Minute,
	)
	now := time.Now().UTC().Unix()
	task := &service.ImageTaskRecord{
		ID:          "imgtask_terminal_duplicate",
		Status:      service.ImageTaskStatusCompleted,
		Result:      json.RawMessage(`{"data":[]}`),
		CompletedAt: &now,
		CreatedAt:   now - 60,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}
	_, created, err := queue.Submit(context.Background(), task, time.Hour, "")
	require.NoError(t, err)
	require.True(t, created)

	processor := &unexpectedImageTaskProcessor{}
	runtime := service.NewImageTaskWorkerRuntime(queue, tasks, processor, state, cfg)
	runtime.Start()
	t.Cleanup(runtime.Stop)

	require.Eventually(t, func() bool {
		stats, statsErr := queue.Stats(context.Background())
		return statsErr == nil && stats.Ready == 0 && stats.Active == 0
	}, 3*time.Second, 20*time.Millisecond)
	processor.mu.Lock()
	require.Zero(t, processor.called)
	processor.mu.Unlock()
}

func TestImageTaskWorkerRuntimePersistsHeartbeatWhileProcessing(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueEnabled:            true,
		WorkerCount:             1,
		QueueReadyKey:           "worker:image:heartbeat:ready",
		QueueActiveKey:          "worker:image:heartbeat:active",
		IdempotencyKeyPrefix:    "worker:image:heartbeat:idem:",
		JobLockKeyPrefix:        "worker:image:heartbeat:lock:",
		ReserveTimeoutSeconds:   1,
		JobLockTTLSeconds:       60,
		HeartbeatSeconds:        1,
		StaleActiveAfterSeconds: 30,
		RecoveryIntervalSeconds: 30,
		RecoverLimit:            100,
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	store := NewImageTaskStore(rdb)
	state := service.NewImageTaskRuntimeState(queue, true, true, true)
	tasks := service.NewQueuedImageTaskService(
		store,
		queue,
		nil,
		imageTaskWorkerTestEncryptor{},
		state,
		time.Hour,
		time.Minute,
	)
	processor := &boundedImageTaskProcessor{
		tasks:   tasks,
		release: make(chan struct{}, 1),
		started: make(chan string, 1),
	}
	runtime := service.NewImageTaskWorkerRuntime(queue, tasks, processor, state, cfg)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_persistent_heartbeat",
		Status:    service.ImageTaskStatusQueued,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	_, created, err := queue.Submit(context.Background(), task, time.Hour, "")
	require.NoError(t, err)
	require.True(t, created)

	runtime.Start()
	t.Cleanup(runtime.Stop)
	taskID := <-processor.started
	started, err := store.Get(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, started.HeartbeatAt)
	initialHeartbeat := *started.HeartbeatAt

	require.Eventually(t, func() bool {
		current, getErr := store.Get(context.Background(), taskID)
		return getErr == nil && current.HeartbeatAt != nil && *current.HeartbeatAt > initialHeartbeat
	}, 3*time.Second, 50*time.Millisecond)
	processor.release <- struct{}{}
}

func TestImageTaskWorkerRuntimeCancelsExecutionWhenLeaseIsLost(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{ImageAsync: config.ImageAsyncConfig{
		QueueEnabled:            true,
		WorkerCount:             1,
		QueueReadyKey:           "worker:image:lease:ready",
		QueueActiveKey:          "worker:image:lease:active",
		IdempotencyKeyPrefix:    "worker:image:lease:idem:",
		JobLockKeyPrefix:        "worker:image:lease:lock:",
		ReserveTimeoutSeconds:   1,
		JobLockTTLSeconds:       60,
		HeartbeatSeconds:        1,
		StaleActiveAfterSeconds: 30,
		RecoveryIntervalSeconds: 30,
		RecoverLimit:            100,
	}}
	queue := NewImageTaskQueue(rdb, cfg)
	store := NewImageTaskStore(rdb)
	state := service.NewImageTaskRuntimeState(queue, true, true, true)
	tasks := service.NewQueuedImageTaskService(
		store,
		queue,
		nil,
		imageTaskWorkerTestEncryptor{},
		state,
		time.Hour,
		time.Minute,
	)
	processor := &cancelAwareImageTaskProcessor{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	runtime := service.NewImageTaskWorkerRuntime(queue, tasks, processor, state, cfg)
	task := &service.ImageTaskRecord{
		ID:        "imgtask_lease_lost",
		Status:    service.ImageTaskStatusQueued,
		Request:   "encrypted-request-envelope",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	_, created, err := queue.Submit(context.Background(), task, time.Hour, "")
	require.NoError(t, err)
	require.True(t, created)

	runtime.Start()
	t.Cleanup(runtime.Stop)
	<-processor.started
	require.NoError(t, rdb.Set(
		context.Background(),
		cfg.ImageAsync.JobLockKeyPrefix+task.ID,
		"replacement-token",
		time.Minute,
	).Err())

	select {
	case <-processor.canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("processor was not canceled after the queue lease was lost")
	}

	got, err := store.Get(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusProcessing, got.Status)
	require.NotEmpty(t, got.Request)
	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.Zero(t, stats.Ready)
	require.EqualValues(t, 1, stats.Active)

	require.NoError(t, rdb.Del(context.Background(), cfg.ImageAsync.JobLockKeyPrefix+task.ID).Err())
	require.NoError(t, rdb.ZAdd(context.Background(), cfg.ImageAsync.QueueActiveKey, redis.Z{
		Score:  float64(time.Now().Add(-time.Hour).UnixMilli()),
		Member: task.ID,
	}).Err())
	recovered, err := queue.RecoverStaleActive(context.Background(), time.Minute, 10)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)

	require.Eventually(t, func() bool {
		var getErr error
		got, getErr = store.Get(context.Background(), task.ID)
		return getErr == nil && got.Status == service.ImageTaskStatusFailed
	}, 3*time.Second, 20*time.Millisecond)
	require.Contains(t, string(got.Error), "IMAGE_TASK_RECOVERY_UNAVAILABLE")
	require.Empty(t, got.Request)

	stats, err = queue.Stats(context.Background())
	require.NoError(t, err)
	require.Zero(t, stats.Ready)
	require.Zero(t, stats.Active)
}
