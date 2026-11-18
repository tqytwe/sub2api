package service

import (
	"context"
	"sync"
	"time"
)

const priorityAnalysisMaxReaders = 2
const priorityAnalysisTimeout = 10 * time.Second

// A process-local budget shared by model catalog, analysis, and trend reads.
// Cancellation covers preparation and database queries; no result cache or
// routing state is involved.
type priorityAnalysisFlights struct {
	mu        sync.Mutex
	preparing chan struct{}
	shutdown  context.Context
	stop      context.CancelFunc
	closed    bool
}

func (b *priorityAnalysisFlights) prepare(ctx context.Context) (context.Context, func(), error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, nil, ErrPriorityLatencySeriesUnavailable
	}
	if b.preparing == nil {
		b.preparing = make(chan struct{}, priorityAnalysisMaxReaders)
		b.shutdown, b.stop = context.WithCancel(context.Background())
	}
	slots, shutdown := b.preparing, b.shutdown
	b.mu.Unlock()
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(shutdown, cancel)
	cleanup := func() { stop(); cancel() }
	select {
	case slots <- struct{}{}:
		return work, func() { <-slots; cleanup() }, nil
	case <-work.Done():
		cleanup()
		return nil, nil, work.Err()
	}
}
func (b *priorityAnalysisFlights) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.stop != nil {
		b.stop()
	}
}
