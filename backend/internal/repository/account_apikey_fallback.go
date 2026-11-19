package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) LoadOpenAIAPIKeyFallbackState(ctx context.Context) (*time.Time, string, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT opened_at,trigger_reason
 FROM openai_apikey_fallback_state ORDER BY opened_at DESC LIMIT 1`)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, "", rows.Err()
	}
	var enabledAt time.Time
	var reason string
	if err = rows.Scan(&enabledAt, &reason); err != nil {
		return nil, "", err
	}
	enabledAt = enabledAt.UTC()
	return &enabledAt, reason, rows.Err()
}

// EnableOpenAIAPIKeyFallback opens only the selected, active OpenAI API Key
// while the saved policy revision still matches. Account state, provenance and
// its idempotent audit event commit together.
func (r *accountRepository) EnableOpenAIAPIKeyFallback(ctx context.Context, accountID int64, revision, reason string) (bool, error) {
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return false, errors.New("OpenAI API Key fallback requires transactional database")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var raw string
	if err = tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1 FOR SHARE`, service.SettingKeyOAuthAutoConfig).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	var cfg service.OAuthAutoConfig
	if err = json.Unmarshal([]byte(raw), &cfg); err != nil {
		return false, err
	}
	if !cfg.OpenAIAPIKeyFallback.Enabled || cfg.Revision != revision || reason != "oauth_unavailable" && reason != "metric_below_threshold" {
		return false, nil
	}
	selected := false
	for _, id := range cfg.OpenAIAPIKeyFallback.AccountIDs {
		if id == accountID {
			selected = true
			break
		}
	}
	if !selected {
		return false, nil
	}
	var schedulable bool
	err = tx.QueryRowContext(ctx, `SELECT schedulable FROM accounts WHERE id=$1 AND platform='openai' AND type='apikey'
 AND status='active' AND parent_account_id IS NULL AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at>NOW()) FOR UPDATE`, accountID).Scan(&schedulable)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if schedulable {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE accounts SET schedulable=TRUE,updated_at=GREATEST(clock_timestamp(),updated_at+INTERVAL '1 microsecond')
 WHERE id=$1 AND schedulable=FALSE`, accountID)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO openai_apikey_fallback_state(account_id,config_revision,trigger_reason,original_schedulable)
 VALUES($1,$2,$3,FALSE) ON CONFLICT(account_id) DO UPDATE SET config_revision=EXCLUDED.config_revision,
 trigger_reason=EXCLUDED.trigger_reason,original_schedulable=EXCLUDED.original_schedulable,opened_at=clock_timestamp(),updated_at=clock_timestamp()`, accountID, revision, reason); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO openai_apikey_fallback_events(account_id,config_revision,trigger_reason,action)
 VALUES($1,$2,$3,'opened') ON CONFLICT(account_id,config_revision,trigger_reason) DO NOTHING`, accountID, revision, reason); err != nil {
		return false, err
	}
	var name string
	if err = tx.QueryRowContext(ctx, `SELECT name FROM accounts WHERE id=$1`, accountID).Scan(&name); err != nil {
		return false, err
	}
	if err = insertAutoConfigEvent(ctx, tx, service.AutoConfigEvent{AccountID: accountID, AccountName: name, Platform: service.PlatformOpenAI, Kind: service.AutoConfigEventFallback, Details: service.AutoConfigEventDetails{FallbackReason: reason}}); err != nil {
		return false, err
	}
	if err = enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, accountID)
	return true, nil
}
