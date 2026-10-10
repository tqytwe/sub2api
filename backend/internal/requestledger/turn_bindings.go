package requestledger

import (
	"context"
	"sync"
)

// TurnBindings maps a forwarding loop's local turn numbers to immutable
// receipts, including loops restarted for account failover.
type TurnBindings struct {
	mu    sync.Mutex
	turns map[int]*Handle
}

func NewTurnBindings(ctx context.Context) *TurnBindings {
	return &TurnBindings{turns: map[int]*Handle{1: FromContext(CurrentContext(ctx))}}
}
func (b *TurnBindings) Bind(turn int, ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.turns[turn] = FromContext(CurrentContext(ctx))
}
func (b *TurnBindings) Context(turn int, ctx context.Context) context.Context {
	b.mu.Lock()
	h := b.turns[turn]
	b.mu.Unlock()
	if h == nil {
		return ctx
	}
	return WithHandle(ctx, h)
}
