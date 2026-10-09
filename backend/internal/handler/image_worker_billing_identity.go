package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// imageWorkerBillingContext installs the durable task/item correlation used by
// existing settlements. Its exact legacy key must survive worker retries.
func imageWorkerBillingContext(ctx context.Context, requestID string) context.Context {
	// A submission/connection identity must not collapse distinct durable items.
	// Keep the legacy client:<task/item> settlement key byte-for-byte on retries.
	ctx = context.WithValue(ctx, ctxkey.UsageBillingRequestID, "")
	return context.WithValue(ctx, ctxkey.ClientRequestID, requestID)
}
