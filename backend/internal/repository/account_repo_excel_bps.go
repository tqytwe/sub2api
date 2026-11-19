package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.AccountExcelBPSRepository = (*accountRepository)(nil)

// DisableExcelBPSOn403 changes only the protocol switch. A stale request cannot
// disable an account whose credentials or opt-in have since been changed.
func (r *accountRepository) DisableExcelBPSOn403(ctx context.Context, account *service.Account) (bool, error) {
	if !account.IsExcelBPSAutoDisableOn403Enabled() {
		return false, nil
	}
	if dbent.TxFromContext(ctx) != nil {
		return r.disableExcelBPSOn403InTx(ctx, account)
	}
	tx, err := r.client.Tx(ctx)
	if errors.Is(err, dbent.ErrTxStarted) {
		return r.disableExcelBPSOn403InTx(ctx, account)
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	changed, err := r.disableExcelBPSOn403InTx(dbent.NewTxContext(ctx, tx), account)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if changed {
		r.syncSchedulerAccountSnapshot(ctx, account.ID)
	}
	return changed, nil
}

func (r *accountRepository) disableExcelBPSOn403InTx(ctx context.Context, account *service.Account) (bool, error) {
	credentials, err := json.Marshal(account.Credentials)
	if err != nil {
		return false, err
	}
	client := clientFromContext(ctx, r.client)
	// The marker is written with the switch so the account list can flag the
	// suspected Excel ban until an admin turns the protocol back on.
	result, err := client.ExecContext(ctx, `
UPDATE accounts
SET extra = jsonb_set(extra, '{openai_excel_bps}', 'false'::jsonb) || jsonb_build_object('openai_excel_bps_403_disabled_at', $3::text), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL AND parent_account_id IS NULL
  AND platform = 'openai' AND type = 'oauth'
  AND extra -> 'openai_connections_managed' IS DISTINCT FROM 'true'::jsonb
  AND credentials = $2::jsonb
  AND extra -> 'openai_excel_bps' = 'true'::jsonb
  AND extra -> 'openai_excel_bps_auto_disable_on_403' = 'true'::jsonb`,
		account.ID, string(credentials), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return false, err
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &account.ID, nil, nil); err != nil {
		return false, err
	}
	return true, nil
}

// The caller already holds the account lock. Only legacy BPS accounts can
// own recovery markers. Other account updates keep their existing query path.
func mergeLockedExcelBPS403Markers(ctx context.Context, client *dbent.Client, account *service.Account, extra map[string]any) (map[string]any, error) {
	if account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeOAuth {
		return service.StripExcelBPS403Markers(extra), nil
	}
	rows, err := client.QueryContext(ctx, `SELECT extra FROM accounts WHERE id=$1 AND deleted_at IS NULL`, account.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAccountNotFound
	}
	var raw []byte
	if err := rows.Scan(&raw); err != nil {
		return nil, err
	}
	var current map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &current); err != nil {
			return nil, err
		}
	}
	return service.MergeExcelBPS403Marker(extra, current), rows.Err()
}
