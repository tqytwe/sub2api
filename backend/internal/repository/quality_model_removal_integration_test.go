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

type isolationFixture struct {
	repo *scheduledTestPlanRepository
	plan *service.ScheduledTestPlan
}

func newIsolationFixture(t *testing.T) isolationFixture {
	t.Helper()
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "u03-owned-models"})
	group := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "u03-owned-group", Platform: service.PlatformAnthropic})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE account_id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
		_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id=$1`, group.ID)
	})
	repo := &scheduledTestPlanRepository{db: integrationDB}
	cfg := &service.PelicanTestConfig{ModelIDs: []string{"sol", "astra", "missing"}, Prompt: "private-probe", Quality: &service.QualityPolicy{Action: service.QualityActionIsolateModels, RemoveGroupIDs: []int64{group.ID}, AutoRestore: true}}
	plan, err := repo.Create(context.Background(), &service.ScheduledTestPlan{AccountID: account.ID, ModelID: "sol", PelicanConfig: cfg, Enabled: true, CronExpression: "*/30 * * * *", MaxResults: 100})
	require.NoError(t, err)
	marker := func(stamp string) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"reason":"quality_rule:%d","isolated_at":"%s"}`, plan.ID, stamp))
	}
	expected := marker("2026-11-07T00:00:00Z")
	state := qualityRuntimeState{Action: service.QualityActionIsolateModels, IsolationModels: cfg.ModelIDs, IsolationGroupIDs: cfg.Quality.RemoveGroupIDs, FailureStreak: 4, PassStreak: 2, Quality5xxEpisode: 9, Models: &qualityIsolationModels{PersistentModels: true, ModelApplied: map[string]json.RawMessage{"astra": expected, "missing": expected}, ModelRateLimits: map[string]json.RawMessage{"astra": json.RawMessage("null"), "missing": json.RawMessage("null")}}}
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO account_quality_states(plan_id,state) VALUES($1,$2)`, plan.ID, encoded)
	require.NoError(t, err)
	extra, err := json.Marshal(map[string]any{"unrelated": "preserved", "quality_model_isolation": map[string]any{"sol": expected, "astra": marker("2026-11-07T00:01:00Z"), "unrelated": map[string]any{"reason": "manual"}}, "model_rate_limits": map[string]any{"sol": map[string]any{"reset_at": "2099-01-01T00:00:00Z"}}})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE accounts SET extra=$2 WHERE id=$1`, account.ID, extra)
	require.NoError(t, err)
	return isolationFixture{repo, plan}
}
func (f isolationFixture) aggregate(t *testing.T) string {
	t.Helper()
	var raw string
	require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_build_object('plan',to_jsonb(p),'state',(SELECT state FROM account_quality_states WHERE plan_id=p.id),'account',to_jsonb(a),'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY group_id) FROM account_groups g WHERE account_id=a.id),'outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM scheduler_outbox o WHERE account_id=a.id),'results',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM scheduled_test_results r WHERE plan_id=p.id))::text FROM scheduled_test_plans p JOIN accounts a ON a.id=p.account_id WHERE p.id=$1`, f.plan.ID).Scan(&raw))
	return raw
}
func TestQualityModelRemovalPostgreSQLOrphanContractsExactlyOneTarget(t *testing.T) {
	f := newIsolationFixture(t)
	ctx := context.Background()
	// Preserve a real removed-group snapshot, including original membership data.
	_, err := integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(state,'{groups}',jsonb_build_object('action','remove_groups','removed',jsonb_build_array(jsonb_build_object('account_id',$2::bigint,'group_id',$3::bigint,'priority',4,'created_at','2026-11-06T00:00:00Z','allowed_models',jsonb_build_array('sol','astra'))))) WHERE plan_id=$1`, f.plan.ID, f.plan.AccountID, f.plan.PelicanConfig.Quality.RemoveGroupIDs[0])
	require.NoError(t, err)
	view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	require.Len(t, view.ModelRemovals, 3)
	require.True(t, view.ModelRemovals[0].Ready)
	require.Equal(t, "remove_orphan_marker_only", view.ModelRemovals[0].Effect)
	require.False(t, view.ModelRemovals[1].Ready)
	require.Equal(t, "changed_same_owner", view.ModelRemovals[1].BlockedReason)
	before := f.aggregate(t)
	_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{RemoveModel: true, Model: "sol", ReasonCode: "orphan_marker_recovery", Confirm: true, Fence: view.Fence})
	require.NoError(t, err)
	after := f.aggregate(t)
	var b, a map[string]any
	require.NoError(t, json.Unmarshal([]byte(before), &b))
	require.NoError(t, json.Unmarshal([]byte(after), &a))
	plan := b["plan"].(map[string]any)
	actualPlan := a["plan"].(map[string]any)
	plan["model_id"] = "astra"
	plan["updated_at"] = actualPlan["updated_at"]
	plan["quality_rule_updated_at"] = actualPlan["quality_rule_updated_at"]
	plan["pelican_config"].(map[string]any)["model_ids"] = []any{"astra", "missing"}
	b["state"].(map[string]any)["isolation_models"] = []any{"astra", "missing"}
	account := b["account"].(map[string]any)
	account["updated_at"] = a["account"].(map[string]any)["updated_at"]
	delete(account["extra"].(map[string]any)["quality_model_isolation"].(map[string]any), "sol")
	b["outbox"] = a["outbox"]
	require.Equal(t, b, a, "all other scope, restrictions, group snapshots, counters and history must survive")
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM scheduler_outbox WHERE account_id=$1 AND event_type='account_changed'`, f.plan.AccountID).Scan(&count))
	require.Equal(t, 1, count)
	// A stale direct/template writer cannot reintroduce the removed target.
	_, err = f.repo.Update(ctx, f.plan)
	require.Error(t, err)
	current, err := f.repo.GetByID(ctx, f.plan.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"astra", "missing"}, current.PelicanConfig.ModelIDs)
	fresh, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	require.Len(t, fresh.ModelRemovals, 2)
}
func TestQualityModelRemovalPostgreSQLFencesAllLiveInputs(t *testing.T) {
	for _, change := range []string{"marker", "credentials", "status", "expiry", "lease", "config", "state", "groups"} {
		t.Run(change, func(t *testing.T) {
			f := newIsolationFixture(t)
			ctx := context.Background()
			view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
			require.NoError(t, err)
			switch change {
			case "marker":
				_, err = integrationDB.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{quality_model_isolation,sol,isolated_at}','"2026-11-07T00:02:00Z"') WHERE id=$1`, f.plan.AccountID)
			case "credentials":
				_, err = integrationDB.Exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}','{"sol":"other"}') WHERE id=$1`, f.plan.AccountID)
			case "status":
				_, err = integrationDB.Exec(`UPDATE accounts SET status='disabled' WHERE id=$1`, f.plan.AccountID)
			case "expiry":
				_, err = integrationDB.Exec(`UPDATE accounts SET expires_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, f.plan.AccountID)
			case "lease":
				_, err = integrationDB.Exec(`UPDATE scheduled_test_plans SET running_until=clock_timestamp()+interval '1 hour' WHERE id=$1`, f.plan.ID)
			case "config":
				_, err = integrationDB.Exec(`UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{prompt}','"changed"') WHERE id=$1`, f.plan.ID)
			case "state":
				_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(state,'{failure_streak}','8') WHERE plan_id=$1`, f.plan.ID)
			case "groups":
				_, err = integrationDB.Exec(`INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,8)`, f.plan.AccountID, f.plan.PelicanConfig.Quality.RemoveGroupIDs[0])
			}
			require.NoError(t, err)
			before := f.aggregate(t)
			_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{RemoveModel: true, Model: "sol", ReasonCode: "orphan_marker_recovery", Confirm: true, Fence: view.Fence})
			require.ErrorContains(t, err, "preview again")
			require.Equal(t, before, f.aggregate(t), "failed confirmation must be read-only")
		})
	}
}
func TestQualityModelRemovalPostgreSQLRejectsUnsafeMarkers(t *testing.T) {
	for _, scenario := range []string{"other_owner", "no_owner", "extra_field", "malformed", "previous", "shared_mapping", "direct_marker", "scope", "last_model", "running", "inactive", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			f := newIsolationFixture(t)
			ctx := context.Background()
			var err error
			switch scenario {
			case "other_owner":
				_, err = integrationDB.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{quality_model_isolation,sol,reason}','"quality_rule:99999"') WHERE id=$1`, f.plan.AccountID)
			case "no_owner":
				_, err = integrationDB.Exec(`UPDATE accounts SET extra=extra #- '{quality_model_isolation,sol,reason}' WHERE id=$1`, f.plan.AccountID)
			case "extra_field":
				_, err = integrationDB.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{quality_model_isolation,sol,reset_at}','"2099-01-01T00:00:00Z"') WHERE id=$1`, f.plan.AccountID)
			case "malformed":
				_, err = integrationDB.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{quality_model_isolation,sol}','7') WHERE id=$1`, f.plan.AccountID)
			case "previous":
				_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(state,'{models,model_rate_limits,sol}','{"reason":"manual"}') WHERE plan_id=$1`, f.plan.ID)
			case "shared_mapping":
				_, err = integrationDB.Exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}','{"astra":"sol"}') WHERE id=$1`, f.plan.AccountID)
			case "direct_marker":
				_, err = integrationDB.Exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{model_mapping}','{"sol":"mapped"}') WHERE id=$1`, f.plan.AccountID)
			case "scope":
				_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(state,'{isolation_models}','["different"]') WHERE plan_id=$1`, f.plan.ID)
			case "last_model":
				_, err = integrationDB.Exec(`UPDATE scheduled_test_plans SET pelican_config=jsonb_set(pelican_config,'{model_ids}','["sol"]') WHERE id=$1`, f.plan.ID)
				require.NoError(t, err)
				_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(state,'{isolation_models}','["sol"]') WHERE plan_id=$1`, f.plan.ID)
			case "running":
				_, err = integrationDB.Exec(`UPDATE scheduled_test_plans SET running_until=clock_timestamp()+interval '1 hour' WHERE id=$1`, f.plan.ID)
			case "inactive":
				_, err = integrationDB.Exec(`UPDATE accounts SET status='disabled' WHERE id=$1`, f.plan.AccountID)
			case "expired":
				_, err = integrationDB.Exec(`UPDATE accounts SET expires_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, f.plan.AccountID)
			}
			require.NoError(t, err)
			view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
			require.NoError(t, err)
			require.NotEmpty(t, view.ModelRemovals)
			require.False(t, view.ModelRemovals[0].Ready, scenario)
			before := f.aggregate(t)
			_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{RemoveModel: true, Model: "sol", ReasonCode: "retire_model", Confirm: true, Fence: view.Fence})
			require.Error(t, err)
			require.Equal(t, before, f.aggregate(t))
		})
	}
}
func TestQualityIsolationPostgreSQLManualRestoreAndReconcile(t *testing.T) {
	for _, op := range []string{"restore", "reconcile"} {
		t.Run(op, func(t *testing.T) {
			f := newIsolationFixture(t)
			ctx := context.Background()
			// Normalize to an exact owned marker; omit the orphan and unrelated marker.
			_, err := integrationDB.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{quality_model_isolation}',(SELECT state->'models'->'model_applied' FROM account_quality_states WHERE plan_id=$2)-'missing') WHERE id=$1`, f.plan.AccountID, f.plan.ID)
			require.NoError(t, err)
			if op == "restore" {
				// Missing markers are reconciled separately; restore must be exact throughout.
				_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=state #- '{models,model_applied,missing}' #- '{models,model_rate_limits,missing}' WHERE plan_id=$1`, f.plan.ID)
				require.NoError(t, err)
			} else {
				_, err = integrationDB.Exec(`UPDATE accounts SET extra=extra-'quality_model_isolation' WHERE id=$1`, f.plan.AccountID)
				require.NoError(t, err)
			}
			view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
			require.NoError(t, err)
			if op == "restore" {
				require.True(t, view.RestoreReady)
			} else {
				require.True(t, view.ReconcileReady)
			}
			_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{Restore: op == "restore", Reconcile: op == "reconcile", Confirm: true, Fence: view.Fence})
			require.NoError(t, err)
			var states int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM account_quality_states WHERE plan_id=$1`, f.plan.ID).Scan(&states))
			require.Zero(t, states)
			current, err := f.repo.GetByID(ctx, f.plan.ID)
			require.NoError(t, err)
			require.False(t, current.Enabled, "manual lifecycle action must fence and pause new workers")
		})
	}
}

func TestQualityIsolationPostgreSQLConcurrentTemplateWriteCannotBypassOwnership(t *testing.T) {
	f := newIsolationFixture(t)
	ctx := context.Background()
	_, err := integrationDB.Exec(`DELETE FROM account_quality_states WHERE plan_id=$1`, f.plan.ID)
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	var pid int
	require.NoError(t, tx.QueryRow(`SELECT pg_backend_pid()`).Scan(&pid))
	var accountID int64
	require.NoError(t, tx.QueryRow(`SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, f.plan.AccountID).Scan(&accountID))
	var planID int64
	require.NoError(t, tx.QueryRow(`SELECT id FROM scheduled_test_plans WHERE id=$1 FOR UPDATE`, f.plan.ID).Scan(&planID))
	done := make(chan error, 1)
	go func() { _, err := f.repo.Update(ctx, f.plan); done <- err }()
	require.Eventually(t, func() bool {
		var blocked bool
		err := integrationDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 10*time.Millisecond)
	_, err = tx.Exec(`INSERT INTO account_quality_states(plan_id,state) VALUES($1,'{"action":"isolate_models"}')`, f.plan.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	select {
	case err := <-done:
		require.Error(t, err, "a writer that waited for the runner must re-read ownership after locking")
	case <-time.After(5 * time.Second):
		t.Fatal("plan update did not terminate")
	}
}

func TestQualityModelRemovalPostgreSQLRollsBackOnOutboxFailure(t *testing.T) {
	f := newIsolationFixture(t)
	ctx := context.Background()
	name := fmt.Sprintf("quality_remove_fail_%d", f.plan.ID)
	_, err := integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.account_id=%d THEN RAISE EXCEPTION 'fixture outbox failed'; END IF; RETURN NEW; END; $$; CREATE TRIGGER %s BEFORE INSERT ON scheduler_outbox FOR EACH ROW EXECUTE FUNCTION %s()`, name, f.plan.AccountID, name, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON scheduler_outbox; DROP FUNCTION IF EXISTS %s()`, name, name))
		require.NoError(t, err)
	})
	view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	before := f.aggregate(t)
	_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{RemoveModel: true, Model: "sol", ReasonCode: "orphan_marker_recovery", Confirm: true, Fence: view.Fence})
	require.ErrorContains(t, err, "fixture outbox failed")
	require.Equal(t, before, f.aggregate(t))
}

func TestQualityIsolationPostgreSQLUsesPlatformDefaultMapping(t *testing.T) {
	f := newIsolationFixture(t)
	ctx := context.Background()
	_, err := integrationDB.Exec(`UPDATE accounts SET platform='antigravity',extra=jsonb_build_object('quality_model_isolation',jsonb_build_object('gemini-3-pro-high',extra->'quality_model_isolation'->'sol')) WHERE id=$1`, f.plan.AccountID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE scheduled_test_plans SET model_id='gemini-3-pro-preview',pelican_config=jsonb_set(pelican_config,'{model_ids}','["gemini-3-pro-preview","astra","missing"]') WHERE id=$1`, f.plan.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(jsonb_set(jsonb_set(state,'{isolation_models}','["gemini-3-pro-preview","astra","missing"]'),'{models,model_applied}','{}'),'{models,model_rate_limits}','{}') WHERE plan_id=$1`, f.plan.ID)
	require.NoError(t, err)
	view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	require.Equal(t, "gemini-3-pro-high", view.ModelRemovals[0].IsolationModel, "preview and scheduler must share platform default aliases")
	require.Equal(t, "remove_orphan_marker_only", view.ModelRemovals[0].Effect)
	_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{RemoveModel: true, Model: "gemini-3-pro-preview", ReasonCode: "orphan_marker_recovery", Confirm: true, Fence: view.Fence})
	require.NoError(t, err)
	var remaining bool
	require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(extra->'quality_model_isolation' ? 'gemini-3-pro-high',false) FROM accounts WHERE id=$1`, f.plan.AccountID).Scan(&remaining))
	require.False(t, remaining)
}

func TestQualityIsolationPostgreSQLOwnedRuleCanPauseWithoutDroppingScope(t *testing.T) {
	f := newIsolationFixture(t)
	ctx := context.Background()
	current, err := f.repo.GetByID(ctx, f.plan.ID)
	require.NoError(t, err)
	current.Enabled = false
	current.CronExpression = "0 * * * *"
	before := f.aggregate(t)
	saved, err := f.repo.UpdateWithVersion(ctx, current, current.UpdatedAt)
	require.NoError(t, err)
	require.False(t, saved.Enabled)
	require.Equal(t, "0 * * * *", saved.CronExpression)
	var b, a map[string]any
	require.NoError(t, json.Unmarshal([]byte(before), &b))
	require.NoError(t, json.Unmarshal([]byte(f.aggregate(t)), &a))
	b["plan"] = a["plan"]
	require.Equal(t, b, a, "pausing may not alter owned restrictions, snapshots, counters, groups or outbox")
	saved.PelicanConfig.ModelIDs = []string{"astra", "missing"}
	saved.ModelID = "astra"
	stable := f.aggregate(t)
	_, err = f.repo.Update(ctx, saved)
	require.Error(t, err, "scope contraction requires explicit per-model consent")
	require.Equal(t, stable, f.aggregate(t))
}

func TestQualityIsolationPostgreSQLDeletedGroupBlocksRestoreAndFencesPreview(t *testing.T) {
	f := newIsolationFixture(t)
	ctx := context.Background()
	groupID := f.plan.PelicanConfig.Quality.RemoveGroupIDs[0]
	_, err := integrationDB.Exec(`UPDATE accounts SET extra=extra-'quality_model_isolation' WHERE id=$1`, f.plan.AccountID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE account_quality_states SET state=jsonb_set(jsonb_set(jsonb_set(state,'{models,model_applied}','{}'),'{models,model_rate_limits}','{}'),'{groups}',jsonb_build_object('action','remove_groups','removed',jsonb_build_array(jsonb_build_object('account_id',$2::bigint,'group_id',$3::bigint,'priority',4,'created_at','2026-11-06T00:00:00Z','allowed_models',jsonb_build_array('sol','astra'))))) WHERE plan_id=$1`, f.plan.ID, f.plan.AccountID, groupID)
	require.NoError(t, err)
	view, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	require.True(t, view.RestoreReady)
	_, err = integrationDB.Exec(`UPDATE groups SET deleted_at=clock_timestamp() WHERE id=$1`, groupID)
	require.NoError(t, err)
	before := f.aggregate(t)
	_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{Restore: true, Confirm: true, Fence: view.Fence})
	require.ErrorContains(t, err, "preview again")
	blocked, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	require.False(t, blocked.RestoreReady)
	require.False(t, blocked.ReconcileReady)
	require.Equal(t, "group_conflict", blocked.RestoreBlockedReason)
	require.Equal(t, before, f.aggregate(t))
	_, err = integrationDB.Exec(`UPDATE groups SET deleted_at=NULL WHERE id=$1`, groupID)
	require.NoError(t, err)
	fresh, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil)
	require.NoError(t, err)
	require.True(t, fresh.RestoreReady)
	_, err = f.repo.QualityOwnership(ctx, f.plan.ID, &service.QualityOwnershipRequest{Restore: true, Confirm: true, Fence: fresh.Fence})
	require.NoError(t, err)
	var priority int
	var allowed string
	require.NoError(t, integrationDB.QueryRow(`SELECT priority,allowed_models::text FROM account_groups WHERE account_id=$1 AND group_id=$2`, f.plan.AccountID, groupID).Scan(&priority, &allowed))
	require.Equal(t, 4, priority)
	require.JSONEq(t, `["sol","astra"]`, allowed)
}

func TestQualityIsolationPostgreSQLGroupDeletionDoesNotDeadlockPreview(t *testing.T) {
	f := newIsolationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	groupID := f.plan.PelicanConfig.Quality.RemoveGroupIDs[0]
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,1)`, f.plan.AccountID, groupID)
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	var locked int64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE id=$1 FOR UPDATE`, groupID).Scan(&locked))
	var pid int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	go func() { _, err := f.repo.QualityOwnership(ctx, f.plan.ID, nil); done <- err }()
	require.Eventually(t, func() bool {
		var blocked bool
		err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked)
		return err == nil && blocked
	}, 3*time.Second, 10*time.Millisecond)
	// Match DeleteCascade: group row first, then memberships. Preview must not
	// hold memberships while waiting for the group row held by this transaction.
	_, err = tx.ExecContext(ctx, `DELETE FROM account_groups WHERE group_id=$1`, groupID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
}
