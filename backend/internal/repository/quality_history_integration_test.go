//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityHistoryPostgreSQLJudgmentAndRoundCompleteness(t *testing.T) {
	ctx := context.Background()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "quality-history-rounds"})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		require.NoError(t, err)
	})
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	config := &service.PelicanTestConfig{
		ModelID: "model", ParallelCount: 2, Prompt: "private question",
		Quality: &service.QualityPolicy{
			Action: "observe_only", ExpectedAnswer: "private answer",
			Judge: &service.QualityJudgeConfig{Kind: "jev", Prompt: "private judge prompt"},
		},
	}
	plan, err := plans.Create(ctx, &service.ScheduledTestPlan{
		AccountID: account.ID, ModelID: "model", CronExpression: "*/30 * * * *",
		Enabled: true, MaxResults: 50, PelicanConfig: config,
	})
	require.NoError(t, err)
	svc := service.NewScheduledTestService(plans, results)
	type probe struct{ status, verdict, action string }
	for _, tc := range []struct {
		name, status string
		probes       []probe
		passed       int
	}{
		{"unknown", "inconclusive", []probe{{"success", "unknown", "inconclusive"}, {"success", "unknown", "inconclusive"}}, 0},
		{"correct_unknown", "inconclusive", []probe{{"success", "correct", "passed"}, {"success", "unknown", "inconclusive"}}, 1},
		{"incorrect_unknown", "failed", []probe{{"failed", "incorrect", "disable_scheduling"}, {"success", "unknown", "inconclusive"}}, 0},
		{"legacy_without_judgment", "inconclusive", []probe{{"success", "", ""}, {"success", "", ""}}, 0},
		{"stale_correct", "inconclusive", []probe{{"success", "correct", "stale_run"}, {"success", "correct", "stale_run"}}, 0},
		{"complete_correct", "success", []probe{{"success", "correct", "passed"}, {"success", "correct", "passed"}}, 2},
		{"partial_correct", "inconclusive", []probe{{"success", "correct", "passed"}}, 1},
		{"all_skipped", "skipped", []probe{{"skipped", "unknown", "inconclusive"}, {"skipped", "unknown", "inconclusive"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, p := range tc.probes {
				var judgment map[string]any
				if p.verdict != "" {
					judgment = map[string]any{"verdict": p.verdict}
				}
				_, err := results.Create(ctx, &service.ScheduledTestResult{
					PlanID: plan.ID, Status: p.status, ResponseText: "private response",
					StartedAt: time.Now(), FinishedAt: time.Now(),
					PelicanConfig: config, QualityRoundID: tc.name,
					QualityAction: p.action, QualityJudgment: judgment,
				})
				require.NoError(t, err)
			}
			page, err := svc.ListQualityHistory(ctx, 0)
			require.NoError(t, err)
			var found *service.QualityHistoryResult
			for _, item := range page.Items {
				if item.PlanID == plan.ID && item.QualityRoundID == tc.name {
					found = item
					break
				}
			}
			require.NotNil(t, found)
			require.Equal(t, tc.status, found.Status)
			require.Equal(t, tc.passed, found.PassedCount)
			require.Len(t, found.ResultIDs, len(tc.probes))
			require.Empty(t, found.ResponseText)
			require.Empty(t, found.PelicanConfig.Prompt)
			require.Empty(t, found.PelicanConfig.Quality.ExpectedAnswer)
			require.Empty(t, found.PelicanConfig.Quality.Judge.Prompt)
			older, err := svc.ListQualityHistory(ctx, found.ID)
			require.NoError(t, err)
			for _, item := range older.Items {
				require.Less(t, item.ID, found.ID)
			}
		})
	}
}
