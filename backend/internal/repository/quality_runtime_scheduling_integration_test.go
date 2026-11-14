//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func qualityRuntimeSchedulingFixture(t *testing.T) (*scheduledTestPlanRepository, *service.ScheduledTestPlan) {
	t.Helper()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "quality-runtime-scheduling", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth})
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE account_id=$1`, account.ID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, err)
	})
	repo := &scheduledTestPlanRepository{db: integrationDB}
	due := time.Now().Add(-time.Minute)
	plan, err := repo.Create(context.Background(), &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "first", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 20, NextRunAt: &due, PelicanConfig: &service.PelicanTestConfig{ModelIDs: []string{"first"}, Quality: &service.QualityPolicy{Action: "disable_scheduling", RetestRestMinutes: 60, TriggerOnUpstream5xx: true}}})
	require.NoError(t, err)
	return repo, plan
}

func claimQualityRuntimeScheduling(t *testing.T, repo *scheduledTestPlanRepository, plan *service.ScheduledTestPlan) {
	t.Helper()
	until := time.Now().Add(time.Minute).Truncate(time.Microsecond)
	version, claimed, err := repo.ClaimQualityRun(context.Background(), plan, until)
	require.NoError(t, err)
	require.True(t, claimed)
	plan.UpdatedAt = version
	plan.RunningUntil = &until
}

func TestQualityRuntimePostgreSQLManualMarkerAndSiblingLease(t *testing.T) {
	ctx := context.Background()
	repo, plan := qualityRuntimeSchedulingFixture(t)
	// An account may have one rule per action scope; use the existing observation
	// scope to exercise cross-rule exclusion without violating that contract.
	siblingConfig := &service.PelicanTestConfig{ModelIDs: []string{"first"}, Quality: &service.QualityPolicy{Action: "observe_only"}}
	sibling, err := repo.Create(ctx, &service.ScheduledTestPlan{AccountID: plan.AccountID, ModelID: plan.ModelID, CronExpression: plan.CronExpression, Enabled: true, MaxResults: 20, PelicanConfig: siblingConfig})
	require.NoError(t, err)
	require.NoError(t, repo.TriggerQuality(ctx, plan.ID))
	plan, err = repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.True(t, plan.ManualQualityRun)
	claimQualityRuntimeScheduling(t, repo, plan)
	current, err := repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.False(t, current.ManualQualityRun, "manual quota bypass is one-shot")
	_, claimed, err := repo.ClaimQualityRun(ctx, sibling, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "one account cannot run overlapping rules")
	stale := *plan
	stale.UpdatedAt = plan.UpdatedAt.Add(-time.Second)
	require.ErrorIs(t, repo.FinishQualityPlan(ctx, &stale, time.Now(), time.Now().Add(time.Hour)), sql.ErrNoRows)
	require.NoError(t, repo.FinishQualityPlan(ctx, plan, time.Now(), time.Now().Add(time.Hour)))
	claimQualityRuntimeScheduling(t, repo, sibling)
}

func TestQualityRuntimePostgreSQLRestBlocksEveryTriggerAndPreservesSchedule(t *testing.T) {
	ctx := context.Background()
	repo, plan := qualityRuntimeSchedulingFixture(t)
	claimQualityRuntimeScheduling(t, repo, plan)
	sample := qualityRoundSample(plan, "first")
	sample.Status = "failed"
	sample.QualityJudgment = map[string]any{"verdict": "incorrect"}
	action, err := repo.CommitQualityRound(ctx, plan, "failed", []*service.ScheduledTestResult{sample}, 20)
	require.NoError(t, err)
	require.Equal(t, "scheduling_disabled", action)
	var deadline time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT (state->>'retest_after')::timestamptz FROM account_quality_states WHERE plan_id=$1`, plan.ID).Scan(&deadline))
	require.True(t, deadline.After(time.Now().Add(59*time.Minute)))
	require.NoError(t, repo.FinishQualityPlan(ctx, plan, time.Now(), time.Now().Add(time.Minute)))
	current, err := repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.False(t, current.NextRunAt.Before(deadline))
	require.Error(t, repo.TriggerQuality(ctx, plan.ID))
	observed := time.Now().Add(time.Second)
	current.TriggerSource = "upstream_5xx"
	current.TriggerObservedAt = &observed
	_, claimed, err := repo.ClaimQualityRun(ctx, current, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "5xx cannot bypass owned rest")
	_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(state,'{retest_after}',to_jsonb(clock_timestamp()-interval '1 second')) WHERE plan_id=$1`, plan.ID)
	require.NoError(t, err)
	claimQualityRuntimeScheduling(t, repo, current)
	var schedulable bool
	require.NoError(t, integrationDB.QueryRow(`SELECT schedulable FROM accounts WHERE id=$1`, plan.AccountID).Scan(&schedulable))
	require.False(t, schedulable, "rest expiration itself never restores scheduling")
}

func TestQualityRuntimePostgreSQLQuotaSkipKeepsCoverageAndFencesLease(t *testing.T) {
	ctx := context.Background()
	repo, plan := qualityRuntimeSchedulingFixture(t)
	claimQualityRuntimeScheduling(t, repo, plan)
	stale := *plan
	wrong := plan.RunningUntil.Add(time.Second)
	stale.RunningUntil = &wrong
	require.ErrorIs(t, repo.FinishQuotaSkippedQuality(ctx, &stale, time.Now().Add(time.Hour)), sql.ErrNoRows)
	require.NoError(t, repo.FinishQuotaSkippedQuality(ctx, plan, time.Now().Add(time.Hour)))
	current, err := repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Nil(t, current.RunningUntil)
	require.Nil(t, current.LastRunAt)
	var results, states int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM scheduled_test_results WHERE plan_id=$1`, plan.ID).Scan(&results))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM account_quality_states WHERE plan_id=$1`, plan.ID).Scan(&states))
	require.Zero(t, results)
	require.Zero(t, states)
}
