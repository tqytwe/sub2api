package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/qualityqueue"
)

const quality5xxSource = "upstream_5xx"

// The queue contains account IDs and observation timestamps only. Actions and
// results continue through the existing atomic quality-round transaction.
type quality5xxTrigger struct {
	queue qualityqueue.Queue
}

func (s *quality5xxTrigger) signalIsCurrent(ctx context.Context, accountID int64, observedAt time.Time) bool {
	if s == nil || s.queue == nil {
		return true
	}
	checkCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	score, err := s.queue.Score(checkCtx, accountID)
	return err == nil && !score.After(observedAt)
}

func (s *quality5xxTrigger) Observe(ctx context.Context, account *Account, status int) {
	if s == nil || s.queue == nil || account == nil || account.ID <= 0 || account.Type != AccountTypeOAuth || status < 500 || status > 599 || ctx.Value(qualityIsolationProbeKey{}) != nil {
		return
	}
	enqueueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 150*time.Millisecond)
	defer cancel()
	if err := s.queue.Enqueue(enqueueCtx, account.ID, time.Now()); err != nil {
		slog.Warn("quality 5xx signal enqueue failed", "account_id", account.ID)
	}
}

func (s *ScheduledTestRunnerService) startQualityTriggers() {
	if s.qualityTrigger == nil || s.qualityTrigger.queue == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.triggerCancel = cancel
	s.triggerWG.Add(1)
	go func() {
		defer s.triggerWG.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var active sync.Map
		slots := make(chan struct{}, scheduledTestDefaultMaxWorkers)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			scanCtx, stop := context.WithTimeout(ctx, 2*time.Second)
			pending, err := s.qualityTrigger.queue.Pending(scanCtx, time.Now().Add(-20*time.Minute))
			stop()
			if err != nil {
				continue
			}
			for _, entry := range pending {
				if _, loaded := active.LoadOrStore(entry.AccountID, true); loaded {
					continue
				}
				select {
				case slots <- struct{}{}:
				default:
					active.Delete(entry.AccountID)
					continue
				}
				s.triggerWG.Add(1)
				go func(entry qualityqueue.Signal) {
					defer s.triggerWG.Done()
					defer func() { <-slots; active.Delete(entry.AccountID) }()
					runCtx, stop := context.WithTimeout(ctx, 12*time.Minute)
					defer stop()
					_ = s.processQualitySignal(runCtx, entry)
				}(entry)
			}
		}
	}()
}

func (s *ScheduledTestRunnerService) processQualitySignal(ctx context.Context, entry qualityqueue.Signal) error {
	if err := s.runQualityTriggeredAccount(ctx, entry.AccountID, entry.ObservedAt); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.qualityTrigger.queue.Acknowledge(ctx, entry)
}

func qualitySignalCovered(plan *ScheduledTestPlan, observedAt time.Time) bool {
	return plan.LastRunAt != nil && !plan.LastRunAt.Before(observedAt)
}

func quality5xxPlanEnabled(plan *ScheduledTestPlan) bool {
	return plan != nil && plan.Enabled && plan.PelicanConfig != nil && plan.PelicanConfig.Quality != nil && plan.PelicanConfig.Quality.TriggerOnUpstream5xx
}

func (s *ScheduledTestRunnerService) runQualityTriggeredAccount(ctx context.Context, accountID int64, observedAt time.Time) error {
	repo := s.planRepoForQuality()
	plans, err := repo.ListByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if !quality5xxPlanEnabled(plan) || qualitySignalCovered(plan, observedAt) {
			continue
		}
		plan.TriggerSource = quality5xxSource
		plan.TriggerObservedAt = &observedAt
		if s.runQualityPlan(ctx, plan) {
			continue
		}
		// A failed claim is not proof of coverage: a sibling lease, rest period,
		// or transient advisory-lock conflict must leave the signal pending.
		current, err := repo.GetByID(ctx, plan.ID)
		if err != nil {
			return err
		}
		if !quality5xxPlanEnabled(current) || qualitySignalCovered(current, observedAt) {
			continue
		}
		return fmt.Errorf("quality plan %d has not covered the upstream signal", plan.ID)
	}
	return nil
}

func (s *RateLimitService) observeQualityResponse(ctx context.Context, account *Account, response *http.Response, err error) {
	if err == nil && response != nil {
		s.observeQualityStatus(ctx, account, response.StatusCode)
	}
}

func (s *RateLimitService) observeQualityStatus(ctx context.Context, account *Account, status int) {
	if s != nil {
		s.qualityTrigger.Observe(ctx, account, status)
	}
}
