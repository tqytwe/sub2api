package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/robfig/cron/v3"
)

func (s *ScheduledTestService) QualityOwnership(ctx context.Context, id int64, req *QualityOwnershipRequest) (*QualityOwnershipPreview, error) {
	repo, ok := s.planRepo.(QualityOwnershipRepository)
	if !ok {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "QUALITY_OWNERSHIP_UNAVAILABLE", "quality ownership is unavailable")
	}
	if req != nil && (req.Reconcile || req.Restore || req.Delete) && (!req.Confirm || len(req.Fence) != 64) {
		return nil, infraerrors.BadRequest("QUALITY_OWNERSHIP_CONFIRM_REQUIRED", "preview and explicitly confirm ownership reconciliation")
	}
	return repo.QualityOwnership(ctx, id, req)
}

func (s *ScheduledTestService) ClaimQualityRun(ctx context.Context, plan *ScheduledTestPlan, until time.Time) (time.Time, bool, error) {
	repo, ok := s.planRepo.(QualityRuntimeRepository)
	if !ok {
		return time.Time{}, false, infraerrors.New(http.StatusServiceUnavailable, "QUALITY_RUNNER_UNAVAILABLE", "quality runner is unavailable")
	}
	return repo.ClaimQualityRun(ctx, plan, until)
}

func (s *ScheduledTestService) CommitQualityRun(ctx context.Context, plan *ScheduledTestPlan, outcome string, result *ScheduledTestResult) (string, error) {
	return s.CommitQualityRound(ctx, plan, outcome, []*ScheduledTestResult{result})
}

// CommitQualityRound grades scheduling ownership once and saves all probe
// samples atomically. Validate the complete batch before clearing sensitive data.
func (s *ScheduledTestService) CommitQualityRound(ctx context.Context, plan *ScheduledTestPlan, outcome string, results []*ScheduledTestResult) (string, error) {
	repo, ok := s.planRepo.(QualityRuntimeRepository)
	if !ok {
		return "action_error", infraerrors.New(http.StatusServiceUnavailable, "QUALITY_RUNNER_UNAVAILABLE", "quality runner is unavailable")
	}
	if plan == nil || len(results) == 0 {
		return "action_error", infraerrors.BadRequest("QUALITY_RESULT_INVALID", "quality round requires a plan and results")
	}
	var round string
	for i, result := range results {
		if err := validateQualityResult(result); err != nil {
			return "action_error", err
		}
		if i == 0 {
			round = result.QualityRoundID
		} else if result.QualityRoundID != round {
			return "action_error", infraerrors.BadRequest("QUALITY_ROUND_INVALID", "quality samples must share a round id")
		}
	}
	if strings.TrimSpace(round) == "" {
		return "action_error", infraerrors.BadRequest("QUALITY_ROUND_REQUIRED", "quality results require an opaque round id")
	}

	for _, result := range results {
		result.PlanID = plan.ID
		sanitizeQualityResult(result)
	}
	return repo.CommitQualityRound(ctx, plan, outcome, results, plan.MaxResults)
}

func sanitizeQualityResult(result *ScheduledTestResult) {
	if result == nil || result.PelicanConfig == nil || result.PelicanConfig.Quality == nil {
		return
	}
	result.ResponseText = ""
	if result.ErrorMessage != "answer_mismatch" && result.ErrorMessage != "quality_config_unsupported" && result.ErrorMessage != "quality_judge_failed" && result.ErrorMessage != "" {
		result.ErrorMessage = "probe_failed"
	}
	config := *result.PelicanConfig
	config.Prompt = ""
	quality := *config.Quality
	quality.ExpectedAnswer = ""
	if quality.Judge != nil {
		judge := *quality.Judge
		judge.Prompt = ""
		quality.Judge = &judge
	}
	config.Quality = &quality
	result.PelicanConfig = &config
}

func (s *ScheduledTestService) FinishQualityRun(ctx context.Context, id int64, until, lastRunAt, nextRunAt time.Time) error {
	repo, ok := s.planRepo.(QualityRuntimeRepository)
	if !ok {
		return infraerrors.New(http.StatusServiceUnavailable, "QUALITY_RUNNER_UNAVAILABLE", "quality runner is unavailable")
	}
	return repo.FinishQualityRun(ctx, id, until, lastRunAt, nextRunAt)
}

func judgeQualityProbe(result *ScheduledTestResult, config *PelicanTestConfig) (map[string]any, string) {
	if result == nil {
		return map[string]any{"verdict": "unknown", "reason": "probe_missing"}, "inconclusive"
	}
	response := result.ResponseText
	digest := sha256.Sum256([]byte(response))
	judgment := map[string]any{"response_sha256": hex.EncodeToString(digest[:])}
	if config == nil || config.Quality == nil || result.Status != "success" || strings.TrimSpace(response) == "" {
		judgment["verdict"] = "unknown"
		judgment["reason"] = "probe_inconclusive"
		result.ResponseText = ""
		return judgment, "inconclusive"
	}
	if strings.TrimSpace(response) == strings.TrimSpace(config.Quality.ExpectedAnswer) {
		judgment["verdict"] = "correct"
		judgment["reason"] = "expected_answer_match"
		result.ResponseText = ""
		return judgment, "passed"
	}
	judgment["verdict"] = "incorrect"
	judgment["reason"] = "expected_answer_mismatch"
	result.Status = "failed"
	result.ErrorMessage = "answer_mismatch"
	result.ResponseText = ""
	return judgment, "failed"
}

func (s *ScheduledTestRunnerService) judgeQualityProbeWithJev(ctx context.Context, result *ScheduledTestResult, config *PelicanTestConfig) (map[string]any, string) {
	if result == nil {
		return map[string]any{"verdict": "unknown", "reason": "probe_missing"}, "inconclusive"
	}
	response := result.ResponseText
	digest := sha256.Sum256([]byte(response))
	judgment := map[string]any{
		"response_sha256": hex.EncodeToString(digest[:]),
		"kind":            "jev",
		"model_id":        "jev-latest",
		"rules_version":   qualityJevRulesVersion,
	}
	if config == nil || config.Quality == nil || result.Status != "success" {
		judgment["verdict"] = "unknown"
		judgment["reason"] = "probe_inconclusive"
		result.ResponseText = ""
		return judgment, "inconclusive"
	}
	var evaluated qualityJevJudgment
	if s == nil || s.qualityJudge == nil {
		evaluated = qualityJevJudgment{Verdict: "unknown", Reason: "judge_jev_not_configured"}
	} else {
		evaluated = s.qualityJudge.judge(ctx, config, response)
	}
	result.ResponseText = ""
	judgment["latency_ms"] = evaluated.LatencyMS
	judgment["verdict"] = evaluated.Verdict
	judgment["reason"] = evaluated.Reason
	result.LatencyMs += evaluated.LatencyMS
	switch evaluated.Verdict {
	case "correct":
		return judgment, "passed"
	case "incorrect":
		result.Status = "failed"
		result.ErrorMessage = "answer_mismatch"
		return judgment, "failed"
	default:
		return judgment, "inconclusive"
	}
}

func unsupportedQualityConfigResult(config *PelicanTestConfig, startedAt time.Time) (*ScheduledTestResult, map[string]any, string) {
	return &ScheduledTestResult{
		Status: "failed", ErrorMessage: "quality_config_unsupported", StartedAt: startedAt,
		FinishedAt: time.Now(), PelicanConfig: config,
	}, map[string]any{"verdict": "unknown", "reason": "quality_config_unsupported"}, "inconclusive"
}

var scheduledTestCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ScheduledTestService provides CRUD operations for scheduled test plans and results.
type ScheduledTestService struct {
	planRepo     ScheduledTestPlanRepository
	resultRepo   ScheduledTestResultRepository
	templateRepo QualityRuleTemplateRepository
	accountTests *AccountTestService
}

// NewScheduledTestService creates a new ScheduledTestService.
func NewScheduledTestService(
	planRepo ScheduledTestPlanRepository,
	resultRepo ScheduledTestResultRepository,
	templateRepo ...QualityRuleTemplateRepository,
) *ScheduledTestService {
	var templates QualityRuleTemplateRepository
	if len(templateRepo) > 0 {
		templates = templateRepo[0]
	}
	return &ScheduledTestService{
		planRepo: planRepo, resultRepo: resultRepo, templateRepo: templates,
	}
}

// SetAccountTestService wires the account model eligibility reader used by
// quality plans.  Keeping this as a setter preserves the lightweight
// constructor used by repository and service tests while production wiring
// supplies the account-backed validator.
func (s *ScheduledTestService) SetAccountTestService(accountTests *AccountTestService) {
	if s != nil {
		s.accountTests = accountTests
	}
}

// CreatePlan validates the cron expression, computes next_run_at, and persists the plan.
func (s *ScheduledTestService) CreatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	if err := prepareQualityPlan(plan); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	if err := s.validateQualityAccountModels(ctx, plan); err != nil {
		return nil, err
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

func (s *ScheduledTestService) ListQualityPlans(ctx context.Context) ([]*ScheduledTestPlan, error) {
	return s.planRepo.ListQualityPlans(ctx)
}

func (s *ScheduledTestService) TriggerQuality(ctx context.Context, id int64) error {
	return s.planRepo.TriggerQuality(ctx, id)
}

// UpdatePlan validates cron and updates the plan.
func (s *ScheduledTestService) UpdatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	if err := prepareQualityPlan(plan); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	if plan.Enabled {
		if err := s.validateQualityAccountModels(ctx, plan); err != nil {
			return nil, err
		}
	}
	plan.NextRunAt = &nextRun

	return s.planRepo.Update(ctx, plan)
}

// validateQualityAccountModels rejects an enabled quality rule that asks an
// account to probe a model outside that account's configured model contract.
// It intentionally uses only the persisted account mapping and does not call
// an upstream catalog: catalog failure must never be treated as proof that a
// model is unsupported.
func (s *ScheduledTestService) validateQualityAccountModels(ctx context.Context, plan *ScheduledTestPlan) error {
	if s == nil || s.accountTests == nil || s.accountTests.accountRepo == nil || plan == nil || plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil {
		return nil
	}
	account, err := s.accountTests.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil {
		return err
	}
	if account == nil {
		return ErrAccountNotFound
	}
	for _, model := range QualityProbeModels(plan) {
		model = strings.TrimSpace(model)
		if model == "" || !account.IsModelSupported(model) {
			return infraerrors.BadRequest("QUALITY_MODEL_NOT_ALLOWED", "quality rule model is not allowed for this account")
		}
	}
	return nil
}

func (s *ScheduledTestService) UpdatePlanWithVersion(ctx context.Context, plan *ScheduledTestPlan, expected time.Time) (*ScheduledTestPlan, error) {
	if err := prepareQualityPlan(plan); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	if plan.Enabled {
		if err := s.validateQualityAccountModels(ctx, plan); err != nil {
			return nil, err
		}
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
	if config.Quality == nil {
		return nil
	}
	if err := validateQualityProbeTargets(config); err != nil {
		return err
	}
	if config.Quality.Action != "observe_only" && config.Quality.Action != "disable_scheduling" {
		return infraerrors.BadRequest("QUALITY_ACTION_INVALID", "unsupported quality action")
	}
	if strings.TrimSpace(config.Prompt) == "" || len(strings.TrimSpace(config.Quality.ExpectedAnswer)) == 0 {
		return infraerrors.BadRequest("QUALITY_ANSWER_REQUIRED", "quality probes require a prompt and expected answer")
	}
	if config.Quality.Judge == nil || (config.Quality.Judge.Kind != "expected" && config.Quality.Judge.Kind != "jev") || config.Quality.Judge.GroupID != 0 || config.Quality.Judge.ModelID != "" {
		return infraerrors.BadRequest("QUALITY_JUDGE_KIND_INVALID", "unsupported quality judge")
	}
	if len(config.Quality.ExpectedAnswer) > 4000 {
		return infraerrors.BadRequest("QUALITY_ANSWER_TOO_LARGE", "expected answer exceeds 4000 characters")
	}
	judgePromptLimit := 8192
	if config.Quality.Judge.Kind == "jev" {
		judgePromptLimit = 16000
	}
	if len(config.Quality.Judge.Prompt) > judgePromptLimit {
		return infraerrors.BadRequest("QUALITY_JUDGE_PROMPT_TOO_LARGE", "judge prompt exceeds the supported limit")
	}
	if config.Quality.Judge.Kind == "expected" && strings.TrimSpace(config.Quality.Judge.Prompt) != "" {
		return infraerrors.BadRequest("QUALITY_JUDGE_PROMPT_UNSUPPORTED", "custom judge prompts require Jev")
	}
	if config.Quality.Judge.Kind == "jev" && strings.TrimSpace(config.Quality.Judge.Prompt) == "" {
		return infraerrors.BadRequest("QUALITY_JUDGE_PROMPT_REQUIRED", "Jev judging requires a rubric")
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
