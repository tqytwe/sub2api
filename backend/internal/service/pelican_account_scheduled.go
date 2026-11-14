package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

func isPelicanSVGConfig(cfg *PelicanTestConfig) bool {
	return cfg != nil && cfg.Quality == nil && (cfg.QuestionKind == "pelican" || cfg.PromptMode == PelicanPromptModeRandom)
}

func (s *ScheduledTestRunnerService) runPelicanPlan(ctx context.Context, plan *ScheduledTestPlan) bool {
	if !isPelicanSVGConfig(plan.PelicanConfig) {
		return false
	}
	if err := validatePelicanPromptMode(plan.PelicanConfig); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "SVG plan=%d invalid prompt mode: %v", plan.ID, err)
		return false
	}
	if err := validateQualityProbeTargets(plan.PelicanConfig); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "SVG plan=%d invalid targets: %v", plan.ID, err)
		return false
	}
	repo, ok := s.planRepo.(PelicanRuntimeRepository)
	if !ok {
		logger.LegacyPrintf("service.scheduled_test_runner", "SVG plan=%d lease storage unavailable", plan.ID)
		return false
	}
	now := time.Now()
	due := now
	if plan.NextRunAt != nil {
		due = *plan.NextRunAt
	}
	next, err := computeNextRun(plan.CronExpression, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "SVG plan=%d invalid schedule: %v", plan.ID, err)
		return false
	}
	until := now.Add(pelicanGroupTestLease).Truncate(time.Microsecond)
	version, claimed, err := repo.ClaimPelicanRun(ctx, plan, now, until, next)
	if err != nil || !claimed {
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "SVG plan=%d claim failed: %v", plan.ID, err)
		}
		return false
	}
	snapshot := *plan
	snapshot.UpdatedAt, snapshot.RunningUntil = version, &until
	snapshot.PelicanConfig = freezePelicanQuestion(plan.PelicanConfig, "scheduled:"+due.UTC().Format(time.RFC3339Nano), nil)
	models := QualityProbeModels(&snapshot)
	count := len(models) * QualityProbeParallelCount(snapshot.PelicanConfig)
	results := make([]*ScheduledTestResult, count)
	jobs := make(chan int, count)
	for index := range results {
		jobs <- index
	}
	close(jobs)
	runCtx, cancel := context.WithTimeout(ctx, pelicanGroupTestRunTimeout)
	defer cancel()
	var wg sync.WaitGroup
	for worker := 0; worker < min(count, qualityProbeMaxWorkers); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				cfg := *snapshot.PelicanConfig
				cfg.ModelID = models[index%len(models)]
				started := time.Now()
				result, runErr := s.runPelican(runCtx, snapshot.AccountID, cfg.ModelID, &cfg)
				if runErr != nil || result == nil {
					message := "SVG generation returned no result"
					if runErr != nil {
						message = runErr.Error()
					}
					result = &ScheduledTestResult{Status: "failed", ErrorMessage: message, StartedAt: started, FinishedAt: time.Now()}
				}
				result.PlanID, result.PelicanConfig = snapshot.ID, &cfg
				results[index] = result
			}
		}()
	}
	wg.Wait()
	saveCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	committed, err := repo.CommitPelicanRound(saveCtx, &snapshot, results, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "SVG plan=%d commit failed: %v", plan.ID, err)
	}
	if !committed || err != nil {
		return false
	}
	if snapshot.AutoRecover {
		for _, result := range results {
			if result.Status == "success" {
				s.tryRecoverAccount(saveCtx, snapshot.AccountID, snapshot.ID)
				break
			}
		}
	}
	return true
}
