package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	liveSettlementBatchSize    = 32
	liveSettlementPollInterval = time.Second
	liveSettlementClaimLease   = 30 * time.Second
	liveSettlementDBTimeout    = 5 * time.Second
)

// liveSettlementWorker is deliberately owned by OpenAIGatewayService: it uses
// the same authoritative RecordUsage path as normal gateway requests. A row
// may safely be claimed by another process after its lease expires.
type liveSettlementWorker struct {
	service  *OpenAIGatewayService
	repo     LiveSettlementOutboxRepository
	workerID string
	ctx      context.Context
	cancel   context.CancelFunc
	start    sync.Once
	stop     sync.Once
	wg       sync.WaitGroup
	running  atomic.Bool
}

func newLiveSettlementWorker(service *OpenAIGatewayService, repo LiveSettlementOutboxRepository) *liveSettlementWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &liveSettlementWorker{
		service: service, repo: repo, workerID: uuid.NewString(), ctx: ctx, cancel: cancel,
	}
}

func (w *liveSettlementWorker) Start() {
	if w == nil || w.service == nil || w.repo == nil {
		return
	}
	w.start.Do(func() {
		w.running.Store(true)
		w.wg.Add(1)
		go w.run()
	})
}

func (w *liveSettlementWorker) Stop() {
	if w == nil {
		return
	}
	w.stop.Do(func() {
		w.cancel()
		w.wg.Wait()
		w.running.Store(false)
	})
}

func (w *liveSettlementWorker) run() {
	defer w.wg.Done()
	defer w.running.Store(false)
	ticker := time.NewTicker(liveSettlementPollInterval)
	defer ticker.Stop()
	for {
		w.service.recoverLiveSettlementClosures(w.ctx)
		w.service.processLiveSettlementBatch(w.ctx)
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// recoverLiveSettlementClosures bridges the unavoidable cross-store window:
// the finalization intent is written in PostgreSQL before the Redis call is
// closed. If the process dies before activation, a restarted worker checks
// Redis and makes the job billable only after it observes "closed". If Redis
// has expired the record, the conservative fallback waits for ExpiresAt.
func (s *OpenAIGatewayService) recoverLiveSettlementClosures(ctx context.Context) {
	if s == nil || s.liveSettlementOutbox == nil {
		return
	}
	queryCtx, cancel := context.WithTimeout(ctx, liveSettlementDBTimeout)
	jobs, err := s.liveSettlementOutbox.ListClosing(queryCtx, liveSettlementBatchSize)
	cancel()
	if err != nil {
		logger.L().Warn("openai_live.settlement_recovery_list_failed", zap.Error(err))
		return
	}
	for _, job := range jobs {
		if job.Record == nil || job.CallHash == "" {
			continue
		}
		if !time.Now().Before(job.Record.ExpiresAt) {
			s.activateLiveSettlement(job.CallHash)
			continue
		}
		store, storeErr := s.liveStore()
		if storeErr != nil {
			continue
		}
		storeCtx, storeCancel := context.WithTimeout(ctx, liveRedisOperationTimeout)
		latest, loadErr := store.GetLiveCall(storeCtx, job.CallHash)
		storeCancel()
		if loadErr != nil {
			// A missing non-expired Redis record might be a transient eviction or a
			// manual cleanup. Do not settle early; expiry remains the safe boundary.
			continue
		}
		if latest.Controller != LiveControllerClosed {
			continue
		}
		// Redis may contain usage received after the initial close intent was
		// staged. Refresh only a still-closing row before activation.
		enqueueCtx, enqueueCancel := context.WithTimeout(ctx, liveSettlementDBTimeout)
		enqueueErr := s.liveSettlementOutbox.Enqueue(enqueueCtx, latest)
		enqueueCancel()
		if enqueueErr != nil {
			logger.L().Warn("openai_live.settlement_recovery_refresh_failed", zap.String("call_hash", job.CallHash), zap.Error(enqueueErr))
			continue
		}
		s.activateLiveSettlement(job.CallHash)
	}
}

func (s *OpenAIGatewayService) activateLiveSettlement(callHash string) {
	if s == nil || s.liveSettlementOutbox == nil || callHash == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), liveSettlementDBTimeout)
	err := s.liveSettlementOutbox.Activate(ctx, callHash)
	cancel()
	if err != nil {
		logger.L().Warn("openai_live.settlement_activation_failed", zap.String("call_hash", callHash), zap.Error(err))
	}
}

func (s *OpenAIGatewayService) processLiveSettlementBatch(ctx context.Context) {
	if s == nil || s.liveSettlementOutbox == nil {
		return
	}
	workerID := "inline-" + uuid.NewString()
	if s.liveSettlementWorker != nil {
		workerID = s.liveSettlementWorker.workerID
	}
	claimCtx, cancel := context.WithTimeout(ctx, liveSettlementDBTimeout)
	jobs, err := s.liveSettlementOutbox.Claim(claimCtx, workerID, liveSettlementBatchSize, liveSettlementClaimLease)
	cancel()
	if err != nil {
		logger.L().Warn("openai_live.settlement_claim_failed", zap.Error(err))
		return
	}
	for _, job := range jobs {
		s.processLiveSettlementJob(ctx, workerID, job)
	}
}

func (s *OpenAIGatewayService) processLiveSettlementJob(ctx context.Context, workerID string, job LiveSettlementJob) {
	if job.Record == nil {
		s.retryLiveSettlementJob(ctx, workerID, job, errors.New("live settlement row has no record"))
		return
	}
	if err := s.settleLiveCall(job.Record); err != nil {
		s.retryLiveSettlementJob(ctx, workerID, job, err)
		return
	}
	ackCtx, ackCancel := context.WithTimeout(ctx, liveSettlementDBTimeout)
	err := s.liveSettlementOutbox.Ack(ackCtx, job.ID, workerID)
	ackCancel()
	if err != nil {
		logger.L().Warn("openai_live.settlement_ack_failed", zap.Int64("job_id", job.ID), zap.Error(err))
		return
	}
	s.releaseLiveLease(job.Record.AccountID, job.Record.UserID, job.Record.APIKeyID, job.Record.LeaseID)
}

func (s *OpenAIGatewayService) retryLiveSettlementJob(ctx context.Context, workerID string, job LiveSettlementJob, cause error) {
	if s == nil || s.liveSettlementOutbox == nil {
		return
	}
	retryAt := time.Now().UTC().Add(liveSettlementRetryDelay(job.Attempts + 1))
	retryCtx, cancel := context.WithTimeout(ctx, liveSettlementDBTimeout)
	err := s.liveSettlementOutbox.Retry(retryCtx, job.ID, workerID, retryAt, boundedLiveSettlementError(cause))
	cancel()
	if err != nil {
		logger.L().Warn("openai_live.settlement_retry_schedule_failed", zap.Int64("job_id", job.ID), zap.Error(errors.Join(cause, err)))
		return
	}
	logger.L().Warn("openai_live.settlement_failed", zap.String("call_hash", job.CallHash), zap.Int("attempt", job.Attempts+1), zap.Error(cause))
}

func liveSettlementRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Second * time.Duration(1<<(attempt-1))
}

func boundedLiveSettlementError(err error) string {
	if err == nil {
		return ""
	}
	message := fmt.Sprintf("%v", err)
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}
