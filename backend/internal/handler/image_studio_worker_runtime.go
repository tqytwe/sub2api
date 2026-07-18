package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type imageStudioWorkerService interface {
	ClaimNextJob(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*service.ImageStudioJob, error)
	HeartbeatJob(ctx context.Context, jobID, leaseOwner string, now time.Time, leaseDuration time.Duration) error
	DecryptJobRequest(job *service.ImageStudioJob) (string, error)
	ClaimNextItem(ctx context.Context, jobID, leaseOwner string, now time.Time) (*service.ImageStudioItem, error)
	RetryWorkerItem(ctx context.Context, job *service.ImageStudioJob, item *service.ImageStudioItem, leaseOwner string, itemErr error, now time.Time) error
	CheckpointWorkerItem(ctx context.Context, job *service.ImageStudioJob, item *service.ImageStudioItem, leaseOwner string, image *service.ImageStudioImagePayload, actualCost float64, now time.Time) error
	CompleteWorkerItem(ctx context.Context, job *service.ImageStudioJob, item *service.ImageStudioItem, leaseOwner string, image *service.ImageStudioImagePayload, actualCost float64, itemErr error, now time.Time) error
	SettleJob(ctx context.Context, jobID, leaseOwner string, now time.Time) (*service.ImageStudioJob, error)
}

type ImageStudioItemProcessor func(
	ctx context.Context,
	job *service.ImageStudioJob,
	item *service.ImageStudioItem,
	requestBody string,
) (*service.ImageStudioImagePayload, float64, error)

type ImageStudioWorkerRuntimeOptions struct {
	WorkerCount       int
	PollInterval      time.Duration
	LeaseDuration     time.Duration
	HeartbeatInterval time.Duration
	Owner             string
}

type ImageStudioWorkerRuntime struct {
	studio    imageStudioWorkerService
	processor ImageStudioItemProcessor
	opts      ImageStudioWorkerRuntimeOptions

	ctx       context.Context
	cancel    context.CancelFunc
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	running   atomic.Bool
}

func NewImageStudioWorkerRuntime(
	studio imageStudioWorkerService,
	processor ImageStudioItemProcessor,
	opts ImageStudioWorkerRuntimeOptions,
) *ImageStudioWorkerRuntime {
	if opts.WorkerCount <= 0 {
		opts.WorkerCount = 4
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 500 * time.Millisecond
	}
	if opts.LeaseDuration <= 0 {
		opts.LeaseDuration = 45 * time.Second
	}
	if opts.HeartbeatInterval <= 0 || opts.HeartbeatInterval >= opts.LeaseDuration {
		opts.HeartbeatInterval = opts.LeaseDuration / 3
	}
	if opts.Owner == "" {
		opts.Owner = "image-studio-" + uuid.NewString()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ImageStudioWorkerRuntime{
		studio:    studio,
		processor: processor,
		opts:      opts,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (r *ImageStudioWorkerRuntime) Start() {
	if r == nil || r.studio == nil || r.processor == nil {
		return
	}
	r.startOnce.Do(func() {
		r.running.Store(true)
		for i := 0; i < r.opts.WorkerCount; i++ {
			r.wg.Add(1)
			go r.workerLoop(i)
		}
	})
}

func (r *ImageStudioWorkerRuntime) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		r.cancel()
		r.wg.Wait()
		r.running.Store(false)
	})
}

func (r *ImageStudioWorkerRuntime) Running() bool {
	return r != nil && r.running.Load()
}

func (r *ImageStudioWorkerRuntime) workerLoop(workerIndex int) {
	defer r.wg.Done()
	leaseOwner := fmt.Sprintf("%s:%d", r.opts.Owner, workerIndex)
	for {
		if err := r.ctx.Err(); err != nil {
			return
		}
		job, err := r.studio.ClaimNextJob(r.ctx, leaseOwner, time.Now().UTC(), r.opts.LeaseDuration)
		if err != nil {
			if r.waitForPoll() {
				return
			}
			continue
		}
		if job == nil {
			if r.waitForPoll() {
				return
			}
			continue
		}
		if err := r.processJob(r.ctx, leaseOwner, job); err != nil && r.ctx.Err() == nil {
			logger.L().Warn("image_studio.worker_job_failed",
				zap.String("job_id", job.ID),
				zap.String("lease_owner", leaseOwner),
				zap.Error(err),
			)
		}
	}
}

func (r *ImageStudioWorkerRuntime) waitForPoll() bool {
	timer := time.NewTimer(r.opts.PollInterval)
	defer timer.Stop()
	select {
	case <-r.ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

func (r *ImageStudioWorkerRuntime) processJob(parent context.Context, leaseOwner string, job *service.ImageStudioJob) error {
	jobCtx, cancel := context.WithCancel(parent)
	defer cancel()
	heartbeatDone := make(chan struct{})
	heartbeatErr := make(chan error, 1)
	go r.heartbeatLoop(jobCtx, cancel, leaseOwner, job.ID, heartbeatDone, heartbeatErr)
	defer func() {
		cancel()
		<-heartbeatDone
	}()

	var singleBody string
	requestReady := false
	for {
		if err := jobCtx.Err(); err != nil {
			select {
			case leaseErr := <-heartbeatErr:
				return leaseErr
			default:
				return err
			}
		}
		item, err := r.studio.ClaimNextItem(jobCtx, job.ID, leaseOwner, time.Now().UTC())
		if err != nil {
			return err
		}
		if item == nil {
			break
		}
		var image *service.ImageStudioImagePayload
		var actualCost float64
		var itemErr error
		if item.Status == service.ImageStudioItemStatusPersisting {
			image = &service.ImageStudioImagePayload{
				Data:        append([]byte(nil), item.CheckpointData...),
				ContentType: item.CheckpointContentType,
			}
			if item.CheckpointActualCost != nil {
				actualCost = *item.CheckpointActualCost
			}
			if len(image.Data) == 0 {
				return fmt.Errorf("image studio checkpoint is empty for item %s", item.ID)
			}
		} else {
			if item.AttemptCount > 1 && !service.ImageStudioProviderSupportsAutomaticRetry(job.Model) {
				itemErr = fmt.Errorf("automatic retry is disabled for provider request without a durable checkpoint")
				if err := r.studio.CompleteWorkerItem(
					jobCtx,
					job,
					item,
					leaseOwner,
					nil,
					0,
					itemErr,
					time.Now().UTC(),
				); err != nil {
					return err
				}
				continue
			}
			if !requestReady {
				body, err := r.studio.DecryptJobRequest(job)
				if err != nil {
					if completeErr := r.studio.CompleteWorkerItem(
						jobCtx,
						job,
						item,
						leaseOwner,
						nil,
						0,
						err,
						time.Now().UTC(),
					); completeErr != nil {
						return completeErr
					}
					return r.failRemainingItemsAndSettle(jobCtx, leaseOwner, job, err)
				}
				singleBody, err = imageStudioSingleImageRequestBody(body)
				if err != nil {
					if completeErr := r.studio.CompleteWorkerItem(
						jobCtx,
						job,
						item,
						leaseOwner,
						nil,
						0,
						err,
						time.Now().UTC(),
					); completeErr != nil {
						return completeErr
					}
					return r.failRemainingItemsAndSettle(jobCtx, leaseOwner, job, err)
				}
				requestReady = true
			}
			image, actualCost, itemErr = r.processor(jobCtx, job, item, singleBody)
			if itemErr == nil && image == nil {
				itemErr = errorsNewImageStudioEmptyOutput()
			}
			if itemErr != nil &&
				item.AttemptCount < service.ImageStudioMaxItemAttempts &&
				service.ImageStudioProviderSupportsAutomaticRetry(job.Model) {
				if err := r.studio.RetryWorkerItem(
					jobCtx,
					job,
					item,
					leaseOwner,
					itemErr,
					time.Now().UTC(),
				); err != nil {
					return err
				}
				continue
			}
			if itemErr == nil {
				if err := r.studio.CheckpointWorkerItem(
					jobCtx,
					job,
					item,
					leaseOwner,
					image,
					actualCost,
					time.Now().UTC(),
				); err != nil {
					if errors.Is(err, service.ErrImageStudioCheckpointCancelled) {
						continue
					}
					return err
				}
			}
		}
		if err := r.studio.CompleteWorkerItem(
			jobCtx,
			job,
			item,
			leaseOwner,
			image,
			actualCost,
			itemErr,
			time.Now().UTC(),
		); err != nil {
			return err
		}
	}
	_, err := r.studio.SettleJob(jobCtx, job.ID, leaseOwner, time.Now().UTC())
	return err
}

func (r *ImageStudioWorkerRuntime) failRemainingItemsAndSettle(
	ctx context.Context,
	leaseOwner string,
	job *service.ImageStudioJob,
	cause error,
) error {
	for {
		item, err := r.studio.ClaimNextItem(ctx, job.ID, leaseOwner, time.Now().UTC())
		if err != nil {
			return err
		}
		if item == nil {
			break
		}
		if item.Status == service.ImageStudioItemStatusPersisting {
			image := &service.ImageStudioImagePayload{
				Data:        append([]byte(nil), item.CheckpointData...),
				ContentType: item.CheckpointContentType,
			}
			actualCost := 0.0
			if item.CheckpointActualCost != nil {
				actualCost = *item.CheckpointActualCost
			}
			if len(image.Data) == 0 {
				return fmt.Errorf("image studio checkpoint is empty for item %s", item.ID)
			}
			if err := r.studio.CompleteWorkerItem(
				ctx,
				job,
				item,
				leaseOwner,
				image,
				actualCost,
				nil,
				time.Now().UTC(),
			); err != nil {
				return err
			}
			continue
		}
		if err := r.studio.CompleteWorkerItem(
			ctx,
			job,
			item,
			leaseOwner,
			nil,
			0,
			cause,
			time.Now().UTC(),
		); err != nil {
			return err
		}
	}
	_, err := r.studio.SettleJob(ctx, job.ID, leaseOwner, time.Now().UTC())
	return err
}

func (r *ImageStudioWorkerRuntime) heartbeatLoop(
	ctx context.Context,
	cancel context.CancelFunc,
	leaseOwner, jobID string,
	done chan<- struct{},
	errCh chan<- error,
) {
	defer close(done)
	ticker := time.NewTicker(r.opts.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := r.studio.HeartbeatJob(ctx, jobID, leaseOwner, now.UTC(), r.opts.LeaseDuration); err != nil {
				select {
				case errCh <- err:
				default:
				}
				cancel()
				return
			}
		}
	}
}

func errorsNewImageStudioEmptyOutput() error {
	return fmt.Errorf("image studio gateway returned no image")
}
