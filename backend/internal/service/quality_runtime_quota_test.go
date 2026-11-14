package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type qualityRuntimeQuotaPlans struct {
	qualityProbeRunnerRepo
	onClaim     func()
	commitError error
	skips       int
}

func (r *qualityRuntimeQuotaPlans) ClaimQualityRun(ctx context.Context, p *ScheduledTestPlan, until time.Time) (time.Time, bool, error) {
	if r.onClaim != nil {
		r.onClaim()
	}
	return p.UpdatedAt, true, nil
}
func (r *qualityRuntimeQuotaPlans) CommitQualityRound(ctx context.Context, p *ScheduledTestPlan, outcome string, results []*ScheduledTestResult, keep int) (string, error) {
	if r.commitError != nil {
		return "", r.commitError
	}
	return r.qualityProbeRunnerRepo.CommitQualityRound(ctx, p, outcome, results, keep)
}
func (r *qualityRuntimeQuotaPlans) FinishQuotaSkippedQuality(context.Context, *ScheduledTestPlan, time.Time) error {
	r.skips++
	return nil
}

type qualityRuntimeQuotaTemplates struct{ QualityRuleTemplateRepository }
type qualityRuntimeQuotaAccounts struct {
	AccountRepository
	account *Account
}

func (r *qualityRuntimeQuotaAccounts) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}
func (*qualityRuntimeQuotaTemplates) ForPlan(context.Context, int64) (*QualityRuleTemplate, error) {
	return &QualityRuleTemplate{AccountFilter: QualityRuleAccountFilter{Statuses: []string{StatusActive}}}, nil
}

func TestQualityRuntimeQuotaRecheckedAfterClaimWithoutCompletingRound(t *testing.T) {
	repo := &qualityRuntimeQuotaPlans{}
	account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive}
	repo.onClaim = func() { until := time.Now().Add(time.Hour); account.RateLimitResetAt = &until }
	accounts := &qualityRuntimeQuotaAccounts{account: account}
	calls := 0
	svc := NewScheduledTestService(repo, nil)
	svc.templateRepo = &qualityRuntimeQuotaTemplates{}
	runner := &ScheduledTestRunnerService{scheduledSvc: svc, accountTestSvc: &AccountTestService{accountRepo: accounts}, runQualityProbe: func(context.Context, int64, string, string) (*ScheduledTestResult, error) {
		calls++
		return probeAnswer("42"), nil
	}}
	runner.runQualityPlan(context.Background(), qualityProbeRunnerPlan([]string{"a"}, 1))
	require.Zero(t, calls, "a post-selection quota change must prevent the actual provider request")
	require.Zero(t, repo.commits, "quota exhaustion is not a quality verdict")
	require.Zero(t, repo.finishes, "last_run_at must not cover queued 5xx signals when no probe ran")
	require.Equal(t, 1, repo.skips)
}

func TestQualityRuntimeFailedCommitDoesNotAdvanceCompletedSchedule(t *testing.T) {
	repo := &qualityRuntimeQuotaPlans{commitError: errors.New("storage unavailable")}
	runner := &ScheduledTestRunnerService{scheduledSvc: NewScheduledTestService(repo, nil), runQualityProbe: func(context.Context, int64, string, string) (*ScheduledTestResult, error) {
		return probeAnswer("42"), nil
	}}
	runner.runQualityPlan(context.Background(), qualityProbeRunnerPlan([]string{"a"}, 1))
	require.Zero(t, repo.finishes, "failed result persistence cannot consume an upstream-error trigger")
}

func TestQualityRuntimeRestAndTriggerConfiguration(t *testing.T) {
	for _, minutes := range []int{-1, 10081} {
		cfg := cloneQualityPelicanConfig(qualityProbeRunnerPlan([]string{"a"}, 1).PelicanConfig)
		data, err := json.Marshal(map[string]any{"action": "disable_scheduling", "expected_answer": "42", "retest_rest_minutes": minutes, "judge": map[string]string{"kind": "expected"}})
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, cfg.Quality))
		require.Error(t, validateQualityConfig(cfg), "invalid rest intervals must not silently disappear")
	}
	cfg := cloneQualityPelicanConfig(qualityProbeRunnerPlan([]string{"a"}, 1).PelicanConfig)
	require.NoError(t, json.Unmarshal([]byte(`{"retest_rest_minutes":60,"trigger_on_upstream_5xx":true}`), cfg.Quality))
	reusable, err := reusableQualityConfig(cfg)
	require.NoError(t, err)
	encoded, err := json.Marshal(reusable)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"retest_rest_minutes":60`)
	require.Contains(t, string(encoded), `"trigger_on_upstream_5xx":true`)
}
