package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) createWithInitialQuality(ctx context.Context, a *service.Account) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = createAccountRecord(ctx, tx.Client(), a); err != nil {
		return err
	}
	p := a.InitialQualityPlan
	_, err = tx.Client().ExecContext(ctx, `INSERT INTO scheduled_test_plans(account_id,model_id,cron_expression,enabled,max_results,auto_recover,next_run_at,pelican_config,created_at,updated_at) VALUES($1,$2,$3,$4,$5,false,$6,$7,NOW(),NOW())`, a.ID, p.ModelID, p.CronExpression, p.Enabled, p.MaxResults, p.NextRunAt, qualityJSONArg(p.PelicanConfig))
	if err != nil {
		return err
	}
	if err = enqueueSchedulerOutbox(ctx, tx.Client(), service.SchedulerOutboxEventAccountChanged, &a.ID, nil, nil); err != nil {
		return err
	}
	return tx.Commit()
}
