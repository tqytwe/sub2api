package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type qualityRuntimeState struct {
	Action              string    `json:"action"`
	PreviousSchedulable bool      `json:"previous_schedulable"`
	AppliedSchedulable  bool      `json:"applied_schedulable"`
	AccountUpdatedAt    time.Time `json:"account_updated_at"`
}

func (r *scheduledTestPlanRepository) ClaimQualityRun(ctx context.Context, plan *service.ScheduledTestPlan, until time.Time) (time.Time, bool, error) {
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET running_until=$3, updated_at=clock_timestamp()
		WHERE id=$1 AND enabled AND updated_at=$2
		  AND pelican_config->'quality' IS NOT NULL
		  AND (running_until IS NULL OR running_until < clock_timestamp())
		RETURNING updated_at
	`, plan.ID, plan.UpdatedAt, until).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return updatedAt, true, nil
}

func (r *scheduledTestPlanRepository) FinishQualityRun(ctx context.Context, id int64, until, lastRunAt, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans
		SET last_run_at=$3, next_run_at=$4, running_until=NULL, updated_at=clock_timestamp()
		WHERE id=$1 AND running_until=$2
	`, id, until, lastRunAt, nextRunAt)
	return err
}

func (r *scheduledTestPlanRepository) CommitQualityRun(ctx context.Context, plan *service.ScheduledTestPlan, outcome string, result *service.ScheduledTestResult, maxResults int) (string, error) {
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
	if outcome == "inconclusive" {
		return commitQualityResult(ctx, tx, result, maxResults, "inconclusive")
	}
	if plan.PelicanConfig.Quality.Action == "observe_only" {
		return commitQualityResult(ctx, tx, result, maxResults, "observed")
	}
	if state.Action != "" && state.Action != "disable_scheduling" {
		return commitQualityResult(ctx, tx, result, maxResults, "action_conflict")
	}
	if outcome == "failed" {
		if state.Action == "disable_scheduling" {
			return commitQualityResult(ctx, tx, result, maxResults, "already_disabled")
		}
		if status != service.StatusActive || !schedulable {
			return commitQualityResult(ctx, tx, result, maxResults, "account_not_schedulable")
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
		return commitQualityResult(ctx, tx, result, maxResults, "scheduling_disabled")
	}
	if outcome == "passed" && state.Action != "" && plan.PelicanConfig.Quality.AutoRestore {
		action, err := restoreQualityRuntimeState(ctx, tx, plan, state, schedulable, accountUpdatedAt)
		if err != nil {
			return "", err
		}
		return commitQualityResult(ctx, tx, result, maxResults, action)
	}
	return commitQualityResult(ctx, tx, result, maxResults, "no_change")
}

func commitQualityResult(ctx context.Context, tx *sql.Tx, result *service.ScheduledTestResult, maxResults int, action string) (string, error) {
	result.QualityAction = action
	if _, err := insertScheduledTestResult(ctx, tx, result); err != nil {
		return "", err
	}
	if err := pruneScheduledTestResults(ctx, tx, result.PlanID, maxResults); err != nil {
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
