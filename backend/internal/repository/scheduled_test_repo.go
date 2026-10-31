package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// --- Plan Repository ---

type scheduledTestPlanRepository struct {
	db *sql.DB
}

func qualityJSONArg(value any) []byte {
	if value == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return b
}

func NewScheduledTestPlanRepository(db *sql.DB) service.ScheduledTestPlanRepository {
	return &scheduledTestPlanRepository{db: db}
}

func (r *scheduledTestPlanRepository) Create(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_plans (account_id, model_id, cron_expression, enabled, max_results, auto_recover, next_run_at, pelican_config, running_until, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, account_id, model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until
	`, plan.AccountID, plan.ModelID, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.NextRunAt, qualityJSONArg(plan.PelicanConfig), plan.RunningUntil)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) GetByID(ctx context.Context, id int64) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until
		FROM scheduled_test_plans WHERE id = $1
	`, id)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) ListByAccountID(ctx context.Context, accountID int64) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until
		FROM scheduled_test_plans WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListQualityPlans(ctx context.Context) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.account_id, p.model_id, p.cron_expression, p.enabled, p.max_results, p.auto_recover,
		       p.last_run_at, p.next_run_at, p.created_at, p.updated_at, p.pelican_config, p.running_until
		FROM scheduled_test_plans p
		JOIN accounts a ON a.id = p.account_id
		WHERE p.pelican_config->'quality' IS NOT NULL AND a.deleted_at IS NULL
		ORDER BY p.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) TriggerQuality(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans
		SET next_run_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND enabled
		  AND pelican_config->'quality' IS NOT NULL
		  AND (running_until IS NULL OR running_until < NOW())
	`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return infraerrors.Conflict("QUALITY_PLAN_NOT_RUNNABLE", "quality rule must be enabled and idle")
	}
	return nil
}

func (r *scheduledTestPlanRepository) ListDue(ctx context.Context, now time.Time) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until
		FROM scheduled_test_plans
		WHERE enabled = true AND next_run_at <= $1
		ORDER BY next_run_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) Update(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET model_id = $2, cron_expression = $3, enabled = $4, max_results = $5, auto_recover = $6, next_run_at = $7, pelican_config = $8, running_until = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING id, account_id, model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until
	`, plan.ID, plan.ModelID, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.NextRunAt, qualityJSONArg(plan.PelicanConfig), plan.RunningUntil)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) UpdateWithVersion(ctx context.Context, plan *service.ScheduledTestPlan, expected time.Time) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET model_id = $2, cron_expression = $3, enabled = $4, max_results = $5, auto_recover = $6, next_run_at = $7, pelican_config = $8, running_until = $9, updated_at = NOW()
		WHERE id = $1 AND updated_at = $10
			AND NOT EXISTS (SELECT 1 FROM account_quality_states WHERE plan_id = $1)
		RETURNING id, account_id, model_id, cron_expression, enabled, max_results, auto_recover, last_run_at, next_run_at, created_at, updated_at, pelican_config, running_until
	`, plan.ID, plan.ModelID, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.NextRunAt, qualityJSONArg(plan.PelicanConfig), plan.RunningUntil, expected)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id = $1 AND pelican_config->'quality' IS NULL AND NOT EXISTS (SELECT 1 FROM account_quality_states WHERE plan_id = $1)`, id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil || deleted > 0 {
		return err
	}
	var config []byte
	var ownedState bool
	err = r.db.QueryRowContext(ctx, `SELECT pelican_config, EXISTS (SELECT 1 FROM account_quality_states WHERE plan_id=$1) FROM scheduled_test_plans WHERE id=$1`, id).Scan(&config, &ownedState)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if ownedState {
		return infraerrors.Conflict("QUALITY_OWNERSHIP_REQUIRED", "plans with owned quality state must be reconciled through the ownership endpoint")
	}
	if len(config) > 0 {
		var parsed service.PelicanTestConfig
		if err := json.Unmarshal(config, &parsed); err != nil {
			return err
		}
		if parsed.Quality != nil {
			return infraerrors.Conflict("QUALITY_OWNERSHIP_REQUIRED", "quality rules must be deleted through the preview and ownership endpoint")
		}
	}
	return nil
}

func (r *scheduledTestPlanRepository) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1
	`, id, lastRunAt, nextRunAt)
	return err
}

// --- Result Repository ---

type scheduledTestResultRepository struct {
	db *sql.DB
}

func NewScheduledTestResultRepository(db *sql.DB) service.ScheduledTestResultRepository {
	return &scheduledTestResultRepository{db: db}
}

func (r *scheduledTestResultRepository) Create(ctx context.Context, result *service.ScheduledTestResult) (*service.ScheduledTestResult, error) {
	return insertScheduledTestResult(ctx, r.db, result)
}

type scheduledTestResultSQL interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertScheduledTestResult(ctx context.Context, db scheduledTestResultSQL, result *service.ScheduledTestResult) (*service.ScheduledTestResult, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_results (plan_id, status, response_text, error_message, latency_ms, started_at, finished_at, pelican_config, quality_round_id, quality_action, quality_judgment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		RETURNING id, plan_id, status, response_text, error_message, latency_ms, started_at, finished_at, created_at, pelican_config, quality_round_id, quality_action, quality_judgment
	`, result.PlanID, result.Status, result.ResponseText, result.ErrorMessage, result.LatencyMs, result.StartedAt, result.FinishedAt, qualityJSONArg(result.PelicanConfig), result.QualityRoundID, result.QualityAction, qualityJSONArg(result.QualityJudgment))

	out := &service.ScheduledTestResult{}
	var config, judgment []byte
	if err := row.Scan(
		&out.ID, &out.PlanID, &out.Status, &out.ResponseText, &out.ErrorMessage,
		&out.LatencyMs, &out.StartedAt, &out.FinishedAt, &out.CreatedAt, &config, &out.QualityRoundID, &out.QualityAction, &judgment,
	); err != nil {
		return nil, err
	}
	if len(config) > 0 {
		out.PelicanConfig = &service.PelicanTestConfig{}
		if err := json.Unmarshal(config, out.PelicanConfig); err != nil {
			return nil, err
		}
	}
	if len(judgment) > 0 {
		if err := json.Unmarshal(judgment, &out.QualityJudgment); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *scheduledTestResultRepository) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*service.ScheduledTestResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_id, status, response_text, error_message, latency_ms, started_at, finished_at, created_at, pelican_config, quality_round_id, quality_action, quality_judgment
		FROM scheduled_test_results
		WHERE plan_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, planID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*service.ScheduledTestResult
	for rows.Next() {
		r := &service.ScheduledTestResult{}
		var config, judgment []byte
		if err := rows.Scan(
			&r.ID, &r.PlanID, &r.Status, &r.ResponseText, &r.ErrorMessage,
			&r.LatencyMs, &r.StartedAt, &r.FinishedAt, &r.CreatedAt, &config, &r.QualityRoundID, &r.QualityAction, &judgment,
		); err != nil {
			return nil, err
		}
		if len(config) > 0 {
			r.PelicanConfig = &service.PelicanTestConfig{}
			if err := json.Unmarshal(config, r.PelicanConfig); err != nil {
				return nil, err
			}
		}
		if len(judgment) > 0 {
			if err := json.Unmarshal(judgment, &r.QualityJudgment); err != nil {
				return nil, err
			}
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ListQualityHistory returns one row per quality run, grouped across parallel
// probes and ordered by the newest result ID. This endpoint intentionally
// omits response_text and strips prompt/answer fields from stored snapshots.
func (r *scheduledTestResultRepository) ListQualityHistory(ctx context.Context, beforeID int64, limit int) ([]*service.QualityHistoryResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH rounds AS (
			SELECT r.id, r.plan_id, r.status, r.error_message, r.latency_ms,
			       min(r.started_at) OVER quality_round AS started_at,
			       max(r.finished_at) OVER quality_round AS finished_at,
			       r.created_at,
			       CASE WHEN r.pelican_config IS NULL THEN NULL ELSE
			         (r.pelican_config - 'prompt') || jsonb_build_object(
			           'quality', (COALESCE(r.pelican_config->'quality', '{}'::jsonb) - 'expected_answer') ||
			             jsonb_build_object('judge', COALESCE(r.pelican_config->'quality'->'judge', '{}'::jsonb) - 'prompt')
			         )
			       END AS pelican_config,
			       r.quality_round_id, r.quality_action, r.quality_judgment,
			       a.id AS account_id, a.name AS account_name,
			       row_number() OVER quality_round AS row_in_round,
			       count(*) FILTER (WHERE r.status='success') OVER quality_round AS passed_count,
			       count(*) FILTER (WHERE r.status='skipped') OVER quality_round AS skipped_count,
			       GREATEST(count(*) OVER quality_round, COALESCE((r.pelican_config->>'parallel_count')::int, 0)) AS total_count,
			       bool_or(r.error_message='answer_mismatch') OVER quality_round AS any_wrong,
			       array_agg(r.id) OVER quality_round AS result_ids
			FROM scheduled_test_results r
			JOIN scheduled_test_plans p ON p.id=r.plan_id
			JOIN accounts a ON a.id=p.account_id
			WHERE r.pelican_config->'quality' IS NOT NULL AND a.deleted_at IS NULL
			WINDOW quality_round AS (
				PARTITION BY r.plan_id, COALESCE(NULLIF(r.quality_round_id, ''), r.id::text)
				ORDER BY r.id DESC ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
			)
		)
		SELECT id, plan_id,
		       CASE WHEN passed_count = total_count AND skipped_count = 0 THEN 'success' ELSE 'failed' END,
		       CASE WHEN any_wrong THEN 'answer_mismatch' ELSE error_message END,
		       latency_ms, started_at, finished_at, created_at, pelican_config, quality_round_id, quality_action, quality_judgment,
		       account_id, account_name, passed_count, skipped_count, total_count, result_ids
		FROM rounds
		WHERE row_in_round=1 AND ($1::bigint=0 OR id<$1)
		ORDER BY id DESC LIMIT $2
	`, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*service.QualityHistoryResult, 0)
	for rows.Next() {
		item := &service.QualityHistoryResult{}
		var config, judgment []byte
		if err := rows.Scan(&item.ID, &item.PlanID, &item.Status, &item.ErrorMessage, &item.LatencyMs,
			&item.StartedAt, &item.FinishedAt, &item.CreatedAt, &config, &item.QualityRoundID, &item.QualityAction,
			&judgment, &item.AccountID, &item.AccountName, &item.PassedCount, &item.SkippedCount, &item.TotalCount,
			pq.Array(&item.ResultIDs)); err != nil {
			return nil, err
		}
		if len(config) > 0 {
			item.PelicanConfig = &service.PelicanTestConfig{}
			if err := json.Unmarshal(config, item.PelicanConfig); err != nil {
				return nil, err
			}
		}
		if len(judgment) > 0 {
			if err := json.Unmarshal(judgment, &item.QualityJudgment); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *scheduledTestResultRepository) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	return pruneScheduledTestResults(ctx, r.db, planID, keepCount)
}

func pruneScheduledTestResults(ctx context.Context, db scheduledTestResultSQL, planID int64, keepCount int) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM scheduled_test_results
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY plan_id ORDER BY created_at DESC) AS rn
				FROM scheduled_test_results
				WHERE plan_id = $1
			) ranked
			WHERE rn > $2
		)
	`, planID, keepCount)
	return err
}

// --- scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanPlan(row scannable) (*service.ScheduledTestPlan, error) {
	p := &service.ScheduledTestPlan{}
	var config []byte
	if err := row.Scan(
		&p.ID, &p.AccountID, &p.ModelID, &p.CronExpression, &p.Enabled, &p.MaxResults, &p.AutoRecover,
		&p.LastRunAt, &p.NextRunAt, &p.CreatedAt, &p.UpdatedAt, &config, &p.RunningUntil,
	); err != nil {
		return nil, err
	}
	if len(config) > 0 {
		p.PelicanConfig = &service.PelicanTestConfig{}
		if err := json.Unmarshal(config, p.PelicanConfig); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func scanPlans(rows *sql.Rows) ([]*service.ScheduledTestPlan, error) {
	var plans []*service.ScheduledTestPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
