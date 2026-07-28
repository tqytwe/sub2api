package middleware

import (
	"context"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type DailyCardBillingSignal struct {
	scheduled atomic.Bool
}

func (s *DailyCardBillingSignal) MarkScheduled() {
	if s != nil {
		s.scheduled.Store(true)
	}
}

func (s *DailyCardBillingSignal) Scheduled() bool {
	return s != nil && s.scheduled.Load()
}

func MarkDailyCardBillingScheduled(ctx context.Context) {
	if ctx == nil {
		return
	}
	if signal, _ := ctx.Value(ctxkey.DailyCardBillingSignal).(*DailyCardBillingSignal); signal != nil {
		signal.MarkScheduled()
	}
}
