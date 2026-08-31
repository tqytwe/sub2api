package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	publicStatusSnapshotJobName   = "public_status_snapshot"
	publicStatusSnapshotSafeDelay = 5 * time.Minute
	publicStatusSnapshotTimeout   = 2 * time.Minute

	// Catch-up deliberately covers only a small recent tail. Each window is
	// independently computed from its historical boundary and INSERTed once,
	// so a restarted service cannot rewrite public history or start an
	// unbounded scan through usage_logs. Newest-first keeps the public status
	// useful while a bounded gap is filled across later runs.
	publicStatusSnapshotCatchupLookback         = 6 * time.Hour
	publicStatusSnapshotMaxCatchupWindowsPerRun = 4
)

var publicStatusSnapshotAdvisoryLockID = hashAdvisoryLockID("public:status:snapshot:leader")

// PublicStatusSnapshotWorker writes one immutable snapshot per completed hour.
// The public request path is intentionally not involved in raw-log aggregation.
type PublicStatusSnapshotWorker struct {
	repo PublicStatusSummaryRepository
	db   *sql.DB
	now  func() time.Time

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

func NewPublicStatusSnapshotWorker(repo PublicStatusSummaryRepository, db *sql.DB) *PublicStatusSnapshotWorker {
	return &PublicStatusSnapshotWorker{repo: repo, db: db, now: time.Now}
}

func (w *PublicStatusSnapshotWorker) Start() {
	if w == nil || w.repo == nil || w.db == nil {
		return
	}
	w.startOnce.Do(func() {
		w.stopCh = make(chan struct{})
		go w.run()
	})
}

func (w *PublicStatusSnapshotWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		if w.stopCh != nil {
			close(w.stopCh)
		}
	})
}

func (w *PublicStatusSnapshotWorker) run() {
	w.refreshOnce()
	for {
		next := nextPublicStatusSnapshotRun(w.now().UTC())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			w.refreshOnce()
		case <-w.stopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (w *PublicStatusSnapshotWorker) refreshOnce() {
	if w == nil || w.repo == nil || w.db == nil {
		return
	}
	windowEnd := publicStatusSnapshotWindowEnd(w.now().UTC())
	if windowEnd.IsZero() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), publicStatusSnapshotTimeout)
	defer cancel()
	release, acquired := tryAcquireDBAdvisoryLock(ctx, w.db, publicStatusSnapshotAdvisoryLockID)
	if !acquired {
		return
	}
	defer release()

	if err := w.refreshPendingWindows(ctx, windowEnd); err != nil {
		log.Printf("[PublicStatusSnapshotWorker] refresh through %s failed: %v", windowEnd.Format(time.RFC3339), err)
	}
}

// refreshPendingWindows attempts only a bounded recent tail. It fails closed
// when the aggregation watermark cannot be read and skips unready hours rather
// than allowing immutable snapshots to freeze partial source data.
func (w *PublicStatusSnapshotWorker) refreshPendingWindows(ctx context.Context, windowEnd time.Time) error {
	if w == nil || w.repo == nil || windowEnd.IsZero() {
		return nil
	}
	existing, err := w.repo.ListPublicStatusSnapshotWindowEnds(ctx, windowEnd.Add(-publicStatusSnapshotCatchupLookback), windowEnd)
	if err != nil {
		return fmt.Errorf("list recent status snapshots: %w", err)
	}
	for _, candidate := range publicStatusSnapshotRefreshWindowEnds(windowEnd, existing) {
		ready, err := w.repo.IsPublicStatusSnapshotWindowReady(ctx, candidate)
		if err != nil {
			return fmt.Errorf("check aggregation watermark for %s: %w", candidate.Format(time.RFC3339), err)
		}
		if !ready {
			continue
		}
		if err := w.repo.RefreshPublicStatusSnapshot(ctx, candidate); err != nil {
			return fmt.Errorf("refresh public status snapshot for %s: %w", candidate.Format(time.RFC3339), err)
		}
	}
	return nil
}

func publicStatusSnapshotWindowEnd(now time.Time) time.Time {
	return now.UTC().Add(-publicStatusSnapshotSafeDelay).Truncate(time.Hour)
}

// publicStatusSnapshotRefreshWindowEnds selects at most four missing completed
// hours in the most recent six-hour horizon. The snapshot table remains the
// source of truth for what has already been persisted; the writer additionally
// uses ON CONFLICT DO NOTHING to preserve idempotency under races.
func publicStatusSnapshotRefreshWindowEnds(windowEnd time.Time, existing []time.Time) []time.Time {
	if windowEnd.IsZero() {
		return nil
	}
	windowEnd = windowEnd.UTC().Truncate(time.Hour)
	oldest := windowEnd.Add(-(publicStatusSnapshotCatchupLookback - time.Hour))
	persisted := make(map[time.Time]struct{}, len(existing))
	for _, value := range existing {
		if value.IsZero() {
			continue
		}
		persisted[value.UTC().Truncate(time.Hour)] = struct{}{}
	}

	out := make([]time.Time, 0, publicStatusSnapshotMaxCatchupWindowsPerRun)
	for candidate := windowEnd; !candidate.Before(oldest); candidate = candidate.Add(-time.Hour) {
		if _, exists := persisted[candidate]; exists {
			continue
		}
		out = append(out, candidate)
		if len(out) == publicStatusSnapshotMaxCatchupWindowsPerRun {
			break
		}
	}
	return out
}

func nextPublicStatusSnapshotRun(now time.Time) time.Time {
	return now.UTC().Truncate(publicStatusSnapshotSafeDelay).Add(publicStatusSnapshotSafeDelay)
}
