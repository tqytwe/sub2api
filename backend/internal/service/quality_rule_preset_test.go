package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type qualityPresetServiceRepo struct {
	ScheduledTestPlanRepository
	presets []*QualityRulePreset
}

func (r *qualityPresetServiceRepo) ListQualityPresets(context.Context) ([]*QualityRulePreset, error) {
	return r.presets, nil
}

func (r *qualityPresetServiceRepo) CreateQualityPreset(_ context.Context, preset *QualityRulePreset) (*QualityRulePreset, error) {
	preset.ID = int64(len(r.presets) + 1)
	r.presets = append(r.presets, preset)
	return preset, nil
}

func qualityPresetInput() *QualityRulePreset {
	return &QualityRulePreset{
		Name: "Reusable check", ModelID: "test-model", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 25,
		PelicanConfig: &PelicanTestConfig{
			RuleName: "source-only name", ModelID: "result-only model", QuestionKind: "text", TestChannel: "account",
			Prompt: "What is 6 times 7?", ReasoningEffort: "high", ParallelCount: 2, ModelIDs: []string{"extra-model"},
			Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "42", Judge: &QualityJudgeConfig{Kind: "expected"}},
		},
	}
}

func TestCreateQualityPresetStoresOnlyReusableQualityConfiguration(t *testing.T) {
	repo := &qualityPresetServiceRepo{}
	svc := NewScheduledTestService(repo, nil)
	input := qualityPresetInput()
	saved, err := svc.CreateQualityPreset(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.ID)
	require.Equal(t, "Reusable check", saved.Name)
	require.Empty(t, saved.PelicanConfig.RuleName)
	require.Empty(t, saved.PelicanConfig.ModelID)
	require.Equal(t, "What is 6 times 7?", saved.PelicanConfig.Prompt)
	require.Equal(t, "42", saved.PelicanConfig.Quality.ExpectedAnswer)
	require.Equal(t, []string{"extra-model"}, saved.PelicanConfig.ModelIDs)
	saved.PelicanConfig.ModelIDs[0] = "changed"
	require.Equal(t, "extra-model", input.PelicanConfig.ModelIDs[0])
	require.Len(t, repo.presets, 1)
	listed, err := svc.ListQualityPresets(context.Background())
	require.NoError(t, err)
	require.Equal(t, []*QualityRulePreset{saved}, listed)
}

func TestCreateQualityPresetRejectsInvalidInputsBeforePersistence(t *testing.T) {
	changes := []struct {
		name   string
		change func(*QualityRulePreset)
	}{
		{"missing name", func(p *QualityRulePreset) { p.Name = " " }},
		{"long name", func(p *QualityRulePreset) { p.Name = strings.Repeat("预", 101) }},
		{"missing model", func(p *QualityRulePreset) { p.ModelID = " " }},
		{"bad cron", func(p *QualityRulePreset) { p.CronExpression = "not a cron" }},
		{"missing quality config", func(p *QualityRulePreset) { p.PelicanConfig.Quality = nil }},
		{"invalid action", func(p *QualityRulePreset) { p.PelicanConfig.Quality.Action = "create_plan" }},
		{"invalid judge", func(p *QualityRulePreset) { p.PelicanConfig.Quality.Judge = nil }},
		{"oversized prompt", func(p *QualityRulePreset) { p.PelicanConfig.Prompt = strings.Repeat("x", 32769) }},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			repo := &qualityPresetServiceRepo{}
			svc := NewScheduledTestService(repo, nil)
			input := qualityPresetInput()
			tc.change(input)
			_, err := svc.CreateQualityPreset(context.Background(), input)
			require.Error(t, err)
			require.Empty(t, repo.presets)
		})
	}
}

func TestCreateQualityPresetDefaultsResultRetentionLikePlanCreation(t *testing.T) {
	repo := &qualityPresetServiceRepo{}
	input := qualityPresetInput()
	input.MaxResults = 0
	saved, err := NewScheduledTestService(repo, nil).CreateQualityPreset(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 50, saved.MaxResults)
}

func TestQualityPresetUnavailableWhenPlanRepositoryDoesNotSupportPresets(t *testing.T) {
	_, err := NewScheduledTestService(nil, nil).ListQualityPresets(context.Background())
	require.Error(t, err)
}
