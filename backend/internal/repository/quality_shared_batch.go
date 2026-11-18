package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/robfig/cron/v3"
)

var _ service.QualitySharedBatchRepository = (*scheduledTestPlanRepository)(nil)

// Captured only as a one-way equality marker, never returned by an API. Mapping,
// credentials and route edits invalidate both the target run and its peer proof.
const qualityBatchCredentialsRevision = `md5(jsonb_build_array(a.credentials,a.platform,a.type,a.proxy_id,
 a.extra->'openai_passthrough',a.extra->'openai_oauth_passthrough',
 a.extra->'openai_connections_managed',a.extra->'openai_auth_connections')::text)`

func (r *scheduledTestPlanRepository) StartQualityBatch(ctx context.Context, candidate *service.QualitySharedBatch, plan *service.ScheduledTestPlan, until time.Time) (*service.QualitySharedBatch, error) {
	questions, err := json.Marshal(candidate.Questions)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `INSERT INTO quality_probe_batches(id,started_at,start_deadline,closes_at,questions)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, candidate.ID, candidate.StartedAt, candidate.StartDeadline, candidate.ClosesAt, questions); err != nil {
		return nil, err
	}
	batch := &service.QualitySharedBatch{}
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT id,started_at,start_deadline,closes_at,questions FROM quality_probe_batches
 WHERE id=$1 AND clock_timestamp()>=started_at AND clock_timestamp()<start_deadline FOR UPDATE`, candidate.ID).
		Scan(&batch.ID, &batch.StartedAt, &batch.StartDeadline, &batch.ClosesAt, &raw)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &batch.Questions); err != nil {
		return nil, err
	}
	run := &service.QualityBatchRun{Plan: plan, Until: until, Trigger: plan.TriggerSource, ObservedAt: plan.TriggerObservedAt}
	err = tx.QueryRowContext(ctx, `SELECT `+qualityBatchCredentialsRevision+` FROM accounts a JOIN scheduled_test_plans p ON p.account_id=a.id
 WHERE p.id=$1 AND p.account_id=$2 AND p.enabled AND p.updated_at=$3 AND p.running_until=$4 AND a.deleted_at IS NULL`, plan.ID, plan.AccountID, plan.UpdatedAt, until).Scan(&run.CredentialsRevision)
	if err != nil {
		return nil, err
	}
	snapshot, err := json.Marshal(service.SanitizedQualityBatchRun(run))
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO quality_probe_batch_runs(batch_id,plan_id,account_id,lease_until,snapshot)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(batch_id,plan_id) DO NOTHING`, batch.ID, plan.ID, plan.AccountID, until, snapshot)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, fmt.Errorf("quality batch run unavailable")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return batch, nil
}

func (r *scheduledTestPlanRepository) CollectQualityBatch(ctx context.Context, run *service.QualityBatchRun) error {
	if len(run.Results) == 0 || run.Results[0].PelicanConfig == nil || run.Results[0].PelicanConfig.QualityBatch == nil {
		return fmt.Errorf("quality batch results unavailable")
	}
	batchID := run.Results[0].PelicanConfig.QualityBatch.ID
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Account -> plan -> run matches existing quality mutation lock order.
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, run.Plan.AccountID).Scan(&id); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM scheduled_test_plans WHERE id=$1 AND enabled AND updated_at=$2 AND running_until=$3 FOR UPDATE`, run.Plan.ID, run.Plan.UpdatedAt, run.Until).Scan(&id); err != nil {
		return err
	}
	var status string
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT status,snapshot FROM quality_probe_batch_runs WHERE batch_id=$1 AND plan_id=$2 AND lease_until=$3 FOR UPDATE`, batchID, run.Plan.ID, run.Until).Scan(&status, &raw); err != nil {
		return err
	}
	if status != "pending" {
		return nil
	}
	var original service.QualityBatchRun
	if err = json.Unmarshal(raw, &original); err != nil {
		return err
	}
	stored := *service.SanitizedQualityBatchRun(run)
	stored.CredentialsRevision = original.CredentialsRevision
	sanitizedResults := stored.Results
	stored.Results = nil
	for _, sample := range sanitizedResults {
		if sample.PelicanConfig == nil || sample.PelicanConfig.QualityBatch == nil || sample.PelicanConfig.QualityBatch.ID != batchID {
			return fmt.Errorf("quality batch sample mismatch")
		}
		sample.QualityAction = "comparison_pending"
		created, e := insertScheduledTestResult(ctx, tx, sample)
		if e != nil {
			return e
		}
		copy := *sample
		copy.ID = created.ID
		stored.Results = append(stored.Results, &copy)
	}
	var beforeClose bool
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()<=closes_at FROM quality_probe_batches WHERE id=$1`, batchID).Scan(&beforeClose); err != nil {
		return err
	}
	stored.Apply = stored.Apply && beforeClose
	snapshot, err := json.Marshal(&stored)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE quality_probe_batch_runs SET snapshot=$3,status='collected' WHERE batch_id=$1 AND plan_id=$2`, batchID, run.Plan.ID, snapshot); err != nil {
		return err
	}
	if err = pruneScheduledTestResults(ctx, tx, run.Plan.ID, run.Plan.MaxResults); err != nil {
		return err
	}
	return tx.Commit()
}

func readQualityBatchResults(ctx context.Context, tx *sql.Tx, batchID string, target int64) ([]*service.QualityBatchRun, error) {
	rows, err := tx.QueryContext(ctx, `SELECT r.snapshot, p.enabled AND p.updated_at=(r.snapshot->'plan'->>'updated_at')::timestamptz
	 AND a.deleted_at IS NULL AND `+qualityBatchCredentialsRevision+`=r.snapshot->>'credentials_revision', a.status='active'
 FROM quality_probe_batch_runs r JOIN scheduled_test_plans p ON p.id=r.plan_id JOIN accounts a ON a.id=p.account_id
 WHERE r.batch_id=$1 AND r.status IN ('collected','finalized')
 AND jsonb_typeof(r.snapshot->'results')='array' AND r.snapshot->'results'<>'[]'::jsonb ORDER BY r.plan_id`, batchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var runs []*service.QualityBatchRun
	for rows.Next() {
		var raw []byte
		var live, active bool
		var run service.QualityBatchRun
		if err = rows.Scan(&raw, &live, &active); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &run); err != nil {
			return nil, err
		}
		if run.Plan == nil || run.Plan.PelicanConfig == nil || run.Plan.PelicanConfig.Quality == nil || len(run.Results) == 0 || len(run.Results) > 100 {
			return nil, fmt.Errorf("quality batch snapshot incomplete")
		}
		for _, sample := range run.Results {
			if sample == nil || sample.PelicanConfig == nil || sample.PelicanConfig.QualityBatch == nil {
				return nil, fmt.Errorf("quality batch sample incomplete")
			}
		}
		run.Apply = run.Apply && live && (run.Plan.ID == target || active)
		run.Plan.TriggerSource, run.Plan.TriggerObservedAt = run.Trigger, run.ObservedAt
		if run.Plan.ID == target || run.Apply {
			runs = append(runs, &run)
		}
	}
	return runs, rows.Err()
}

func (r *scheduledTestPlanRepository) FinalizeQualityBatches(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `SELECT r.batch_id,r.plan_id FROM quality_probe_batch_runs r JOIN quality_probe_batches b ON b.id=r.batch_id
	 WHERE (r.status='collected' OR (r.status='pending' AND r.lease_until<=clock_timestamp())) AND b.start_deadline<=clock_timestamp()
	 AND (b.closes_at<=clock_timestamp() OR r.lease_until<=clock_timestamp() OR NOT EXISTS
	 (SELECT 1 FROM quality_probe_batch_runs p WHERE p.batch_id=b.id AND p.status='pending')) ORDER BY b.started_at,r.plan_id LIMIT 100`)
	if err != nil {
		return err
	}
	type target struct {
		batch string
		plan  int64
	}
	var targets []target
	for rows.Next() {
		var t target
		if err = rows.Scan(&t.batch, &t.plan); err != nil {
			_ = rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, t := range targets {
		if err = r.finalizeQualityBatchRun(ctx, t.batch, t.plan); err != nil {
			return err
		}
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM quality_probe_batches WHERE id IN
 (SELECT id FROM quality_probe_batches WHERE closes_at<clock_timestamp()-interval '7 days' ORDER BY closes_at LIMIT 1000)`)
	return err
}

func (r *scheduledTestPlanRepository) finalizeQualityBatchRun(ctx context.Context, batchID string, planID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var ready bool
	if err = tx.QueryRowContext(ctx, `SELECT start_deadline<=clock_timestamp() FROM quality_probe_batches WHERE id=$1 FOR UPDATE`, batchID).Scan(&ready); err != nil || !ready {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT b.closes_at<=clock_timestamp() OR EXISTS(SELECT 1 FROM quality_probe_batch_runs t WHERE t.batch_id=b.id AND t.plan_id=$2 AND t.lease_until<=clock_timestamp()) OR NOT EXISTS(SELECT 1 FROM quality_probe_batch_runs r
 WHERE r.batch_id=b.id AND r.status='pending') FROM quality_probe_batches b WHERE b.id=$1`, batchID, planID).Scan(&ready); err != nil || !ready {
		return err
	}
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT a.id FROM accounts a JOIN scheduled_test_plans p ON p.account_id=a.id WHERE p.id=$1 FOR UPDATE OF a`, planID).Scan(&id); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM scheduled_test_plans WHERE id=$1 FOR UPDATE`, planID).Scan(&id); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM quality_probe_batch_runs WHERE batch_id=$1 AND plan_id=$2 FOR UPDATE`, batchID, planID).Scan(&status); err == sql.ErrNoRows {
		return nil
	} else if err != nil || status != "collected" {
		if err != nil {
			return err
		}
		if status == "pending" {
			_, err = tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET running_until=NULL,last_run_at=clock_timestamp(),
			 next_run_at=GREATEST(next_run_at,clock_timestamp()+interval '1 minute'),updated_at=clock_timestamp()
			 WHERE id=$1 AND running_until=(SELECT lease_until FROM quality_probe_batch_runs WHERE batch_id=$2 AND plan_id=$1)`, planID, batchID)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE quality_oauth_401_streaks SET failures=0,own_revoked_401=false,last_round_until=claimed_until WHERE plan_id=$1 AND claimed_until=(SELECT lease_until FROM quality_probe_batch_runs WHERE batch_id=$2 AND plan_id=$1)`, planID, batchID)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE quality_probe_batch_runs SET status='finalized' WHERE batch_id=$1 AND plan_id=$2`, batchID, planID)
			if err != nil {
				return err
			}
			return tx.Commit()
		}
		return nil
	}
	peers, err := readQualityBatchResults(ctx, tx, batchID, planID)
	if err != nil {
		return err
	}
	var run *service.QualityBatchRun
	for _, p := range peers {
		if p.Plan.ID == planID {
			run = p
		}
	}
	if run == nil {
		return fmt.Errorf("quality batch run missing")
	}
	// Delayed 5xx evidence cannot restore scheduling after a newer real failure.
	if run.Trigger == "upstream_5xx" {
		run.Plan.PelicanConfig.Quality.AutoRestore = false
	}
	outcome := service.EvaluateQualityBatch(run, peers)
	action := "inconclusive"
	if run.Apply {
		action, err = commitQualityRoundTx(ctx, tx, run.Plan, outcome, run.Results, run.Plan.MaxResults)
		if err != nil {
			return err
		}
		if action == "stale_run" || action == "account_deleted" {
			if _, err = commitQualityResults(ctx, tx, run.Results, run.Plan.MaxResults, action); err != nil {
				return err
			}
		}
	} else {
		if _, err = commitQualityResults(ctx, tx, run.Results, run.Plan.MaxResults, action); err != nil {
			return err
		}
	}
	deleted, err := completeQualityOAuth401RoundTx(ctx, tx, run.Plan, run.Until, run.OAuth401)
	if err != nil {
		return err
	}
	if deleted {
		return tx.Commit()
	}
	finished := run.Results[0].FinishedAt
	for _, s := range run.Results {
		if s.FinishedAt.After(finished) {
			finished = s.FinishedAt
		}
	}
	schedule, err := cron.ParseStandard(run.Plan.CronExpression)
	if err != nil {
		return err
	}
	next := schedule.Next(time.Now())
	var version time.Time
	err = tx.QueryRowContext(ctx, `UPDATE scheduled_test_plans SET running_until=NULL,last_run_at=$4,next_run_at=GREATEST(next_run_at,$5),updated_at=clock_timestamp()
 WHERE id=$1 AND updated_at=$2 AND running_until=$3 RETURNING updated_at`, planID, run.Plan.UpdatedAt, run.Until, finished, next).Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		run.Plan.UpdatedAt = version
	}
	// Keep peer evidence after our finish updates the runtime version. Config or
	// credential changes still invalidate it on the next target finalization.
	snapshot, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE quality_probe_batch_runs SET status='finalized',snapshot=$3 WHERE batch_id=$1 AND plan_id=$2`, batchID, planID, snapshot)
	if err != nil {
		return err
	}
	return tx.Commit()
}
