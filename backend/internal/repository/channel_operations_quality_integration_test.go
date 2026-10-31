//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelOperationsQualityRoundsUseFrozenModelsAndCurrentChannelScope(t *testing.T) {
	ctx := context.Background()
	channels := &channelRepository{db: integrationDB}
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	channel := &service.Channel{Name: fmt.Sprintf("quality-ops-%d", time.Now().UnixNano()), Status: service.StatusActive, BillingModelSource: service.BillingModelSourceRequested}
	require.NoError(t, channels.Create(ctx, channel))
	var groupID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'openai') RETURNING id`, fmt.Sprintf("quality-ops-%d", channel.ID)).Scan(&groupID))
	require.NoError(t, channels.SetGroupIDs(ctx, channel.ID, []int64{groupID}))
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("quality-ops-%d", channel.ID), Platform: service.PlatformOpenAI})
	outsider := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("quality-ops-outsider-%d", channel.ID), Platform: service.PlatformOpenAI})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, account.ID, groupID)
	require.NoError(t, err)
	baseConfig := &service.PelicanTestConfig{
		ModelID: "model-a", ModelIDs: []string{"model-a", "model-b"}, ParallelCount: 1,
		Quality: &service.QualityPolicy{Action: "observe_only"},
	}
	plan, err := plans.Create(ctx, &service.ScheduledTestPlan{
		AccountID: account.ID, ModelID: "model-a", CronExpression: "*/30 * * * *",
		Enabled: true, MaxResults: 50, PelicanConfig: baseConfig,
	})
	require.NoError(t, err)
	outsiderPlan, err := plans.Create(ctx, &service.ScheduledTestPlan{
		AccountID: outsider.ID, ModelID: "outsider", CronExpression: "*/30 * * * *",
		Enabled: true, MaxResults: 10, PelicanConfig: &service.PelicanTestConfig{ModelID: "outsider", ParallelCount: 1, Quality: &service.QualityPolicy{Action: "observe_only"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, planID := range []int64{plan.ID, outsiderPlan.ID} {
			_, _ = integrationDB.ExecContext(ctx, `DELETE FROM scheduled_test_results WHERE plan_id=$1`, planID)
			_ = plans.Delete(ctx, planID)
		}
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id IN ($1,$2)`, account.ID, outsider.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id IN ($1,$2)`, account.ID, outsider.ID)
		_ = channels.Delete(ctx, channel.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, groupID)
	})

	add := func(planID int64, round, model, status, verdict, action, message string) int64 {
		t.Helper()
		config := *baseConfig
		config.ModelID = model
		if planID == outsiderPlan.ID {
			config.ModelIDs = nil
		}
		judgment := map[string]any{"verdict": verdict}
		if verdict == "" {
			judgment = nil
		}
		at := time.Now().UTC()
		result, createErr := results.Create(ctx, &service.ScheduledTestResult{
			PlanID: planID, Status: status, ErrorMessage: message, StartedAt: at, FinishedAt: at,
			PelicanConfig: &config, QualityRoundID: round, QualityAction: action, QualityJudgment: judgment,
		})
		require.NoError(t, createErr)
		return result.ID
	}

	// Complete two-model pass, complete degraded round, and a partial round.
	add(plan.ID, "pass", "model-a", "success", "correct", "observed", "")
	add(plan.ID, "pass", "model-b", "success", "correct", "observed", "")
	add(plan.ID, "degraded", "model-a", "failed", "incorrect", "failure_counted:1/1", "answer_mismatch")
	add(plan.ID, "degraded", "model-b", "success", "correct", "observed", "")
	partialModelID := add(plan.ID, "partial", "model-a", "success", "correct", "observed", "")
	add(outsiderPlan.ID, "outside-channel", "outsider", "success", "correct", "observed", "")

	from := time.Now().Add(-time.Minute).UTC()
	until := time.Now().Add(time.Minute).UTC()
	filter := service.ChannelOperationsFilter{
		ChannelID: channel.ID, From: from, Until: until, ModelScope: "upstream",
		Dimension: "channel", Page: 1, PageSize: 2,
	}
	type row struct {
		ID       int64  `json:"id"`
		Model    string `json:"measured_model"`
		Outcome  string `json:"outcome"`
		Complete bool   `json:"round_complete"`
	}
	type response struct {
		Summary struct {
			Rounds       int `json:"rounds"`
			Passed       int `json:"passed"`
			Degraded     int `json:"degraded"`
			Inconclusive int `json:"inconclusive"`
		} `json:"summary"`
		Rows    []row `json:"rows"`
		HasMore bool  `json:"has_more"`
	}
	read := func() response {
		t.Helper()
		section := channels.ReadChannelOperations(ctx, filter).Quality
		require.Equal(t, "ready", section.Status, section.Error)
		var got response
		require.NoError(t, json.Unmarshal(section.Data, &got))
		return got
	}
	got := read()
	require.Equal(t, 5, got.Summary.Rounds)
	require.Equal(t, 3, got.Summary.Passed)
	require.Equal(t, 1, got.Summary.Degraded)
	require.Equal(t, 1, got.Summary.Inconclusive)
	require.Len(t, got.Rows, 2)
	require.True(t, got.HasMore)

	filter.Model = "model-a"
	modelA := read()
	require.Equal(t, 3, modelA.Summary.Rounds)
	require.Equal(t, 1, modelA.Summary.Passed)
	require.Equal(t, 1, modelA.Summary.Degraded)
	require.Equal(t, 1, modelA.Summary.Inconclusive)
	var foundPartial bool
	for _, item := range modelA.Rows {
		if item.ID == partialModelID {
			foundPartial = true
			require.Equal(t, "inconclusive", item.Outcome)
			require.False(t, item.Complete)
		}
	}
	require.True(t, foundPartial)

	filter.Model = "public-model"
	filter.ModelScope = "public"
	unsupported := channels.ReadChannelOperations(ctx, filter).Quality
	require.Equal(t, "unsupported_scope", unsupported.Status)
	filter.Model = ""
	filter.ModelScope = "upstream"
	filter.GroupID = groupID + 999999
	require.Zero(t, read().Summary.Rounds)
	filter.GroupID = groupID
	filter.AccountID = outsider.ID
	require.Zero(t, read().Summary.Rounds)
	filter.AccountID = 0

	filter.Model = ""
	filter.QualityBefore = got.Rows[1].ID
	older := read()
	require.Len(t, older.Rows, 2)
	require.Less(t, older.Rows[0].ID, got.Rows[1].ID)
	require.Equal(t, 5, older.Summary.Rounds)
}
