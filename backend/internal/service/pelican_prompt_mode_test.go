package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPelicanGroupRandomQuestionInput(t *testing.T) {
	for _, mode := range []string{"random", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			var input PelicanGroupTestPlanInput
			require.NoError(t, json.Unmarshal([]byte(`{"group_id":4,"model_id":"gpt-6-astra","prompt_mode":"`+mode+`","reasoning_effort":"medium","cron_expression":"*/30 * * * *","enabled":true}`), &input))
			svc, _ := newGroupTestService(newGroupTestRepoFake(), nil, nil)
			plan, err := svc.CreatePlan(context.Background(), input)
			if mode == "invalid" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "random mode generates its own prompt and must not require a custom one")
			body, err := json.Marshal(plan.PelicanConfig)
			require.NoError(t, err)
			require.Contains(t, string(body), `"prompt_mode":"random"`)
			require.NotContains(t, string(body), `"pelican_question"`, "stored reusable rules must not contain a frozen round")
		})
	}
}

func TestPelicanGroupRejectsInvalidPromptModeWithCustomPrompt(t *testing.T) {
	var input PelicanGroupTestPlanInput
	require.NoError(t, json.Unmarshal([]byte(`{"group_id":4,"model_id":"gpt-6-astra","prompt":"draw","prompt_mode":"typo","reasoning_effort":"medium"}`), &input))
	svc, _ := newGroupTestService(newGroupTestRepoFake(), nil, nil)
	_, err := svc.CreatePlan(context.Background(), input)
	require.Error(t, err, "unknown modes must not be silently treated as custom prompts")
}

func TestPelicanGroupOldEditorPreservesRandomMode(t *testing.T) {
	current := groupTestPlan(1)
	current.PelicanConfig.PromptMode = "random"
	repo := newGroupTestRepoFake(current)
	svc, _ := newGroupTestService(repo, nil, nil)
	updated, err := svc.UpdatePlan(context.Background(), current.ID, PelicanGroupTestPlanInput{ModelID: "fixture-model", CronExpression: "*/30 * * * *", ReasoningEffort: "medium", Enabled: false})
	require.NoError(t, err)
	require.Equal(t, "random", updated.PelicanConfig.PromptMode)
}
