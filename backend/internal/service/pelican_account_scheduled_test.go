package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type svgRuntimeRepo struct {
	ScheduledTestPlanRepository
	claimed   bool
	committed bool
	commitErr error
	results   []*ScheduledTestResult
}

func (r *svgRuntimeRepo) ClaimPelicanRun(_ context.Context, plan *ScheduledTestPlan, _, _, _ time.Time) (time.Time, bool, error) {
	return plan.UpdatedAt.Add(time.Second), r.claimed, nil
}
func (r *svgRuntimeRepo) CommitPelicanRound(_ context.Context, _ *ScheduledTestPlan, results []*ScheduledTestResult, _ time.Time) (bool, error) {
	r.results = results
	return r.committed, r.commitErr
}

type svgRecoveryRepo struct {
	AccountRepository
	account *Account
	reads   int
}

func (r *svgRecoveryRepo) GetByID(context.Context, int64) (*Account, error) {
	r.reads++
	return r.account, nil
}
func (r *svgRecoveryRepo) ClearError(context.Context, int64) error {
	r.account.Status = StatusActive
	return nil
}
func (r *svgRecoveryRepo) ClearRateLimit(context.Context, int64) error {
	r.account.RateLimitedAt = nil
	return nil
}
func (r *svgRecoveryRepo) ClearAntigravityQuotaScopes(context.Context, int64) error {
	delete(r.account.Extra, "antigravity_quota_scopes")
	return nil
}
func (r *svgRecoveryRepo) ClearModelRateLimits(context.Context, int64) error {
	delete(r.account.Extra, "model_rate_limits")
	return nil
}
func (r *svgRecoveryRepo) ClearTempUnschedulable(context.Context, int64) error {
	r.account.TempUnschedulableUntil = nil
	return nil
}

func TestPelicanAccountScheduledAutoRecoverRequiresSuccessfulCommittedRound(t *testing.T) {
	for _, tt := range []struct {
		name        string
		status      string
		enabled     bool
		committed   bool
		commitErr   error
		wantRecover bool
	}{
		{name: "successful round", status: "success", enabled: true, committed: true, wantRecover: true},
		{name: "failed round", status: "failed", enabled: true, committed: true},
		{name: "option disabled", status: "success", committed: true},
		{name: "edited plan rejects stale result", status: "success", enabled: true},
		{name: "storage failure", status: "success", enabled: true, committed: true, commitErr: errors.New("fixture commit failure")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &svgRuntimeRepo{claimed: true, committed: tt.committed, commitErr: tt.commitErr}
			accounts := &svgRecoveryRepo{account: &Account{ID: 4, Status: StatusError}}
			runner := &ScheduledTestRunnerService{planRepo: repo, rateLimitSvc: &RateLimitService{accountRepo: accounts}, runPelican: func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error) {
				return &ScheduledTestResult{Status: tt.status}, nil
			}}
			plan := &ScheduledTestPlan{ID: 1, AccountID: 4, ModelID: "m", AutoRecover: tt.enabled, CronExpression: "*/30 * * * *", PelicanConfig: &PelicanTestConfig{QuestionKind: "pelican", PromptMode: "random"}}
			require.Equal(t, tt.committed && tt.commitErr == nil, runner.runPelicanPlan(context.Background(), plan))
			if tt.wantRecover {
				require.Equal(t, StatusActive, accounts.account.Status)
				require.Equal(t, 1, accounts.reads)
			} else {
				require.Equal(t, StatusError, accounts.account.Status)
				require.Zero(t, accounts.reads)
			}
		})
	}
}

func TestPelicanAccountScheduledRecoveryPreservesManualPauseAndQualityIsolation(t *testing.T) {
	for _, status := range []string{StatusError, StatusDisabled} {
		t.Run(status, func(t *testing.T) {
			now := time.Now()
			isolation := map[string]any{"m": true}
			accounts := &svgRecoveryRepo{account: &Account{ID: 4, Status: status, Schedulable: false, RateLimitedAt: &now, Extra: map[string]any{QualityModelIsolationKey: isolation, "model_rate_limits": map[string]any{"m": true}}}}
			repo := &svgRuntimeRepo{claimed: true, committed: true}
			runner := &ScheduledTestRunnerService{planRepo: repo, rateLimitSvc: &RateLimitService{accountRepo: accounts}, runPelican: func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error) {
				return &ScheduledTestResult{Status: "success"}, nil
			}}
			plan := &ScheduledTestPlan{ID: 1, AccountID: 4, ModelID: "m", AutoRecover: true, CronExpression: "*/30 * * * *", PelicanConfig: &PelicanTestConfig{QuestionKind: "pelican", PromptMode: "random"}}
			require.True(t, runner.runPelicanPlan(context.Background(), plan))
			require.Nil(t, accounts.account.RateLimitedAt)
			require.NotContains(t, accounts.account.Extra, "model_rate_limits")
			require.False(t, accounts.account.Schedulable)
			require.Equal(t, isolation, accounts.account.Extra[QualityModelIsolationKey])
			if status == StatusDisabled {
				require.Equal(t, StatusDisabled, accounts.account.Status)
			}
		})
	}
}

func TestPelicanAccountScheduledRoundsShareQuestionAcrossAccountsAndSamples(t *testing.T) {
	due := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	var question *PelicanQuestion
	for _, accountID := range []int64{4, 5} {
		repo := &svgRuntimeRepo{claimed: true, committed: true}
		runner := &ScheduledTestRunnerService{planRepo: repo, runPelican: func(_ context.Context, id int64, model string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
			require.Equal(t, accountID, id)
			require.Equal(t, "model", model)
			return &ScheduledTestResult{Status: "success", ResponseText: "<svg></svg>"}, nil
		}}
		cfg := &PelicanTestConfig{QuestionKind: "pelican", PromptMode: "random", ParallelCount: 2}
		plan := &ScheduledTestPlan{ID: 10, AccountID: accountID, ModelID: "model", NextRunAt: &due, UpdatedAt: due, CronExpression: "*/30 * * * *", PelicanConfig: cfg}
		require.True(t, runner.runPelicanPlan(context.Background(), plan))
		require.Len(t, repo.results, 2)
		for _, result := range repo.results {
			if question == nil {
				question = result.PelicanConfig.PelicanQuestion
			}
			require.Equal(t, question, result.PelicanConfig.PelicanQuestion)
			require.Equal(t, intelligenceTestPrompt(result.PelicanConfig), result.PelicanConfig.ActualPrompt)
		}
		require.Nil(t, cfg.PelicanQuestion)
		require.Empty(t, cfg.Prompt)
	}
}
func TestPelicanAccountScheduledClaimPreventsCallsAndFailuresRemainHistory(t *testing.T) {
	repo := &svgRuntimeRepo{claimed: false, committed: true}
	calls := 0
	runner := &ScheduledTestRunnerService{planRepo: repo, runPelican: func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error) {
		calls++
		return nil, errors.New("fixture failure")
	}}
	plan := &ScheduledTestPlan{ID: 1, ModelID: "m", CronExpression: "*/30 * * * *", PelicanConfig: &PelicanTestConfig{QuestionKind: "pelican", PromptMode: "random"}}
	require.False(t, runner.runPelicanPlan(context.Background(), plan))
	require.Zero(t, calls)
	repo.claimed = true
	require.True(t, runner.runPelicanPlan(context.Background(), plan))
	require.Equal(t, 1, calls)
	require.Equal(t, "failed", repo.results[0].Status)
	require.NotNil(t, repo.results[0].PelicanConfig.PelicanQuestion)
}
