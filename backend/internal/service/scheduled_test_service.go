package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/robfig/cron/v3"
)

func (s *ScheduledTestService) QualityOwnership(ctx context.Context, id int64, req *QualityOwnershipRequest) (*QualityOwnershipPreview, error) {
	repo, ok := s.planRepo.(QualityOwnershipRepository)
	if !ok {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "QUALITY_OWNERSHIP_UNAVAILABLE", "quality ownership is unavailable")
	}
	if req != nil && (req.Restore || req.Delete) && (!req.Confirm || len(req.Fence) != 64) {
		return nil, infraerrors.BadRequest("QUALITY_OWNERSHIP_CONFIRM_REQUIRED", "preview and explicitly confirm ownership reconciliation")
	}
	return repo.QualityOwnership(ctx, id, req)
}

var scheduledTestCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ScheduledTestService provides CRUD operations for scheduled test plans and results.
type ScheduledTestService struct {
	planRepo   ScheduledTestPlanRepository
	resultRepo ScheduledTestResultRepository
}

// NewScheduledTestService creates a new ScheduledTestService.
func NewScheduledTestService(
	planRepo ScheduledTestPlanRepository,
	resultRepo ScheduledTestResultRepository,
) *ScheduledTestService {
	return &ScheduledTestService{
		planRepo:   planRepo,
		resultRepo: resultRepo,
	}
}

// CreatePlan validates the cron expression, computes next_run_at, and persists the plan.
func (s *ScheduledTestService) CreatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	if err := validateQualityConfig(plan.PelicanConfig); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	plan.NextRunAt = &nextRun

	if plan.MaxResults <= 0 {
		plan.MaxResults = 50
	}

	return s.planRepo.Create(ctx, plan)
}

// GetPlan retrieves a plan by ID.
func (s *ScheduledTestService) GetPlan(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	return s.planRepo.GetByID(ctx, id)
}

// ListPlansByAccount returns all plans for a given account.
func (s *ScheduledTestService) ListPlansByAccount(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	return s.planRepo.ListByAccountID(ctx, accountID)
}

// UpdatePlan validates cron and updates the plan.
func (s *ScheduledTestService) UpdatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	if err := validateQualityConfig(plan.PelicanConfig); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	plan.NextRunAt = &nextRun

	return s.planRepo.Update(ctx, plan)
}

func (s *ScheduledTestService) UpdatePlanWithVersion(ctx context.Context, plan *ScheduledTestPlan, expected time.Time) (*ScheduledTestPlan, error) {
	if err := validateQualityConfig(plan.PelicanConfig); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	plan.NextRunAt = &nextRun
	if repo, ok := s.planRepo.(VersionedScheduledTestPlanRepository); ok {
		return repo.UpdateWithVersion(ctx, plan, expected)
	}
	return nil, infraerrors.New(http.StatusServiceUnavailable, "QUALITY_VERSION_UNAVAILABLE", "versioned plan updates are unavailable")
}

func validateQualityConfig(config *PelicanTestConfig) error {
	if config == nil {
		return nil
	}
	if len(config.Prompt) > 32768 {
		return infraerrors.BadRequest("QUALITY_PROMPT_TOO_LARGE", "quality prompt exceeds 32768 characters")
	}
	if config.ParallelCount < 0 || config.ParallelCount > 32 {
		return infraerrors.BadRequest("QUALITY_PARALLEL_COUNT_INVALID", "parallel_count must be between 0 and 32")
	}
	if config.Quality != nil && config.Quality.Judge != nil {
		if config.Quality.Judge.Kind != "" && config.Quality.Judge.Kind != "jev" && config.Quality.Judge.Kind != "llm" {
			return infraerrors.BadRequest("QUALITY_JUDGE_KIND_INVALID", "unsupported quality judge kind")
		}
		if len(config.Quality.Judge.Prompt) > 8192 {
			return infraerrors.BadRequest("QUALITY_JUDGE_PROMPT_TOO_LARGE", "judge prompt exceeds 8192 characters")
		}
	}
	return nil
}

// DeletePlan removes a plan and its results (via CASCADE).
func (s *ScheduledTestService) DeletePlan(ctx context.Context, id int64) error {
	return s.planRepo.Delete(ctx, id)
}

// ListResults returns the most recent results for a plan.
func (s *ScheduledTestService) ListResults(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.resultRepo.ListByPlanID(ctx, planID, limit)
}

// SaveResult inserts a result and prunes old entries beyond maxResults.
func (s *ScheduledTestService) SaveResult(ctx context.Context, planID int64, maxResults int, result *ScheduledTestResult) error {
	if result != nil && result.PelicanConfig != nil && result.PelicanConfig.Quality != nil && result.QualityRoundID == "" {
		result.QualityRoundID = NewQualityRoundID()
	}
	if err := validateQualityResult(result); err != nil {
		return err
	}
	result.PlanID = planID
	if _, err := s.resultRepo.Create(ctx, result); err != nil {
		return err
	}
	return s.resultRepo.PruneOldResults(ctx, planID, maxResults)
}

// NewQualityRoundID returns an opaque identifier shared by all probes in one
// scheduled quality execution. It contains no account, model, or prompt data.
func NewQualityRoundID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func validateQualityResult(result *ScheduledTestResult) error {
	if result == nil {
		return infraerrors.BadRequest("QUALITY_RESULT_INVALID", "quality result is required")
	}
	if result.PelicanConfig != nil && result.PelicanConfig.Quality != nil && result.QualityRoundID == "" {
		return infraerrors.BadRequest("QUALITY_ROUND_REQUIRED", "quality results require an opaque round id")
	}
	if len(result.QualityRoundID) > 128 {
		return infraerrors.BadRequest("QUALITY_ROUND_INVALID", "quality round id is too long")
	}
	for key := range result.QualityJudgment {
		if len(key) > 64 {
			return infraerrors.BadRequest("QUALITY_JUDGMENT_INVALID", "quality judgment keys are too long")
		}
	}
	switch result.QualityAction {
	case "", "passed", "degraded", "inconclusive", "stale_run", "disable_scheduling", "observe_only":
		return nil
	default:
		return infraerrors.BadRequest("QUALITY_ACTION_INVALID", "unsupported quality action")
	}
}

func computeNextRun(cronExpr string, from time.Time) (time.Time, error) {
	sched, err := scheduledTestCronParser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}
