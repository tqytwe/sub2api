package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

type ImageTaskProcessor interface {
	ProcessImageTask(ctx context.Context, taskID string) error
}

type ImageTaskWorkerOptions struct {
	WorkerCount      int
	ReserveTimeout   time.Duration
	JobLockTTL       time.Duration
	Heartbeat        time.Duration
	StaleActiveAfter time.Duration
	RecoveryInterval time.Duration
	RecoverLimit     int
}

func ImageTaskWorkerOptionsFromConfig(cfg *config.Config) ImageTaskWorkerOptions {
	opts := ImageTaskWorkerOptions{
		WorkerCount:      4,
		ReserveTimeout:   5 * time.Second,
		JobLockTTL:       5 * time.Minute,
		Heartbeat:        30 * time.Second,
		StaleActiveAfter: 10 * time.Minute,
		RecoveryInterval: time.Minute,
		RecoverLimit:     100,
	}
	if cfg == nil {
		return opts
	}
	if cfg.ImageAsync.WorkerCount > 0 {
		opts.WorkerCount = cfg.ImageAsync.WorkerCount
	}
	if cfg.ImageAsync.ReserveTimeoutSeconds > 0 {
		opts.ReserveTimeout = time.Duration(cfg.ImageAsync.ReserveTimeoutSeconds) * time.Second
	}
	if cfg.ImageAsync.JobLockTTLSeconds > 0 {
		opts.JobLockTTL = time.Duration(cfg.ImageAsync.JobLockTTLSeconds) * time.Second
	}
	if cfg.ImageAsync.HeartbeatSeconds > 0 {
		opts.Heartbeat = time.Duration(cfg.ImageAsync.HeartbeatSeconds) * time.Second
	}
	if cfg.ImageAsync.StaleActiveAfterSeconds > 0 {
		opts.StaleActiveAfter = time.Duration(cfg.ImageAsync.StaleActiveAfterSeconds) * time.Second
	}
	if cfg.ImageAsync.RecoveryIntervalSeconds > 0 {
		opts.RecoveryInterval = time.Duration(cfg.ImageAsync.RecoveryIntervalSeconds) * time.Second
	}
	if cfg.ImageAsync.RecoverLimit > 0 {
		opts.RecoverLimit = cfg.ImageAsync.RecoverLimit
	}
	return opts
}

type ImageTaskWorkerRuntime struct {
	queue     ImageTaskQueue
	tasks     *ImageTaskService
	processor ImageTaskProcessor
	state     *ImageTaskRuntimeState
	opts      ImageTaskWorkerOptions
	enabled   bool

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewImageTaskWorkerRuntime(
	queue ImageTaskQueue,
	tasks *ImageTaskService,
	processor ImageTaskProcessor,
	state *ImageTaskRuntimeState,
	cfg *config.Config,
) *ImageTaskWorkerRuntime {
	return &ImageTaskWorkerRuntime{
		queue:     queue,
		tasks:     tasks,
		processor: processor,
		state:     state,
		opts:      ImageTaskWorkerOptionsFromConfig(cfg),
		enabled:   cfg != nil && cfg.ImageAsync.QueueEnabled,
	}
}

func (r *ImageTaskWorkerRuntime) Start() {
	if r == nil || !r.enabled || r.queue == nil || r.tasks == nil || r.processor == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	if r.state != nil {
		r.state.SetWorkerRunning(true)
	}

	var wg sync.WaitGroup
	for i := 0; i < r.opts.WorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.runWorker(ctx)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.runRecovery(ctx)
	}()
	go func(done chan struct{}) {
		wg.Wait()
		close(done)
	}(r.done)
}

func (r *ImageTaskWorkerRuntime) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	if r.state != nil {
		r.state.SetWorkerRunning(false)
	}
}

func (r *ImageTaskWorkerRuntime) Running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel != nil
}

func (r *ImageTaskWorkerRuntime) runWorker(ctx context.Context) {
	for ctx.Err() == nil {
		taskID, err := r.queue.Reserve(ctx, r.opts.ReserveTimeout)
		if errors.Is(err, ErrImageTaskQueueEmpty) {
			continue
		}
		if err != nil {
			if ctx.Err() == nil {
				r.recordError(err)
				sleepOrDone(ctx, time.Second)
			}
			continue
		}
		r.processReserved(ctx, taskID)
	}
}

func (r *ImageTaskWorkerRuntime) processReserved(ctx context.Context, taskID string) {
	lock, acquired, err := r.queue.TryAcquireJobLock(ctx, taskID, r.opts.JobLockTTL)
	if err != nil {
		r.recordError(err)
		_ = r.queue.Requeue(ctx, taskID)
		return
	}
	if !acquired {
		// A duplicate ready message can race with the worker that already owns
		// this task. Leave its active lease untouched; the owner will Ack it.
		return
	}
	defer func() { _ = lock.Release(context.Background()) }()
	leaseCtx := withImageTaskJobLock(ctx, lock)
	detachedLeaseCtx := func() context.Context {
		return context.WithoutCancel(leaseCtx)
	}

	if err := r.tasks.MarkProcessing(leaseCtx, taskID); err != nil {
		if errors.Is(err, ErrImageTaskAlreadyTerminal) || errors.Is(err, ErrImageTaskNotFound) {
			if ackErr := lock.Ack(context.Background()); ackErr != nil {
				r.recordError(ackErr)
			}
			return
		}
		if errors.Is(err, ErrImageTaskUnsafeResume) {
			failErr := r.tasks.Fail(
				detachedLeaseCtx(),
				taskID,
				500,
				imageTaskErrorCodeJSON(
					"api_error",
					"IMAGE_TASK_RECOVERY_UNAVAILABLE",
					"processing image task could not be safely recovered after restart",
				),
			)
			if failErr != nil {
				r.recordError(failErr)
				return
			}
			if ackErr := lock.Ack(context.Background()); ackErr != nil {
				r.recordError(ackErr)
			}
			return
		}
		r.recordError(err)
		if requeueErr := lock.Requeue(context.Background()); requeueErr != nil {
			r.recordError(requeueErr)
		}
		return
	}

	taskCtx, cancelTask := context.WithCancel(leaseCtx)
	defer cancelTask()
	hbStop := make(chan struct{})
	hbDone := make(chan struct{})
	hbErr := make(chan error, 1)
	go r.runHeartbeat(taskCtx, taskID, lock, cancelTask, hbStop, hbDone, hbErr)
	err = r.processSafely(taskCtx, taskID)
	close(hbStop)
	<-hbDone
	var leaseErr error
	select {
	case leaseErr = <-hbErr:
	default:
	}
	if leaseErr != nil {
		r.recordError(leaseErr)
		return
	}
	if err != nil {
		r.recordError(err)
		if failErr := r.tasks.Fail(
			detachedLeaseCtx(),
			taskID,
			http.StatusInternalServerError,
			imageTaskErrorJSON("api_error", "asynchronous image task execution failed"),
		); failErr != nil {
			r.recordError(failErr)
			return
		}
	}
	if ackErr := lock.Ack(context.Background()); ackErr != nil {
		r.recordError(ackErr)
	}
}

func (r *ImageTaskWorkerRuntime) processSafely(ctx context.Context, taskID string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().Error("image_task.worker_panicked", zap.String("task_id", taskID), zap.Any("panic", recovered))
			err = errors.New("image task worker panicked")
		}
	}()
	return r.processor.ProcessImageTask(ctx, taskID)
}

func (r *ImageTaskWorkerRuntime) runHeartbeat(
	ctx context.Context,
	taskID string,
	lock ImageTaskJobLock,
	cancelTask context.CancelFunc,
	stop <-chan struct{},
	done chan<- struct{},
	result chan<- error,
) {
	defer close(done)
	ticker := time.NewTicker(r.opts.Heartbeat)
	defer ticker.Stop()
	fail := func(err error) {
		select {
		case result <- err:
		default:
		}
		cancelTask()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if err := r.queue.Heartbeat(ctx, taskID); err != nil && ctx.Err() == nil {
				fail(err)
				return
			}
			if err := r.tasks.Heartbeat(ctx, taskID); err != nil && ctx.Err() == nil {
				fail(err)
				return
			}
			if err := lock.Refresh(ctx, r.opts.JobLockTTL); err != nil && ctx.Err() == nil {
				fail(err)
				return
			}
		}
	}
}

func (r *ImageTaskWorkerRuntime) runRecovery(ctx context.Context) {
	if recovery, ok := r.queue.(ImageTaskLegacyRecovery); ok {
		if _, err := recovery.FailUnrecoverableProcessing(ctx, time.Now().UTC(), r.opts.RecoverLimit); err != nil && ctx.Err() == nil {
			r.recordError(err)
		}
	}
	ticker := time.NewTicker(r.opts.RecoveryInterval)
	defer ticker.Stop()
	for {
		if _, err := r.queue.RecoverStaleActive(ctx, r.opts.StaleActiveAfter, r.opts.RecoverLimit); err != nil && ctx.Err() == nil {
			r.recordError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *ImageTaskWorkerRuntime) recordError(err error) {
	if r.state != nil {
		r.state.RecordError(err)
	}
}
