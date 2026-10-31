//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCommitQualityRoundPostgreSQLAtomicityAndFencing(t *testing.T) {
	for _, scenario := range []string{"second insert fails", "complete round", "stale rule version", "paused rule"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "quality-round-transaction"})
			t.Cleanup(func() {
				_, err := integrationDB.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE account_id=$1`, account.ID)
				require.NoError(t, err)
				_, err = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
				require.NoError(t, err)
			})
			plans := NewScheduledTestPlanRepository(integrationDB)
			resultsRepo := NewScheduledTestResultRepository(integrationDB)
			config := &service.PelicanTestConfig{ModelIDs: []string{"first", "second"}, Quality: &service.QualityPolicy{Action: "disable_scheduling"}}
			plan, err := plans.Create(ctx, &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "first", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 1, PelicanConfig: config})
			require.NoError(t, err)
			svc := service.NewScheduledTestService(plans, resultsRepo)
			until := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
			version, claimed, err := svc.ClaimQualityRun(ctx, plan, until)
			require.NoError(t, err)
			require.True(t, claimed)
			plan.UpdatedAt = version
			plan.RunningUntil = &until
			samples := []*service.ScheduledTestResult{qualityRoundSample(plan, "first"), qualityRoundSample(plan, "second")}
			samples[1].Status = "failed"
			samples[1].ErrorMessage = "answer_mismatch"
			samples[1].QualityJudgment = map[string]any{"verdict": "incorrect"}
			var initialOutbox int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM scheduler_outbox WHERE account_id=$1`, account.ID).Scan(&initialOutbox))
			if scenario == "second insert fails" {
				// This fixture-only trigger fails after the account action, outbox and
				// first result insert have executed inside the same transaction.
				name := fmt.Sprintf("quality_round_fail_%d", plan.ID)
				ddl := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS TRIGGER LANGUAGE plpgsql AS $$
      BEGIN IF NEW.plan_id=%d AND NEW.pelican_config->>'model_id'='second'
        THEN RAISE EXCEPTION 'fixture second probe insert failed'; END IF; RETURN NEW; END; $$;
      CREATE TRIGGER %s BEFORE INSERT ON scheduled_test_results FOR EACH ROW EXECUTE FUNCTION %s()`, name, plan.ID, name, name)
				_, err := integrationDB.ExecContext(ctx, ddl)
				require.NoError(t, err)
				t.Cleanup(func() {
					_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON scheduled_test_results; DROP FUNCTION IF EXISTS %s()`, name, name))
					require.NoError(t, err)
				})
			} else if scenario == "stale rule version" {
				_, err := integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET updated_at=clock_timestamp() WHERE id=$1`, plan.ID)
				require.NoError(t, err)
			} else if scenario == "paused rule" {
				_, err := integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false WHERE id=$1`, plan.ID)
				require.NoError(t, err)
			}
			action, err := svc.CommitQualityRound(ctx, plan, "failed", samples)
			if scenario == "second insert fails" {
				require.ErrorContains(t, err, "fixture second probe insert failed")
				require.Empty(t, action)
			} else {
				require.NoError(t, err)
			}
			var schedulable bool
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT schedulable FROM accounts WHERE id=$1`, account.ID).Scan(&schedulable))
			var states, outbox int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM account_quality_states WHERE plan_id=$1`, plan.ID).Scan(&states))
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM scheduler_outbox WHERE account_id=$1`, account.ID).Scan(&outbox))
			stored, readErr := resultsRepo.ListByPlanID(ctx, plan.ID, 100)
			require.NoError(t, readErr)
			if scenario == "complete round" {
				require.Equal(t, "scheduling_disabled", action)
				require.False(t, schedulable)
				require.Equal(t, 1, states)
				require.Equal(t, initialOutbox+1, outbox, "the whole round applies scheduling only once")
				require.Len(t, stored, 2, "row budget1 cannot split the newly committed round")
				models := map[string]bool{}
				for _, sample := range stored {
					models[sample.PelicanConfig.ModelID] = true
					require.Equal(t, "scheduling_disabled", sample.QualityAction)
					require.Equal(t, "same-round", sample.QualityRoundID)
				}
				require.Equal(t, map[string]bool{"first": true, "second": true}, models)
			} else {
				if scenario != "second insert fails" {
					require.Equal(t, "stale_run", action)
				}
				require.True(t, schedulable)
				require.Zero(t, states)
				require.Equal(t, initialOutbox, outbox)
				require.Empty(t, stored, "no earlier sample may survive a failed or stale round")
			}
		})
	}
}
