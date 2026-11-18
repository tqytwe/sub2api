package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var qualityAccount401Pattern = regexp.MustCompile(`(^|[^0-9])401([^0-9]|$)`)

func claimQualityOAuth401Tx(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, version, until time.Time) error {
	if plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil || plan.PelicanConfig.Quality.OAuth401MaxRounds == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO quality_oauth_401_streaks
 (plan_id,config_updated_at,config_revision,credentials_revision,claimed_until)
 SELECT $1,$2,md5(jsonb_build_array(p.pelican_config,p.model_id,p.cron_expression)::text),
 md5(COALESCE(a.credentials,'{}'::jsonb)::text),$3 FROM accounts a JOIN scheduled_test_plans p ON p.id=$1
 WHERE a.id=$4 AND a.type='oauth' AND a.deleted_at IS NULL
 ON CONFLICT(plan_id) DO UPDATE SET
 failures=CASE WHEN quality_oauth_401_streaks.config_revision=EXCLUDED.config_revision
 AND quality_oauth_401_streaks.credentials_revision=EXCLUDED.credentials_revision
 AND quality_oauth_401_streaks.last_round_until=quality_oauth_401_streaks.claimed_until
 THEN quality_oauth_401_streaks.failures ELSE 0 END,
 own_revoked_401=CASE WHEN quality_oauth_401_streaks.config_revision=EXCLUDED.config_revision
 AND quality_oauth_401_streaks.credentials_revision=EXCLUDED.credentials_revision
 AND quality_oauth_401_streaks.last_round_until=quality_oauth_401_streaks.claimed_until
 THEN quality_oauth_401_streaks.own_revoked_401 ELSE false END,
 config_updated_at=EXCLUDED.config_updated_at,config_revision=EXCLUDED.config_revision,credentials_revision=EXCLUDED.credentials_revision,claimed_until=EXCLUDED.claimed_until`, plan.ID, version, until, plan.AccountID)
	return err
}

// Called with the account -> plan locks held. This deletes only the quality
// rule; any owned restriction blocks cleanup until explicit reconciliation.
func completeQualityOAuth401RoundTx(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, until time.Time, e *service.QualityOAuth401Round) (bool, error) {
	if plan == nil || plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil || plan.PelicanConfig.Quality.OAuth401MaxRounds == 0 || e == nil {
		return false, nil
	}
	var enabled, lease bool
	var version time.Time
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT enabled,updated_at,COALESCE(running_until=$2 AND running_until>clock_timestamp(),false),pelican_config FROM scheduled_test_plans WHERE id=$1 FOR UPDATE`, plan.ID, until).Scan(&enabled, &version, &lease, &raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !enabled || !lease || !version.Equal(plan.UpdatedAt) {
		return false, nil
	}
	var cfg service.PelicanTestConfig
	if err = json.Unmarshal(raw, &cfg); err != nil {
		return false, err
	}
	if cfg.Quality == nil || cfg.Quality.OAuth401MaxRounds < 1 || cfg.Quality.OAuth401MaxRounds > 100 || cfg.Quality.OAuth401MaxRounds != plan.PelicanConfig.Quality.OAuth401MaxRounds {
		return false, nil
	}
	var typ, status, message, revision string
	var deleted sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT type,status,COALESCE(error_message,''),deleted_at,md5(COALESCE(credentials,'{}'::jsonb)::text) FROM accounts WHERE id=$1 FOR UPDATE`, plan.AccountID).Scan(&typ, &status, &message, &deleted, &revision)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var failures int
	var capturedVersion, capturedUntil time.Time
	var capturedRevision string
	var last sql.NullTime
	var revoked bool
	err = tx.QueryRowContext(ctx, `SELECT failures,config_updated_at,credentials_revision,claimed_until,last_round_until,own_revoked_401 FROM quality_oauth_401_streaks WHERE plan_id=$1 FOR UPDATE`, plan.ID).Scan(&failures, &capturedVersion, &capturedRevision, &capturedUntil, &last, &revoked)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !capturedUntil.Equal(until) || !capturedVersion.Equal(version) || last.Valid && !last.Time.Before(until) {
		return false, nil
	}
	trusted := revoked || e.HasDirectRevoked401
	qualifies := typ == service.AccountTypeOAuth && !deleted.Valid && status == "error" && qualityAccount401Pattern.MatchString(message) && capturedRevision == revision && cfg.TestChannel != "bps" && (e.AllDirect401 || e.All401OrUnavailable && trusted)
	if qualifies {
		failures = min(failures+1, 100)
	} else {
		failures = 0
	}
	if _, err = tx.ExecContext(ctx, `UPDATE quality_oauth_401_streaks SET failures=$2,last_round_until=$3,own_revoked_401=$4 WHERE plan_id=$1`, plan.ID, failures, until, qualifies && trusted); err != nil {
		return false, err
	}
	if failures < cfg.Quality.OAuth401MaxRounds {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM account_quality_states WHERE plan_id=$1)`, plan.ID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
