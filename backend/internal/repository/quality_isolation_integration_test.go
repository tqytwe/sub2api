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

func TestQualityIsolationPostgreSQLAppliesAndRecoversPerModel(t *testing.T) {
	ctx := context.Background()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "u03-quarantine", Platform: service.PlatformAnthropic})
	peer := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "u03-preserved", Platform: service.PlatformAnthropic})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "u03-model-isolation", Extra: map[string]any{"unrelated": "preserved"}})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id IN ($1,$2)`, group.ID, peer.ID)
	})
	_, err := integrationDB.Exec(`INSERT INTO account_groups(account_id,group_id,priority,allowed_models) VALUES($1,$2,3,'["first"]'),($1,$3,9,NULL)`, account.ID, group.ID, peer.ID)
	require.NoError(t, err)
	var config service.PelicanTestConfig
	raw, err := json.Marshal(map[string]any{
		"model_ids": []string{"first", "second"}, "prompt": "probe", "quality": map[string]any{
			"action": "isolate_models", "remove_group_ids": []int64{group.ID}, "auto_restore": true,
			"expected_answer": "ok", "judge": map[string]any{"kind": "expected"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &config))
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	plan, err := plans.Create(ctx, &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "first", PelicanConfig: &config, Enabled: true, CronExpression: "*/30 * * * *", MaxResults: 100})
	require.NoError(t, err)
	svc := service.NewScheduledTestService(plans, results)
	round := func(first, second string) string {
		t.Helper()
		require.NoError(t, plans.TriggerQuality(ctx, plan.ID))
		current, err := plans.GetByID(ctx, plan.ID)
		require.NoError(t, err)
		until := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
		version, claimed, err := svc.ClaimQualityRun(ctx, current, until)
		require.NoError(t, err)
		require.True(t, claimed)
		current.UpdatedAt, current.RunningUntil = version, &until
		samples := []*service.ScheduledTestResult{qualityRoundSample(current, "first"), qualityRoundSample(current, "second")}
		for i, verdict := range []string{first, second} {
			if verdict == "incorrect" {
				samples[i].Status, samples[i].ErrorMessage = "failed", "answer_mismatch"
			}
			samples[i].QualityJudgment = map[string]any{"verdict": verdict}
		}
		action, err := svc.CommitQualityRound(ctx, current, "failed", samples)
		require.NoError(t, err)
		require.NoError(t, svc.FinishQualityRun(ctx, plan.ID, until, time.Now(), time.Now().Add(time.Hour)))
		return action
	}
	read := func() (map[string]any, int) {
		t.Helper()
		var extra []byte
		var schedulable bool
		require.NoError(t, integrationDB.QueryRow(`SELECT extra,schedulable FROM accounts WHERE id=$1`, account.ID).Scan(&extra, &schedulable))
		require.True(t, schedulable, "isolation must not disable the account")
		var value map[string]any
		require.NoError(t, json.Unmarshal(extra, &value))
		require.Equal(t, "preserved", value["unrelated"])
		var memberships int
		require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM account_groups WHERE account_id=$1`, account.ID).Scan(&memberships))
		return value, memberships
	}
	require.Equal(t, "models_isolated", round("incorrect", "correct"))
	extra, memberships := read()
	require.Len(t, extra["quality_model_isolation"], 1)
	require.Equal(t, 2, memberships)
	require.Equal(t, "groups_removed", round("incorrect", "incorrect"))
	extra, memberships = read()
	require.Len(t, extra["quality_model_isolation"], 2)
	require.Equal(t, 1, memberships)
	require.Equal(t, "groups_restored_models_isolated", round("correct", "incorrect"))
	extra, memberships = read()
	require.Len(t, extra["quality_model_isolation"], 1)
	require.Contains(t, extra["quality_model_isolation"], "second")
	require.Equal(t, 2, memberships)
	require.Equal(t, "restored", round("correct", "correct"))
	extra, memberships = read()
	require.NotContains(t, extra, "quality_model_isolation")
	require.Equal(t, 2, memberships)
}

func TestQualityIsolationPostgreSQLAccountEditsPreserveServerState(t *testing.T) {
	for _, operation := range []string{"save", "patch", "bulk"} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/present=%t", operation, present), func(t *testing.T) {
				ctx := context.Background()
				tx := testEntTx(t)
				repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
				account := mustCreateAccount(t, tx.Client(), &service.Account{Name: "u03-edit-guard"})
				stale, err := repo.GetByID(ctx, account.ID)
				require.NoError(t, err)
				var marker any
				if present {
					marker = map[string]any{"model": map[string]any{"reason": "quality_rule:7", "isolated_at": "2026-11-07T00:00:00Z"}}
					_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_build_object('quality_model_isolation',$2::jsonb) WHERE id=$1`, account.ID, `{"model":{"reason":"quality_rule:7","isolated_at":"2026-11-07T00:00:00Z"}}`)
					require.NoError(t, err)
				}
				input := map[string]any{"unrelated": "updated", service.QualityModelIsolationKey: map[string]any{"stale": "must-not-be-written"}}
				switch operation {
				case "save":
					stale.Extra = input
					err = repo.Update(ctx, stale)
				case "patch":
					err = repo.UpdateExtra(ctx, account.ID, input)
				case "bulk":
					_, err = repo.BulkUpdate(ctx, []int64{account.ID}, service.AccountBulkUpdate{Extra: input})
				}
				require.NoError(t, err)
				current, err := repo.GetByID(ctx, account.ID)
				require.NoError(t, err)
				require.Equal(t, marker, current.Extra[service.QualityModelIsolationKey], "ordinary edits cannot clear, change or resurrect server-owned isolation")
				require.Equal(t, "updated", current.Extra["unrelated"])
			})
		}
	}
}
