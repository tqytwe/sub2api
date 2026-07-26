package service

import (
	"context"
	"sync"
	"time"
)

type mobilePushDispatcher interface {
	DispatchOne(context.Context) (bool, error)
}

type MobilePushWorker struct {
	dispatcher mobilePushDispatcher
	interval   time.Duration
	batchSize  int
	mu         sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
}

func (w *MobilePushWorker) Start() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go func() {
		defer close(w.done)
		_ = w.Run(ctx)
	}()
}

func (w *MobilePushWorker) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.cancel = nil
	w.done = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

func NewMobilePushWorker(dispatcher mobilePushDispatcher, interval time.Duration, batchSize int) *MobilePushWorker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 || batchSize > 500 {
		batchSize = 50
	}
	return &MobilePushWorker{dispatcher: dispatcher, interval: interval, batchSize: batchSize}
}

func (w *MobilePushWorker) Run(ctx context.Context) error {
	if w == nil || w.dispatcher == nil {
		return nil
	}
	_, _ = w.RunOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, _ = w.RunOnce(ctx)
		}
	}
}

func (w *MobilePushWorker) RunOnce(ctx context.Context) (int, error) {
	if w == nil || w.dispatcher == nil {
		return 0, nil
	}
	processedCount := 0
	for processedCount < w.batchSize {
		processed, err := w.dispatcher.DispatchOne(ctx)
		if err != nil {
			return processedCount, err
		}
		if !processed {
			return processedCount, nil
		}
		processedCount++
	}
	return processedCount, nil
}
