package repository

import (
	"context"
	"time"
)

// Bound each maintenance pass; durable current fallback state is retained.
func (r *accountRepository) PruneOAuthAutomationHistory(ctx context.Context, cutoff time.Time) error {
	for _, table := range []string{"account_auto_config_events", "openai_apikey_fallback_events"} {
		if _, err := r.sql.ExecContext(ctx, `DELETE FROM `+table+` WHERE id IN (SELECT id FROM `+table+` WHERE created_at<$1 ORDER BY id LIMIT 1000)`, cutoff); err != nil {
			return err
		}
	}
	return nil
}
