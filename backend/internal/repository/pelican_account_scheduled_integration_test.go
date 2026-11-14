//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPelicanAccountRoundLeaseAndAtomicSnapshots(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	account := mustCreateAccount(t, client, &service.Account{Name: "svg-account-round", Platform: "openai", Type: "apikey", Credentials: map[string]any{"api_key": "synthetic-test-value"}})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	repo := NewScheduledTestPlanRepository(integrationDB).(*scheduledTestPlanRepository)
	due := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	plan, err := repo.Create(ctx, &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "fixture-model", Enabled: true, MaxResults: 10, CronExpression: "*/30 * * * *", NextRunAt: &due, PelicanConfig: &service.PelicanTestConfig{QuestionKind: "pelican", PromptMode: "random"}})
	require.NoError(t, err)
	now := time.Now()
	until := now.Add(time.Minute).Truncate(time.Microsecond)
	version, claimed, err := repo.ClaimPelicanRun(ctx, plan, now, until, now.Add(30*time.Minute))
	require.NoError(t, err)
	require.True(t, claimed)
	_, claimed, err = repo.ClaimPelicanRun(ctx, plan, now, until, now.Add(30*time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "stale replica cannot duplicate a round")
	question, err := service.NewPelicanQuestion()
	require.NoError(t, err)
	snapshot := *plan
	snapshot.UpdatedAt = version
	snapshot.RunningUntil = &until
	result := &service.ScheduledTestResult{PlanID: plan.ID, Status: "success", ResponseText: "<svg></svg>", StartedAt: now, FinishedAt: now, PelicanConfig: question}
	committed, err := repo.CommitPelicanRound(ctx, &snapshot, []*service.ScheduledTestResult{result}, now)
	require.NoError(t, err)
	require.True(t, committed)
	saved, err := NewScheduledTestResultRepository(integrationDB).ListByPlanID(ctx, plan.ID, 10)
	require.NoError(t, err)
	require.Len(t, saved, 1)
	require.Equal(t, question.PelicanQuestion, saved[0].PelicanConfig.PelicanQuestion)
	committed, err = repo.CommitPelicanRound(ctx, &snapshot, []*service.ScheduledTestResult{result}, now)
	require.NoError(t, err)
	require.False(t, committed, "completed lease cannot duplicate history")
	// A real edit invalidates a running snapshot. The history stays unchanged.
	_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET next_run_at=$2 WHERE id=$1`, plan.ID, due)
	require.NoError(t, err)
	plan, err = repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	version, claimed, err = repo.ClaimPelicanRun(ctx, plan, now, until, now.Add(30*time.Minute))
	require.NoError(t, err)
	require.True(t, claimed)
	snapshot = *plan
	snapshot.UpdatedAt = version
	snapshot.RunningUntil = &until
	_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false,updated_at=clock_timestamp() WHERE id=$1`, plan.ID)
	require.NoError(t, err)
	committed, err = repo.CommitPelicanRound(ctx, &snapshot, []*service.ScheduledTestResult{result}, now)
	require.NoError(t, err)
	require.False(t, committed)
	saved, err = NewScheduledTestResultRepository(integrationDB).ListByPlanID(ctx, plan.ID, 10)
	require.NoError(t, err)
	require.Len(t, saved, 1)
}
