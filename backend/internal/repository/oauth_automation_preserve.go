package repository

import (
	"context"
	"encoding/json"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// The caller already holds the account row lock. Stale refresh snapshots never
// remove automation provenance or restore native settings after degradation.
func preserveOAuthAutomationAccount(ctx context.Context, client *dbent.Client, account *service.Account, extra map[string]any) (map[string]any, error) {
	rows, err := client.QueryContext(ctx, `SELECT COALESCE(extra,'{}'::jsonb),credentials,concurrency,schedulable FROM accounts WHERE id=$1`, account.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var raw, credentials []byte
	var concurrency int
	var schedulable bool
	if !rows.Next() {
		return nil, service.ErrAccountNotFound
	}
	if err = rows.Scan(&raw, &credentials, &concurrency, &schedulable); err != nil {
		return nil, err
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	// Release the result before inserting audit rows on the same transaction.
	if err = rows.Close(); err != nil {
		return nil, err
	}
	current := map[string]any{}
	if err = json.Unmarshal(raw, &current); err != nil {
		return nil, err
	}
	if extra == nil {
		extra = map[string]any{}
	}
	edit := service.OAuthAutomationManualEditFrom(ctx)
	for _, key := range []string{"auto_config_initial_revision", service.OAuthDegradationActionExtraKey, service.AutoConfigConcurrencyExtraKey} {
		delete(extra, key)
		if value, ok := current[key]; ok {
			extra[key] = value
		}
	}
	if edit.Concurrency {
		delete(extra, service.AutoConfigConcurrencyExtraKey)
		if _, held := current[service.AutoConfigConcurrencyExtraKey]; held {
			if err := insertAutoConfigEvent(ctx, client, service.AutoConfigEvent{AccountID: account.ID, AccountName: account.Name, Platform: account.Platform, Kind: service.AutoConfigEventManualRecovery, Details: service.AutoConfigEventDetails{RecoveredControls: []string{"concurrency"}, PreviousConcurrency: concurrency, Concurrency: account.Concurrency}}); err != nil {
				return nil, err
			}
		}
	}
	if marker, applied := current[service.OAuthDegradationActionExtraKey]; applied {
		raw, err := json.Marshal(marker)
		if err != nil {
			return nil, err
		}
		var state service.OAuthDegradationState
		if err = json.Unmarshal(raw, &state); err != nil {
			return nil, err
		}
		if state.NativeSchedulingPaused && !edit.Scheduling {
			account.Schedulable = schedulable
		}
		if !edit.Concurrency {
			account.Concurrency = concurrency
		}
		if !edit.Models {
			live := map[string]any{}
			if err = json.Unmarshal(credentials, &live); err != nil {
				return nil, err
			}
			if account.Credentials == nil {
				account.Credentials = map[string]any{}
			}
			for _, key := range []string{"model_mapping", "model_mapping_mode", "compact_model_mapping", "oauth_initial_selected_models"} {
				delete(account.Credentials, key)
				if value, ok := live[key]; ok {
					account.Credentials[key] = value
				}
			}
			for _, key := range []string{"openai_passthrough", "openai_oauth_passthrough"} {
				delete(extra, key)
				if value, ok := current[key]; ok {
					extra[key] = value
				}
			}
		}
	}
	return extra, nil
}

func stripOAuthAutomationExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	out := make(map[string]any, len(extra))
	for key, value := range extra {
		if key != "auto_config_initial_revision" && key != service.OAuthDegradationActionExtraKey && key != service.AutoConfigConcurrencyExtraKey {
			out[key] = value
		}
	}
	return out
}

// The bulk editor is explicit administrator intent. Hold release and its
// history share the same transaction as the native concurrency change.
func recordBulkOAuthConcurrencyRecovery(ctx context.Context, db sqlExecutor, ids []int64, concurrency int) error {
	rows, err := db.QueryContext(ctx, `SELECT id,name,platform,concurrency FROM accounts WHERE id=ANY($1) AND deleted_at IS NULL AND extra ? 'auto_config_concurrency' ORDER BY id FOR UPDATE`, pq.Array(ids))
	if err != nil {
		return err
	}
	var events []service.AutoConfigEvent
	for rows.Next() {
		var e service.AutoConfigEvent
		if err = rows.Scan(&e.AccountID, &e.AccountName, &e.Platform, &e.Details.PreviousConcurrency); err != nil {
			_ = rows.Close()
			return err
		}
		e.Kind = service.AutoConfigEventManualRecovery
		e.Details.Concurrency = concurrency
		e.Details.RecoveredControls = []string{"concurrency"}
		events = append(events, e)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		if err = insertAutoConfigEvent(ctx, db, e); err != nil {
			return err
		}
	}
	return nil
}
