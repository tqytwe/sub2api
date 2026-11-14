package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type qualityRuntimeState struct {
	RetestAfter         *time.Time              `json:"retest_after,omitempty"`
	Action              string                  `json:"action"`
	PreviousSchedulable bool                    `json:"previous_schedulable"`
	AppliedSchedulable  bool                    `json:"applied_schedulable"`
	AccountUpdatedAt    time.Time               `json:"account_updated_at"`
	Models              *qualityIsolationModels `json:"models,omitempty"`
	Groups              *qualityIsolationGroups `json:"groups,omitempty"`
	IsolationModels     []string                `json:"isolation_models,omitempty"`
	IsolationGroupIDs   []int64                 `json:"isolation_group_ids,omitempty"`
	FailureStreak       int                     `json:"failure_streak,omitempty"`
	PassStreak          int                     `json:"pass_streak,omitempty"`
	Quality5xxEpisode   int64                   `json:"quality_5xx_episode,omitempty"`
}

func (r *scheduledTestPlanRepository) ClaimQualityRun(ctx context.Context, plan *service.ScheduledTestPlan, until time.Time) (time.Time, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return time.Time{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('quality-account:' || $1::text,0))`, plan.AccountID).Scan(&locked); err != nil {
		return time.Time{}, false, err
	}
	if !locked {
		return time.Time{}, false, nil
	}
	var updatedAt time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET running_until=$3, updated_at=clock_timestamp(),pelican_config=pelican_config-'_manual_quality_run'
		WHERE id=$1 AND enabled AND updated_at=$2
		  AND pelican_config->'quality' IS NOT NULL
		  AND EXISTS(SELECT 1 FROM accounts live WHERE live.id=scheduled_test_plans.account_id AND live.deleted_at IS NULL)
		  AND (running_until IS NULL OR running_until < clock_timestamp())
		  AND (next_run_at IS NULL OR next_run_at<=clock_timestamp() OR
		    ($4 AND pelican_config->'quality'->>'trigger_on_upstream_5xx'='true'))
		  AND (NOT $4 OR (pelican_config->'quality'->>'trigger_on_upstream_5xx'='true'
		    AND (last_run_at IS NULL OR last_run_at<$5)
		    AND EXISTS(SELECT 1 FROM accounts a WHERE a.id=scheduled_test_plans.account_id AND a.type='oauth' AND a.deleted_at IS NULL)))
		  AND NOT EXISTS(SELECT 1 FROM account_quality_states rest WHERE rest.plan_id=scheduled_test_plans.id
		    AND COALESCE((pelican_config->'quality'->>'retest_rest_minutes')::int,0)>0
		    AND (rest.state->>'retest_after')::timestamptz>clock_timestamp())
		  AND NOT EXISTS(SELECT 1 FROM scheduled_test_plans other WHERE other.account_id=scheduled_test_plans.account_id
		    AND other.id<>scheduled_test_plans.id AND other.pelican_config->'quality' IS NOT NULL AND other.running_until>clock_timestamp())
		RETURNING updated_at
	`, plan.ID, plan.UpdatedAt, until, plan.TriggerSource == "upstream_5xx", plan.TriggerObservedAt).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return updatedAt, true, tx.Commit()
}

func (r *scheduledTestPlanRepository) FinishQualityRun(ctx context.Context, id int64, until, lastRunAt, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans
		SET last_run_at=$3, next_run_at=GREATEST(next_run_at,$4), running_until=NULL, updated_at=clock_timestamp()
		WHERE id=$1 AND running_until=$2
	`, id, until, lastRunAt, nextRunAt)
	return err
}

func (r *scheduledTestPlanRepository) FinishQualityPlan(ctx context.Context, plan *service.ScheduledTestPlan, started, next time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE scheduled_test_plans SET last_run_at=$4,
 next_run_at=GREATEST(next_run_at,$5),running_until=NULL,updated_at=clock_timestamp()
 WHERE id=$1 AND updated_at=$2 AND running_until=$3 AND running_until>clock_timestamp() AND enabled`, plan.ID, plan.UpdatedAt, plan.RunningUntil, started, next)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (r *scheduledTestPlanRepository) CommitQualityRun(ctx context.Context, plan *service.ScheduledTestPlan, outcome string, result *service.ScheduledTestResult, maxResults int) (string, error) {
	return r.CommitQualityRound(ctx, plan, outcome, []*service.ScheduledTestResult{result}, maxResults)
}

func (r *scheduledTestPlanRepository) CommitQualityRound(ctx context.Context, plan *service.ScheduledTestPlan, outcome string, results []*service.ScheduledTestResult, maxResults int) (string, error) {
	if len(results) == 0 {
		return "", errors.New("quality round requires results")
	}
	if plan == nil || plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil || plan.RunningUntil == nil {
		return "stale_run", nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	var accountID int64
	err = tx.QueryRowContext(ctx, `SELECT account_id FROM scheduled_test_plans WHERE id=$1`, plan.ID).Scan(&accountID)
	if err == sql.ErrNoRows {
		return "stale_run", nil
	}
	if err != nil {
		return "", err
	}
	if accountID != plan.AccountID {
		return "stale_run", nil
	}
	var schedulable bool
	var accountUpdatedAt time.Time
	var status string
	err = tx.QueryRowContext(ctx, `SELECT id, schedulable, updated_at, status FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, accountID).
		Scan(&accountID, &schedulable, &accountUpdatedAt, &status)
	if err == sql.ErrNoRows {
		return "account_deleted", nil
	}
	if err != nil {
		return "", err
	}
	var lockedAccountID int64
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT account_id, enabled AND updated_at=$2 AND running_until=$3 AND running_until>clock_timestamp()
		FROM scheduled_test_plans WHERE id=$1 FOR UPDATE`, plan.ID, plan.UpdatedAt, *plan.RunningUntil).Scan(&lockedAccountID, &valid)
	if err == sql.ErrNoRows {
		return "stale_run", nil
	}
	if err != nil {
		return "", err
	}
	if lockedAccountID != accountID || !valid {
		return "stale_run", nil
	}
	var stateBytes []byte
	err = tx.QueryRowContext(ctx, `SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`, plan.ID).Scan(&stateBytes)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	var state qualityRuntimeState
	if len(stateBytes) != 0 {
		if err := json.Unmarshal(stateBytes, &state); err != nil {
			return "", err
		}
	}
	commit := func(action string) (string, error) {
		ownedDisable := action == "already_disabled" && schedulable == state.AppliedSchedulable && accountUpdatedAt.Equal(state.AccountUpdatedAt)
		if action == "scheduling_disabled" || ownedDisable || action == "models_isolated" || action == "groups_removed" || action == "groups_restored_models_isolated" {
			if err := applyQualityRetestRest(ctx, tx, plan, outcome, results); err != nil {
				return "", err
			}
		}
		return commitQualityResults(ctx, tx, results, maxResults, action)
	}
	if plan.PelicanConfig.Quality.Action == service.QualityActionIsolateModels {
		action, err := applyQualityIsolationRound(ctx, tx, plan, &state, stateBytes, results, status)
		if err != nil {
			return "", err
		}
		return commit(action)
	}
	if outcome == "inconclusive" {
		return commit("inconclusive")
	}
	if plan.PelicanConfig.Quality.Action == "observe_only" {
		return commit("observed")
	}
	if state.Action != "" && state.Action != "disable_scheduling" {
		return commit("action_conflict")
	}
	if outcome == "failed" {
		if state.Action == "disable_scheduling" {
			return commit("already_disabled")
		}
		if status != service.StatusActive || !schedulable {
			return commit("account_not_schedulable")
		}
		if err := tx.QueryRowContext(ctx, `UPDATE accounts SET schedulable=false, updated_at=clock_timestamp() WHERE id=$1 RETURNING updated_at`, plan.AccountID).Scan(&accountUpdatedAt); err != nil {
			return "", err
		}
		accountID := plan.AccountID
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			return "", err
		}
		state = qualityRuntimeState{Action: "disable_scheduling", PreviousSchedulable: true, AppliedSchedulable: false, AccountUpdatedAt: accountUpdatedAt}
		encoded, err := json.Marshal(state)
		if err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_quality_states(plan_id,state,updated_at) VALUES($1,$2,NOW())
			ON CONFLICT(plan_id) DO UPDATE SET state=EXCLUDED.state, updated_at=EXCLUDED.updated_at`, plan.ID, encoded); err != nil {
			return "", err
		}
		return commit("scheduling_disabled")
	}
	if outcome == "passed" && state.Action != "" && plan.PelicanConfig.Quality.AutoRestore {
		action, err := restoreQualityRuntimeState(ctx, tx, plan, state, schedulable, accountUpdatedAt)
		if err != nil {
			return "", err
		}
		return commit(action)
	}
	return commit("no_change")
}

func applyQualityRetestRest(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, outcome string, results []*service.ScheduledTestResult) error {
	policy := plan.PelicanConfig.Quality
	failed := outcome == "failed"
	if policy.Action == service.QualityActionIsolateModels {
		failed = false
		for _, verdict := range qualityIsolationOutcomes(plan, results) {
			failed = failed || verdict == "failed"
		}
	}
	if !failed || policy.RetestRestMinutes <= 0 || policy.Action == "observe_only" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `WITH resting AS (
 UPDATE account_quality_states SET state=jsonb_set(state,'{retest_after}',to_jsonb(clock_timestamp()+make_interval(mins=>$2::int)))
 WHERE plan_id=$1 AND state->>'action' IN ('disable_scheduling','isolate_models')
 RETURNING (state->>'retest_after')::timestamptz AS deadline)
 UPDATE scheduled_test_plans SET next_run_at=GREATEST(next_run_at,resting.deadline) FROM resting WHERE id=$1`, plan.ID, policy.RetestRestMinutes)
	return err
}

func commitQualityResults(ctx context.Context, tx *sql.Tx, results []*service.ScheduledTestResult, maxResults int, action string) (string, error) {
	for _, result := range results {
		result.QualityAction = action
		if _, err := insertScheduledTestResult(ctx, tx, result); err != nil {
			return "", err
		}
	}
	if err := pruneScheduledTestResults(ctx, tx, results[0].PlanID, maxResults); err != nil {
		return "", err
	}
	return action, tx.Commit()
}

func restoreQualityRuntimeState(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, state qualityRuntimeState, schedulable bool, updatedAt time.Time) (string, error) {
	if state.Action != "disable_scheduling" || schedulable != state.AppliedSchedulable || !updatedAt.Equal(state.AccountUpdatedAt) {
		return "restore_conflict", nil
	}
	var restoredAt time.Time
	if err := tx.QueryRowContext(ctx, `UPDATE accounts SET schedulable=$2, updated_at=clock_timestamp() WHERE id=$1 RETURNING updated_at`, plan.AccountID, state.PreviousSchedulable).Scan(&restoredAt); err != nil {
		return "", err
	}
	accountID := plan.AccountID
	if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_quality_states WHERE plan_id=$1`, plan.ID); err != nil {
		return "", err
	}
	return "restored", nil
}
