package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPelicanQuestionFrozenRounds(t *testing.T) {
	plan := &PelicanTestConfig{QuestionKind: "pelican", PromptMode: PelicanPromptModeRandom, ParallelCount: 3}
	for index := range pelicanQuestionBank {
		calls := 0
		round := freezePelicanQuestion(plan, "round", func(size int) int {
			calls++
			require.Equal(t, 4, size)
			return index
		})
		require.Equal(t, 1, calls)
		require.Equal(t, pelicanQuestionBank[index].id, round.PelicanQuestion.ID)
		require.Len(t, round.PelicanQuestion.Criteria, 4)
		require.Contains(t, round.ActualPrompt, PelicanDeliveryContract)
		body, err := json.Marshal(round)
		require.NoError(t, err)
		var saved PelicanTestConfig
		require.NoError(t, json.Unmarshal(body, &saved))
		require.Equal(t, round, &saved)
	}
	require.Empty(t, plan.Prompt)
	require.Nil(t, plan.PelicanQuestion)
	require.Empty(t, plan.ActualPrompt)
}

func TestPelicanQuestionManualIdentityRebuildsUntrustedText(t *testing.T) {
	manual, err := NewPelicanQuestion()
	require.NoError(t, err)
	original := *manual.PelicanQuestion
	manual.PelicanQuestion.Prompt = "client forged prompt"
	manual.PelicanQuestion.Criteria = []string{"client forged rubric"}
	resolved, err := ResolveManualPelicanQuestion(manual.PelicanQuestion)
	require.NoError(t, err)
	require.Equal(t, original.Prompt, resolved.PelicanQuestion.Prompt)
	require.NotContains(t, resolved.PelicanQuestion.Criteria, "client forged rubric")
	for _, mutation := range []func(*PelicanQuestion){
		func(q *PelicanQuestion) { q.Version = "future" },
		func(q *PelicanQuestion) { q.ID = "unknown" },
		func(q *PelicanQuestion) { q.RoundID = "scheduled:123" },
		func(q *PelicanQuestion) { q.RoundID = "manual:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz" },
	} {
		question := original
		mutation(&question)
		_, err := ResolveManualPelicanQuestion(&question)
		require.Error(t, err)
	}
	_, err = ResolveManualPelicanQuestion(nil)
	require.Error(t, err)
}

func TestPelicanQuestionLegacyCustomAndPlanMetadata(t *testing.T) {
	for _, mode := range []string{"", "custom"} {
		cfg := freezePelicanQuestion(&PelicanTestConfig{Prompt: "my custom SVG", PromptMode: mode}, "round", func(int) int { t.Fatal("custom must not select"); return 0 })
		require.Equal(t, "my custom SVG", cfg.Prompt)
		require.Nil(t, cfg.PelicanQuestion)
	}
	for _, cfg := range []*PelicanTestConfig{
		{PromptMode: "typo"},
		{PromptMode: PelicanPromptModeRandom, QuestionKind: "candy"},
		{PromptMode: PelicanPromptModeRandom, Quality: &QualityPolicy{}},
	} {
		require.Error(t, validatePelicanPromptMode(cfg))
	}
	original := &PelicanTestConfig{QuestionKind: "pelican", PromptMode: "random", PelicanQuestion: &PelicanQuestion{ID: "untrusted"}, ActualPrompt: "untrusted"}
	plan := &ScheduledTestPlan{ModelID: "model", CronExpression: "*/30 * * * *", PelicanConfig: original}
	require.NoError(t, prepareQualityPlan(plan))
	require.Nil(t, plan.PelicanConfig.PelicanQuestion)
	require.Empty(t, plan.PelicanConfig.ActualPrompt)
	require.NotNil(t, original.PelicanQuestion, "validation must not mutate the caller's reusable config")
}

func TestPelicanRandomGroupSamplesShareNominalDueQuestion(t *testing.T) {
	plan := groupTestPlan(2)
	plan.PelicanConfig.PromptMode = PelicanPromptModeRandom
	due := groupTestNow.Add(-time.Minute)
	plan.NextRunAt = &due
	original := plan.PelicanConfig
	repo := newGroupTestRepoFake(plan)
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "fixture-a")}, {account: account(12, "fixture-b")}}}
	svc, _ := newGroupTestService(repo, router, func(_ context.Context, _ int64, _ string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
		require.NotNil(t, cfg.PelicanQuestion)
		return &ScheduledTestResult{Status: "success", ResponseText: "<svg></svg>"}, nil
	})
	svc.runScheduled(context.Background(), plan, groupTestNow)
	require.Len(t, repo.results, 2)
	expected := freezePelicanQuestion(&PelicanTestConfig{PromptMode: PelicanPromptModeRandom}, "scheduled:"+due.UTC().Format(time.RFC3339Nano), nil)
	for _, result := range repo.results {
		require.Equal(t, expected.PelicanQuestion, result.PelicanConfig.PelicanQuestion)
		require.Equal(t, expected.ActualPrompt, result.PelicanConfig.ActualPrompt)
	}
	require.Nil(t, original.PelicanQuestion)
	require.Equal(t, "draw", original.Prompt)
}
