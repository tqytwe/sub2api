package service

import (
	"context"
	"time"
)

// OAuth refresh results must obey the deadline even if the context timer's
// cancellation callback has not run yet. Preserve explicit cancellation first.
func oauthRefreshContextErr(ctx context.Context, now func() time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if now == nil {
			now = time.Now
		}
		if !now().Before(deadline) {
			return context.DeadlineExceeded
		}
	}
	return nil
}
