package service

import (
	"context"
	"time"
)

type balanceLedgerSourceContextKey struct{}

type balanceLedgerTotalRechargedAdjuster interface {
	AdjustTotalRecharged(ctx context.Context, id int64, delta float64) error
}

// BalanceLedgerSourceOverride lets an outer workflow keep its domain source as
// the ledger source while reusing an inner balance-grant implementation.
type BalanceLedgerSourceOverride struct {
	SourceType     string
	SourceID       string
	IdempotencyKey string
	ActorType      string
	ActorUserID    *int64
	Description    string
	Metadata       map[string]any
	BalancePolicy  string
	FrozenPolicy   string
	CreatedAt      *time.Time
}

func ContextWithBalanceLedgerSource(ctx context.Context, source BalanceLedgerSourceOverride) context.Context {
	return context.WithValue(ctx, balanceLedgerSourceContextKey{}, source)
}

func BalanceLedgerSourceFromContext(ctx context.Context) (BalanceLedgerSourceOverride, bool) {
	source, ok := ctx.Value(balanceLedgerSourceContextKey{}).(BalanceLedgerSourceOverride)
	return source, ok
}
