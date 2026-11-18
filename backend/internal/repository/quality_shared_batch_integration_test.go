//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityClosureMigrationsAreIdempotent(t *testing.T) {
	for _, name := range []string{"370_quality_shared_batches.sql", "371_quality_oauth_401_streaks.sql", "372_quality_oauth_401_trusted_revocation.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		for range 2 {
			_, err = integrationDB.ExecContext(context.Background(), string(sql))
			require.NoError(t, err, name)
		}
	}
}

func TestQualitySharedBatchPostgreSQLDelayedCalibratedAction(t *testing.T) {
	for _, scenario := range []string{"all wrong", "passing peer", "changed credentials", "paused peer", "changed mapping", "finished peer first", "expired pending peer", "late collected"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			plans := NewScheduledTestPlanRepository(integrationDB)
			repo := plans.(service.QualitySharedBatchRepository)
			resultsRepo := NewScheduledTestResultRepository(integrationDB)
			svc := service.NewScheduledTestService(plans, resultsRepo)
			start := time.Now().UTC().Truncate(time.Minute)
			batch := &service.QualitySharedBatch{ID: fmt.Sprintf("fixture-%d", time.Now().UnixNano()), StartedAt: start, StartDeadline: start.Add(2 * time.Minute), ClosesAt: start.Add(12 * time.Minute), Questions: []service.QualityBatchQuestion{{Prompt: "generated fixture question", Expected: "21", Question: &service.QualityRandomQuestion{ID: "fixture", Family: "paths", Version: "1"}}}}
			t.Cleanup(func() {
				_, e := integrationDB.ExecContext(ctx, `DELETE FROM quality_probe_batches WHERE id=$1`, batch.ID)
				require.NoError(t, e)
			})
			var runs []*service.QualityBatchRun
			for i := 0; i < 2; i++ {
				account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "shared-batch", Platform: "openai", Type: "apikey", Credentials: map[string]any{"api_key": "fixture-only"}})
				t.Cleanup(func() {
					_, e := integrationDB.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE account_id=$1`, account.ID)
					require.NoError(t, e)
					_, e = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
					require.NoError(t, e)
				})
				cfg := &service.PelicanTestConfig{QuestionKind: service.QualityRandomQuestionKind, ModelIDs: []string{"model"}, ParallelCount: 1, Quality: &service.QualityPolicy{Action: "disable_scheduling", Judge: &service.QualityJudgeConfig{Kind: "expected"}}}
				plan, e := plans.Create(ctx, &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "model", Enabled: true, CronExpression: "*/30 * * * *", MaxResults: 1, PelicanConfig: cfg})
				require.NoError(t, e)
				until := time.Now().Add(15 * time.Minute).Truncate(time.Microsecond)
				version, claimed, e := svc.ClaimQualityRun(ctx, plan, until)
				require.NoError(t, e)
				require.True(t, claimed)
				plan.UpdatedAt, plan.RunningUntil = version, &until
				joined, e := repo.StartQualityBatch(ctx, batch, plan, until)
				require.NoError(t, e)
				require.Equal(t, batch.Questions, joined.Questions)
				// A repeated instance cannot rejoin and issue another paid probe.
				_, e = repo.StartQualityBatch(ctx, batch, plan, until)
				require.Error(t, e)
				passed := i == 1 && scenario != "all wrong"
				status, verdict, message := "failed", "incorrect", "answer_mismatch"
				if passed {
					status, verdict, message = "success", "correct", ""
				}
				sample := &service.ScheduledTestResult{PlanID: plan.ID, Status: status, ErrorMessage: message, QualityRoundID: fmt.Sprintf("fixture-%d", plan.ID), QualityJudgment: map[string]any{"verdict": verdict}, StartedAt: time.Now(), FinishedAt: time.Now(), PelicanConfig: &service.PelicanTestConfig{QuestionKind: service.QualityRandomQuestionKind, ModelID: "model", RandomQuestion: batch.Questions[0].Question, Quality: cfg.Quality, QualityBatch: &service.QualityBatchEvidence{ID: batch.ID, Route: "openai:account", MappedModel: "model", StartedAt: batch.StartedAt, StartDeadline: batch.StartDeadline, ClosesAt: batch.ClosesAt, ComparisonKey: "fixture-shared-comparison", Decision: "pending"}}}
				run := &service.QualityBatchRun{Apply: true, Plan: plan, Until: until, Results: []*service.ScheduledTestResult{sample}}
				if scenario == "expired pending peer" && i == 1 {
					runs = append(runs, run)
					continue
				}
				require.NoError(t, repo.CollectQualityBatch(ctx, run))
				require.NoError(t, repo.CollectQualityBatch(ctx, run))
				history, e := resultsRepo.ListByPlanID(ctx, plan.ID, 10)
				require.NoError(t, e)
				require.Len(t, history, 1)
				require.Equal(t, "comparison_pending", history[0].QualityAction)
				runs = append(runs, run)
			}
			var schedulable bool
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT schedulable FROM accounts WHERE id=$1`, runs[0].Plan.AccountID).Scan(&schedulable))
			require.True(t, schedulable)
			if scenario == "changed credentials" {
				_, e := integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials='{"api_key":"rotated"}' WHERE id=$1`, runs[1].Plan.AccountID)
				require.NoError(t, e)
			}
			if scenario == "changed mapping" {
				_, e := integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=credentials||'{"model_mapping":{"model":"other"}}' WHERE id=$1`, runs[1].Plan.AccountID)
				require.NoError(t, e)
			}
			if scenario == "paused peer" {
				_, e := integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false WHERE id=$1`, runs[1].Plan.ID)
				require.NoError(t, e)
			}
			// Mutations happen before delayed comparison: changed credentials,
			// mapping, or a paused peer must not calibrate this wrong result.
			_, e := integrationDB.ExecContext(ctx, `UPDATE quality_probe_batches SET start_deadline=clock_timestamp()-interval '1 second' WHERE id=$1`, batch.ID)
			require.NoError(t, e)
			if scenario == "expired pending peer" {
				expired := time.Now().Add(-time.Second)
				_, e = integrationDB.ExecContext(ctx, `UPDATE quality_probe_batch_runs SET lease_until=$3 WHERE batch_id=$1 AND plan_id=$2`, batch.ID, runs[1].Plan.ID, expired)
				require.NoError(t, e)
				_, e = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET running_until=$2 WHERE id=$1`, runs[1].Plan.ID, expired)
				require.NoError(t, e)
				_, e = integrationDB.ExecContext(ctx, `UPDATE quality_probe_batches SET started_at=clock_timestamp()-interval '14 minutes',start_deadline=clock_timestamp()-interval '12 minutes',closes_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, batch.ID)
				require.NoError(t, e)
				// Complete expired peer cleanup first; its empty pending snapshot
				// must never prevent another collected account from finalizing.
				require.NoError(t, plans.(*scheduledTestPlanRepository).finalizeQualityBatchRun(ctx, batch.ID, runs[1].Plan.ID))
			}
			if scenario == "late collected" {
				_, e = integrationDB.ExecContext(ctx, `UPDATE quality_probe_batch_runs SET snapshot=jsonb_set(snapshot,'{apply}','false'::jsonb) WHERE batch_id=$1 AND plan_id=$2`, batch.ID, runs[1].Plan.ID)
				require.NoError(t, e)
			}
			if scenario == "finished peer first" {
				require.NoError(t, plans.(*scheduledTestPlanRepository).finalizeQualityBatchRun(ctx, batch.ID, runs[1].Plan.ID))
			}
			require.NoError(t, repo.FinalizeQualityBatches(ctx))
			require.NoError(t, repo.FinalizeQualityBatches(ctx))
			history, e := resultsRepo.ListByPlanID(ctx, runs[0].Plan.ID, 10)
			require.NoError(t, e)
			require.Len(t, history, 1)
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT schedulable FROM accounts WHERE id=$1`, runs[0].Plan.AccountID).Scan(&schedulable))
			if scenario == "passing peer" || scenario == "finished peer first" {
				require.False(t, schedulable)
				require.Equal(t, "scheduling_disabled", history[0].QualityAction)
				require.Equal(t, "peer_passed", history[0].PelicanConfig.QualityBatch.Decision)
			} else {
				require.True(t, schedulable)
				require.Equal(t, "inconclusive", history[0].QualityAction)
				require.Equal(t, "no_passing_peer", history[0].PelicanConfig.QualityBatch.Decision)
			}
			var raw []byte
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT snapshot FROM quality_probe_batch_runs WHERE batch_id=$1 AND plan_id=$2`, batch.ID, runs[0].Plan.ID).Scan(&raw))
			var stored service.QualityBatchRun
			require.NoError(t, json.Unmarshal(raw, &stored))
			require.Empty(t, stored.Results[0].ResponseText)
		})
	}
}

func TestQualityOAuth401StreakDeletesOnlyUnownedRule(t *testing.T) {
	ctx := context.Background()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "quality-401-streak", Platform: "openai", Type: service.AccountTypeOAuth, Credentials: map[string]any{"access_token": "fixture"}})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE account_id=$1`, account.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
		require.NoError(t, err)
	})
	_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET status='error',schedulable=false,error_message='upstream 401' WHERE id=$1`, account.ID)
	require.NoError(t, err)
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	svc := service.NewScheduledTestService(plans, results)
	plan, err := plans.Create(ctx, &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "model", Enabled: true, CronExpression: "*/30 * * * *", MaxResults: 20,
		PelicanConfig: &service.PelicanTestConfig{ModelIDs: []string{"model"}, ParallelCount: 1, Quality: &service.QualityPolicy{Action: "observe_only", OAuth401MaxRounds: 2, Judge: &service.QualityJudgeConfig{Kind: "expected"}}}})
	require.NoError(t, err)
	for round := 0; round < 2; round++ {
		if round > 0 {
			_, err = integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET next_run_at=clock_timestamp()-interval '1 second' WHERE id=$1`, plan.ID)
			require.NoError(t, err)
			plan, err = plans.GetByID(ctx, plan.ID)
			require.NoError(t, err)
		}
		until := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Microsecond)
		version, claimed, claimErr := svc.ClaimQualityRun(ctx, plan, until)
		require.NoError(t, claimErr)
		require.True(t, claimed)
		plan.UpdatedAt, plan.RunningUntil = version, &until
		sample := &service.ScheduledTestResult{Status: "failed", ErrorMessage: "API returned 401: fixture body", DirectOAuth401: true,
			QualityRoundID: fmt.Sprintf("round-%d", round), StartedAt: time.Now(), FinishedAt: time.Now(), PelicanConfig: &service.PelicanTestConfig{ModelID: "model", Quality: plan.PelicanConfig.Quality}}
		action, commitErr := svc.CommitQualityRound(ctx, plan, "inconclusive", []*service.ScheduledTestResult{sample})
		require.NoError(t, commitErr)
		if round == 0 {
			require.NotEqual(t, "oauth401_rule_deleted", action)
			next := time.Now().Add(time.Hour)
			require.NoError(t, plans.(*scheduledTestPlanRepository).FinishQualityPlan(ctx, plan, time.Now(), next))
		}
	}
	var accountCount, planCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM accounts WHERE id=$1`, account.ID).Scan(&accountCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM scheduled_test_plans WHERE id=$1`, plan.ID).Scan(&planCount))
	require.Equal(t, 1, accountCount)
	require.Zero(t, planCount, "complete direct OAuth 401 rounds delete only the unowned rule")
	var schedulable bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT schedulable FROM accounts WHERE id=$1`, account.ID).Scan(&schedulable))
	require.False(t, schedulable, "rule cleanup never changes account state")
}
