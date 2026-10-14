package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type subscriptionSettlementRecoverer interface {
	RecoverSubscriptionSettlements(context.Context, int) (int, error)
}

type subscriptionSettlementWorker struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

func startSubscriptionSettlementWorker(repo UsageBillingRepository) *subscriptionSettlementWorker {
	r, ok := repo.(subscriptionSettlementRecoverer)
	if !ok {
		return nil
	}
	w := &subscriptionSettlementWorker{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-w.stop:
				return
			case <-timer.C:
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			_, err := r.RecoverSubscriptionSettlements(ctx, 20)
			cancel()
			if err != nil {
				slog.Error("subscription_settlement_recovery_failed")
			}
			timer.Reset(15 * time.Second)
		}
	}()
	return w
}

func (w *subscriptionSettlementWorker) Stop() {
	if w == nil {
		return
	}
	w.once.Do(func() { close(w.stop) })
	<-w.done
}
