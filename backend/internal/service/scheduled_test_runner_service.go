package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const scheduledTestDefaultMaxWorkers = 10

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo        ScheduledTestPlanRepository
	scheduledSvc    *ScheduledTestService
	accountTestSvc  *AccountTestService
	rateLimitSvc    *RateLimitService
	qualityJudge    *qualityJevJudge
	runQualityProbe func(context.Context, int64, string, string) (*ScheduledTestResult, error)
	cfg             *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewScheduledTestRunnerService creates a new runner.
func NewScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	cfg *config.Config,
	moderationSvc *ContentModerationService,
) *ScheduledTestRunnerService {
	return &ScheduledTestRunnerService{
		planRepo:        planRepo,
		scheduledSvc:    scheduledSvc,
		accountTestSvc:  accountTestSvc,
		rateLimitSvc:    rateLimitSvc,
		qualityJudge:    newQualityJevJudge(moderationSvc, accountTestSvc),
		runQualityProbe: accountTestSvc.RunQualityProbeBackground,
		cfg:             cfg,
	}
}

// Start begins the cron ticker (every minute).
func (s *ScheduledTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *ScheduledTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledTestRunnerService) runScheduled() {
	// Delay 10s so execution lands at ~:10 of each minute instead of :00.
	time.Sleep(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	now := time.Now()
	if s.scheduledSvc != nil {
		if created, err := s.scheduledSvc.SyncQualityTemplates(ctx); err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "quality template sync error: %v", err)
		} else if created > 0 {
			logger.LegacyPrintf("service.scheduled_test_runner", "quality templates added %d rules", created)
		}
	}
	plans, err := s.planRepo.ListDue(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] found %d due plans", len(plans))

	sem := make(chan struct{}, scheduledTestDefaultMaxWorkers)
	var wg sync.WaitGroup

	for _, plan := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *ScheduledTestPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlan(ctx, p)
		}(plan)
	}

	wg.Wait()
}

func (s *ScheduledTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledTestPlan) {
	if plan.PelicanConfig != nil && plan.PelicanConfig.Quality != nil {
		s.runQualityPlan(ctx, plan)
		return
	}
	result, err := s.accountTestSvc.RunTestBackground(ctx, plan.AccountID, plan.ModelID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d RunTestBackground error: %v", plan.ID, err)
		return
	}

	if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
	}

	// Auto-recover account if test succeeded and auto_recover is enabled.
	if result.Status == "success" && plan.AutoRecover {
		s.tryRecoverAccount(ctx, plan.AccountID, plan.ID)
	}

	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}

	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

func (s *ScheduledTestRunnerService) runQualityPlan(ctx context.Context, plan *ScheduledTestPlan) {
	now := time.Now()
	leaseUntil := now.Add(15 * time.Minute).Truncate(time.Microsecond)
	claimVersion, claimed, err := s.scheduledSvc.ClaimQualityRun(ctx, plan, leaseUntil)
	if err != nil || !claimed {
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "quality plan=%d claim failed: %v", plan.ID, err)
		}
		return
	}
	plan.UpdatedAt = claimVersion
	plan.RunningUntil = &leaseUntil

	results, outcome := s.runQualityRound(ctx, plan, now)

	saveCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	action, actionErr := s.scheduledSvc.CommitQualityRound(saveCtx, plan, outcome, results)
	if actionErr != nil {
		action = "action_error"
		logger.LegacyPrintf("service.scheduled_test_runner", "quality plan=%d outcome apply failed: %v", plan.ID, actionErr)
	}
	for _, result := range results {
		result.QualityAction = action
	}
	nextRun, nextErr := computeNextRun(plan.CronExpression, time.Now())
	if nextErr != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "quality plan=%d computeNextRun failed: %v", plan.ID, nextErr)
		return
	}
	if finishErr := s.scheduledSvc.FinishQualityRun(saveCtx, plan.ID, leaseUntil, time.Now(), nextRun); finishErr != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "quality plan=%d finish failed: %v", plan.ID, finishErr)
	}
}

// runQualityRound runs the frozen target set without changing rule identity.
// Model jobs are interleaved so a slow target cannot monopolize all workers.
func (s *ScheduledTestRunnerService) runQualityRound(ctx context.Context, plan *ScheduledTestPlan, startedAt time.Time) ([]*ScheduledTestResult, string) {
	config := cloneQualityPelicanConfig(plan.PelicanConfig)
	round := NewQualityRoundID()
	if err := validateQualityConfig(config); err != nil {
		result, judgment, outcome := unsupportedQualityConfigResult(config, startedAt)
		result.QualityJudgment = judgment
		result.QualityRoundID = round
		return []*ScheduledTestResult{result}, outcome
	}
	models := QualityProbeModels(plan)
	config.ModelIDs = qualityProbeModelIDs(models)
	count := len(models) * QualityProbeParallelCount(config)
	results := make([]*ScheduledTestResult, count)
	outcomes := make([]string, count)
	jobs := make(chan int, count)
	for i := 0; i < count; i++ {
		jobs <- i
	}
	close(jobs)
	workers := min(count, qualityProbeMaxWorkers)
	if config.Quality.Judge.Kind == "jev" && s.qualityJudge != nil {
		workers = min(workers, cap(s.qualityJudge.slots))
	}
	var probes sync.WaitGroup
	for i := 0; i < workers; i++ {
		probes.Add(1)
		go func() {
			defer probes.Done()
			for index := range jobs {
				sampleConfig := cloneQualityPelicanConfig(config)
				sampleConfig.ModelID = models[index%len(models)]
				probeStart := time.Now()
				var result *ScheduledTestResult
				err := ctx.Err()
				if err == nil {
					result, err = s.runQualityProbe(ctx, plan.AccountID, sampleConfig.ModelID, sampleConfig.Prompt)
				}
				if err != nil {
					result = &ScheduledTestResult{Status: "failed", ErrorMessage: "probe_failed", StartedAt: probeStart, FinishedAt: time.Now()}
				}
				result.PelicanConfig = sampleConfig
				if sampleConfig.Quality.Judge.Kind == "jev" {
					result.QualityJudgment, outcomes[index] = s.judgeQualityProbeWithJev(ctx, result, sampleConfig)
				} else {
					result.QualityJudgment, outcomes[index] = judgeQualityProbe(result, sampleConfig)
				}
				if result.Status != "success" && result.ErrorMessage != "answer_mismatch" {
					result.ErrorMessage = "probe_failed"
				}
				result.QualityRoundID = round
				results[index] = result
			}
		}()
	}
	probes.Wait()
	outcome := "passed"
	for _, probeOutcome := range outcomes {
		if probeOutcome == "failed" {
			return results, "failed"
		}
		if probeOutcome != "passed" {
			outcome = "inconclusive"
		}
	}
	return results, outcome
}

// tryRecoverAccount attempts to recover an account from recoverable runtime state.
func (s *ScheduledTestRunnerService) tryRecoverAccount(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover failed: %v", planID, err)
		return
	}
	if recovery == nil {
		return
	}

	if recovery.ClearedError {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d recovered from error status", planID, accountID)
	}
	if recovery.ClearedRateLimit {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d cleared rate-limit/runtime state", planID, accountID)
	}
}
