package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *scheduledTestPlanRepository) ClaimPelicanRun(ctx context.Context, plan *service.ScheduledTestPlan, now, until, next time.Time) (time.Time, bool, error) {
	var version time.Time
	err := r.db.QueryRowContext(ctx, `UPDATE scheduled_test_plans
 SET running_until=$4, next_run_at=$5, updated_at=clock_timestamp()
 WHERE id=$1 AND updated_at=$2 AND enabled AND next_run_at <= $3
 AND (running_until IS NULL OR running_until < $3)
 AND (pelican_config->'quality' IS NULL OR pelican_config->'quality'='null'::jsonb)
 AND (pelican_config->>'question_kind'='pelican' OR pelican_config->>'prompt_mode'='random')
 RETURNING updated_at`, plan.ID, plan.UpdatedAt, now, until, next).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	return version, err == nil, err
}

func (r *scheduledTestPlanRepository) CommitPelicanRound(ctx context.Context, plan *service.ScheduledTestPlan, results []*service.ScheduledTestResult, finished time.Time) (bool, error) {
	if len(results) == 0 || plan.RunningUntil == nil {
		return false, errors.New("SVG round requires results and a lease")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM scheduled_test_plans
 WHERE id=$1 AND updated_at=$2 AND running_until=$3 AND enabled
 AND running_until > clock_timestamp() FOR UPDATE`, plan.ID, plan.UpdatedAt, plan.RunningUntil).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, result := range results {
		if _, err := insertScheduledTestResult(ctx, tx, result); err != nil {
			return false, err
		}
	}
	if err := pruneScheduledTestResults(ctx, tx, plan.ID, plan.MaxResults); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET last_run_at=$2, running_until=NULL,
 updated_at=clock_timestamp() WHERE id=$1`, plan.ID, finished); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
