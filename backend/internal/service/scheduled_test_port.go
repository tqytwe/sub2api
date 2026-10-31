package service

import (
	"context"
	"time"
)

// ScheduledTestPlan represents a scheduled test plan domain model.
type ScheduledTestPlan struct {
	PelicanConfig  *PelicanTestConfig `json:"pelican_config,omitempty"`
	RunningUntil   *time.Time         `json:"running_until,omitempty"`
	ID             int64              `json:"id"`
	AccountID      int64              `json:"account_id"`
	ModelID        string             `json:"model_id"`
	CronExpression string             `json:"cron_expression"`
	Enabled        bool               `json:"enabled"`
	MaxResults     int                `json:"max_results"`
	AutoRecover    bool               `json:"auto_recover"`
	LastRunAt      *time.Time         `json:"last_run_at"`
	NextRunAt      *time.Time         `json:"next_run_at"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// PelicanTestConfig is an optional quality probe configuration. A nil value
// preserves the legacy account connectivity test contract.
type PelicanTestConfig struct {
	RuleName        string         `json:"rule_name,omitempty"`
	Quality         *QualityPolicy `json:"quality,omitempty"`
	QuestionKind    string         `json:"question_kind,omitempty"`
	TestChannel     string         `json:"test_channel,omitempty"`
	Prompt          string         `json:"prompt,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	ParallelCount   int            `json:"parallel_count,omitempty"`
	ModelID         string         `json:"model_id,omitempty"`
	ModelIDs        []string       `json:"model_ids,omitempty"`
}

type QualityPolicy struct {
	Action         string              `json:"action,omitempty"`
	ExpectedAnswer string              `json:"expected_answer,omitempty"`
	AutoRestore    bool                `json:"auto_restore,omitempty"`
	Judge          *QualityJudgeConfig `json:"judge,omitempty"`
}

type QualityJudgeConfig struct {
	Kind    string `json:"kind,omitempty"`
	GroupID int64  `json:"group_id,omitempty"`
	ModelID string `json:"model_id,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
}

// ScheduledTestResult represents a single test execution result.
type ScheduledTestResult struct {
	PelicanConfig   *PelicanTestConfig `json:"pelican_config,omitempty"`
	QualityRoundID  string             `json:"quality_round_id,omitempty"`
	QualityAction   string             `json:"quality_action,omitempty"`
	QualityJudgment map[string]any     `json:"quality_judgment,omitempty"`
	ID              int64              `json:"id"`
	PlanID          int64              `json:"plan_id"`
	Status          string             `json:"status"`
	ResponseText    string             `json:"response_text"`
	ErrorMessage    string             `json:"error_message"`
	LatencyMs       int64              `json:"latency_ms"`
	StartedAt       time.Time          `json:"started_at"`
	FinishedAt      time.Time          `json:"finished_at"`
	CreatedAt       time.Time          `json:"created_at"`
}

// QualityHistoryResult is one cursor-addressable quality round. A round may
// contain multiple probe results (for example, when parallel_count is used),
// so the counts describe the complete round rather than a single row.
type QualityHistoryResult struct {
	ScheduledTestResult
	AccountID    int64   `json:"account_id"`
	AccountName  string  `json:"account_name"`
	PassedCount  int     `json:"passed_count"`
	SkippedCount int     `json:"skipped_count"`
	TotalCount   int     `json:"total_count"`
	ResultIDs    []int64 `json:"result_ids"`
}

type QualityHistoryPage struct {
	Items      []*QualityHistoryResult `json:"items"`
	NextCursor int64                   `json:"next_cursor"`
}

// ScheduledTestPlanRepository defines the data access interface for test plans.
type ScheduledTestPlanRepository interface {
	Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error)
	ListQualityPlans(ctx context.Context) ([]*ScheduledTestPlan, error)
	TriggerQuality(ctx context.Context, id int64) error
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
	Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
}

type VersionedScheduledTestPlanRepository interface {
	UpdateWithVersion(context.Context, *ScheduledTestPlan, time.Time) (*ScheduledTestPlan, error)
}

// ScheduledTestResultRepository defines the data access interface for test results.
type ScheduledTestResultRepository interface {
	Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error)
	ListByPlanID(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error)
	ListQualityHistory(ctx context.Context, beforeID int64, limit int) ([]*QualityHistoryResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
}

type QualityOwnershipPreview struct {
	PlanID         int64  `json:"plan_id"`
	AccountID      int64  `json:"account_id"`
	RuleVersion    string `json:"rule_version"`
	Fence          string `json:"fence"`
	Action         string `json:"action,omitempty"`
	StatePresent   bool   `json:"state_present"`
	RestoreReady   bool   `json:"restore_ready"`
	ReconcileReady bool   `json:"reconcile_ready"`
}

type QualityOwnershipRequest struct {
	Fence     string `json:"fence"`
	Confirm   bool   `json:"confirm"`
	Reconcile bool   `json:"-"`
	Restore   bool   `json:"-"`
	Delete    bool   `json:"-"`
}

type QualityOwnershipRepository interface {
	QualityOwnership(context.Context, int64, *QualityOwnershipRequest) (*QualityOwnershipPreview, error)
}

type QualityRuntimeRepository interface {
	ClaimQualityRun(context.Context, *ScheduledTestPlan, time.Time) (time.Time, bool, error)
	CommitQualityRun(context.Context, *ScheduledTestPlan, string, *ScheduledTestResult, int) (string, error)
	FinishQualityRun(context.Context, int64, time.Time, time.Time, time.Time) error
}
