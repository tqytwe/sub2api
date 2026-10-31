package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type qualityTargetPlanRepo struct {
	ScheduledTestPlanRepository
	saved *ScheduledTestPlan
}

func TestQualityProbeTargetsDoNotChangeConnectivityPlans(t *testing.T) {
	models := make([]string, 51)
	for i := range models {
		models[i] = fmt.Sprintf("model-%d", i)
	}
	for _, ids := range [][]string{{"quality-model"}, models} {
		for _, update := range []bool{false, true} {
			plan := &ScheduledTestPlan{ModelID: "connectivity-model", CronExpression: "0 * * * *", PelicanConfig: &PelicanTestConfig{ModelIDs: ids, ParallelCount: 32}}
			repo := &qualityTargetPlanRepo{}
			svc := NewScheduledTestService(repo, nil)
			var err error
			if update {
				_, err = svc.UpdatePlan(context.Background(), plan)
			} else {
				_, err = svc.CreatePlan(context.Background(), plan)
			}
			require.NoError(t, err)
			require.Equal(t, "connectivity-model", repo.saved.ModelID)
			require.Equal(t, ids, repo.saved.PelicanConfig.ModelIDs)
		}
	}
}

func (r *qualityTargetPlanRepo) Create(_ context.Context, p *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	r.saved = p
	return p, nil
}
func (r *qualityTargetPlanRepo) Update(_ context.Context, p *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	r.saved = p
	return p, nil
}

func TestQualityProbeTargetValidationBoundsActualRoundCost(t *testing.T) {
	for _, cfg := range []*PelicanTestConfig{
		{ModelIDs: []string{strings.Repeat("m", 101)}},
		{ModelIDs: []string{"a", "b", "c", "d"}, ParallelCount: 32},
	} {
		cfg.Prompt = "question"
		cfg.Quality = &QualityPolicy{Action: "observe_only", ExpectedAnswer: "42", Judge: &QualityJudgeConfig{Kind: "expected"}}
		require.Error(t, validateQualityConfig(cfg))
	}
	models := make([]string, 51)
	for i := range models {
		models[i] = fmt.Sprintf("model-%d", i)
	}
	require.Error(t, validateQualityConfig(&PelicanTestConfig{ModelIDs: models, Prompt: "question", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "42", Judge: &QualityJudgeConfig{Kind: "expected"}}}))
	require.NoError(t, validateQualityConfig(&PelicanTestConfig{ParallelCount: 32}))
	require.NoError(t, validateQualityConfig(&PelicanTestConfig{ParallelCount: 0, ModelIDs: []string{"a", "b"}}))
}

func TestQualityProbeTargetsNormalizeAtPlanWriteWithoutChangingCallerConfig(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprint(update), func(t *testing.T) {
			original := &PelicanTestConfig{ModelIDs: []string{" second ", "second", "first", " "}, ParallelCount: 2, Prompt: "question", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "42", Judge: &QualityJudgeConfig{Kind: "expected"}}}
			plan := &ScheduledTestPlan{ModelID: "old", CronExpression: "0 * * * *", PelicanConfig: original}
			repo := &qualityTargetPlanRepo{}
			svc := NewScheduledTestService(repo, nil)
			var err error
			if update {
				_, err = svc.UpdatePlan(context.Background(), plan)
			} else {
				_, err = svc.CreatePlan(context.Background(), plan)
			}
			require.NoError(t, err)
			require.Equal(t, "second", repo.saved.ModelID)
			require.Equal(t, []string{"second", "first"}, repo.saved.PelicanConfig.ModelIDs)
			require.Equal(t, []string{" second ", "second", "first", " "}, original.ModelIDs)
		})
	}
}

func TestQualityProbeTargetTemplateValidationMatchesDirectPlan(t *testing.T) {
	template := templateFixture()
	template.PelicanConfig.ParallelCount = 32
	template.PelicanConfig.ModelIDs = []string{"a", "b", "c", "d"}
	require.Error(t, prepareQualityTemplate(template))
	template = templateFixture()
	template.PelicanConfig.ModelIDs = []string{" first ", "first", "second"}
	require.NoError(t, prepareQualityTemplate(template))
	require.Equal(t, "first", template.ModelID)
	require.Equal(t, []string{"first", "second"}, template.PelicanConfig.ModelIDs)
}
