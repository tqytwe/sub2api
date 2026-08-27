package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

const (
	canvasPromptMirrorInterval      = 30 * time.Minute
	canvasPromptMirrorRunTimeout    = 2 * time.Minute
	canvasPromptMirrorLeaderLockKey = "mobile-canvas-prompt-mirror-refresh"
	canvasPromptMirrorLeaderLockTTL = 3 * time.Minute
)

// CanvasPromptMirrorWorker refreshes the durable snapshot independently of
// request handling. A failed refresh is logged and leaves the previous good
// snapshot untouched; the mobile client can therefore keep using its cache.
type CanvasPromptMirrorWorker struct {
	service   *CanvasPromptMirrorService
	interval  time.Duration
	timeout   time.Duration
	lockCache LeaderLockCache
	db        *sql.DB
	owner     string
	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewCanvasPromptMirrorWorker(service *CanvasPromptMirrorService, interval, timeout time.Duration) *CanvasPromptMirrorWorker {
	if interval <= 0 {
		interval = canvasPromptMirrorInterval
	}
	if timeout <= 0 {
		timeout = canvasPromptMirrorRunTimeout
	}
	return &CanvasPromptMirrorWorker{
		service: service, interval: interval, timeout: timeout,
		owner: fmt.Sprintf("canvas-mirror-%d", os.Getpid()),
	}
}

func (w *CanvasPromptMirrorWorker) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) *CanvasPromptMirrorWorker {
	if w != nil {
		w.lockCache = lockCache
		w.db = db
	}
	return w
}

func (w *CanvasPromptMirrorWorker) Start() {
	if w == nil || w.service == nil {
		return
	}
	w.mu.Lock()
	if w.cancel != nil {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	done := w.done
	w.mu.Unlock()
	go func() {
		defer close(done)
		w.runOnce(ctx)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
}

func (w *CanvasPromptMirrorWorker) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.cancel, w.done = nil, nil
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

func (w *CanvasPromptMirrorWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.service == nil {
		return nil
	}
	return w.runOnce(ctx)
}

func (w *CanvasPromptMirrorWorker) runOnce(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, w.timeout)
	defer cancel()
	release, acquired := tryAcquireSingletonLeaderLock(ctx, w.lockCache, w.db, canvasPromptMirrorLeaderLockKey, w.owner, canvasPromptMirrorLeaderLockTTL)
	if !acquired {
		return nil
	}
	defer release()
	_, err := w.service.Refresh(ctx)
	if err != nil {
		slog.Default().Warn("canvas prompt mirror refresh failed", "error", err)
	}
	return err
}
