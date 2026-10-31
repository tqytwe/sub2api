package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type qualityRoundPersistenceRepo struct {
	ScheduledTestPlanRepository
	results []*ScheduledTestResult
	calls   int
}

func (r *qualityRoundPersistenceRepo) ClaimQualityRun(context.Context, *ScheduledTestPlan, time.Time) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (r *qualityRoundPersistenceRepo) FinishQualityRun(context.Context, int64, time.Time, time.Time, time.Time) error {
	return nil
}
func (r *qualityRoundPersistenceRepo) CommitQualityRun(context.Context, *ScheduledTestPlan, string, *ScheduledTestResult, int) (string, error) {
	panic("single-result persistence must delegate to round")
}
func (r *qualityRoundPersistenceRepo) CommitQualityRound(_ context.Context, _ *ScheduledTestPlan, _ string, results []*ScheduledTestResult, _ int) (string, error) {
	r.calls++
	r.results = results
	return "observed", nil
}

func qualityRoundPrivateSample(model string) *ScheduledTestResult {
	return &ScheduledTestResult{Status: "success", ResponseText: "private response", ErrorMessage: "private provider error", QualityRoundID: "shared-round",
		PelicanConfig: &PelicanTestConfig{ModelID: model, ModelIDs: []string{"first", "second"}, Prompt: "private prompt", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "private answer", Judge: &QualityJudgeConfig{Kind: "jev", Prompt: "private rubric"}}}, QualityJudgment: map[string]any{"verdict": "correct"}}
}
func TestCommitQualityRoundSanitizesEverySampleBeforePersistence(t *testing.T) {
	repo := &qualityRoundPersistenceRepo{}
	svc := NewScheduledTestService(repo, nil)
	plan := &ScheduledTestPlan{ID: 9, MaxResults: 1}
	samples := []*ScheduledTestResult{qualityRoundPrivateSample("first"), qualityRoundPrivateSample("second")}
	_, err := svc.CommitQualityRound(context.Background(), plan, "passed", samples)
	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
	require.Len(t, repo.results, 2)
	for i, sample := range repo.results {
		require.Equal(t, int64(9), sample.PlanID)
		require.Empty(t, sample.ResponseText)
		require.Empty(t, sample.PelicanConfig.Prompt)
		require.Empty(t, sample.PelicanConfig.Quality.ExpectedAnswer)
		require.Empty(t, sample.PelicanConfig.Quality.Judge.Prompt)
		require.Equal(t, "probe_failed", sample.ErrorMessage)
		require.Equal(t, []string{"first", "second"}[i], sample.PelicanConfig.ModelID)
	}
}
func TestCommitQualityRoundValidatesEntireBatchBeforePersisting(t *testing.T) {
	for _, tc := range []string{"missing sample", "different round", "empty batch"} {
		t.Run(tc, func(t *testing.T) {
			repo := &qualityRoundPersistenceRepo{}
			svc := NewScheduledTestService(repo, nil)
			samples := []*ScheduledTestResult{qualityRoundPrivateSample("first"), qualityRoundPrivateSample("second")}
			switch tc {
			case "missing sample":
				samples[1] = nil
			case "different round":
				samples[1].QualityRoundID = "other"
			case "empty batch":
				samples = nil
			}
			_, err := svc.CommitQualityRound(context.Background(), &ScheduledTestPlan{ID: 9}, "passed", samples)
			require.Error(t, err)
			require.Zero(t, repo.calls)
			if len(samples) > 0 {
				require.Equal(t, "private response", samples[0].ResponseText)
			}
		})
	}
}
