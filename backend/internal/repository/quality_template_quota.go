package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *qualityRuleTemplateRepository) ForPlan(ctx context.Context, planID int64) (*service.QualityRuleTemplate, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT t.account_filter FROM quality_rule_templates t
 JOIN quality_rule_template_accounts link ON link.template_id=t.id WHERE link.plan_id=$1`, planID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	template := &service.QualityRuleTemplate{}
	if err := json.Unmarshal(raw, &template.AccountFilter); err != nil {
		return nil, err
	}
	return template, nil
}

func (r *scheduledTestPlanRepository) FinishQuotaSkippedQuality(ctx context.Context, plan *service.ScheduledTestPlan, next time.Time) error {
	// No model request or verdict: retain last_run_at and all ownership evidence.
	result, err := r.db.ExecContext(ctx, `UPDATE scheduled_test_plans SET running_until=NULL,
 next_run_at=GREATEST(next_run_at,$4),updated_at=clock_timestamp()
 WHERE id=$1 AND updated_at=$2 AND running_until=$3`, plan.ID, plan.UpdatedAt, plan.RunningUntil, next)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return sql.ErrNoRows
	}
	return err
}
