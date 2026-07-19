package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// BatchImageRuntimeState is shared by the public API, worker runtime, and
// operations health endpoint so readiness cannot drift between components.
type BatchImageRuntimeState struct {
	cfg      *config.Config
	queue    BatchImageQueue
	database BatchImageDatabaseHealthChecker

	mu            sync.RWMutex
	workerRunning bool
	lastError     string
	lastErrorAt   *time.Time
}

type BatchImageRuntimeSnapshot struct {
	Enabled       bool                 `json:"enabled"`
	QueueEnabled  bool                 `json:"queue_enabled"`
	DatabaseReady bool                 `json:"database_ready"`
	RedisReady    bool                 `json:"redis_ready"`
	WorkerRunning bool                 `json:"worker_running"`
	Ready         bool                 `json:"ready"`
	Queue         BatchImageQueueStats `json:"queue"`
	LastError     string               `json:"last_error,omitempty"`
	LastErrorAt   *time.Time           `json:"last_error_at,omitempty"`
}

type BatchImageDatabaseHealthChecker interface {
	PingContext(ctx context.Context) error
}

func NewBatchImageRuntimeState(queue BatchImageQueue, database BatchImageDatabaseHealthChecker, cfg *config.Config) *BatchImageRuntimeState {
	return &BatchImageRuntimeState{cfg: cfg, queue: queue, database: database}
}

func (s *BatchImageRuntimeState) RequireReady(ctx context.Context) error {
	if s == nil || s.cfg == nil || !s.cfg.BatchImage.QueueEnabled || !s.WorkerRunning() {
		return ErrBatchImageRuntimeNotReady
	}
	if s.database == nil {
		return ErrBatchImageRuntimeNotReady
	}
	if err := s.database.PingContext(ctx); err != nil {
		s.RecordError(err)
		return ErrBatchImageRuntimeNotReady
	}
	checker, ok := s.queue.(BatchImageQueueHealthChecker)
	if !ok || checker == nil {
		return ErrBatchImageRuntimeNotReady
	}
	if err := checker.Ping(ctx); err != nil {
		s.RecordError(err)
		return ErrBatchImageRuntimeNotReady
	}
	return nil
}

func (s *BatchImageRuntimeState) SetWorkerRunning(running bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.workerRunning = running
	s.mu.Unlock()
}

func (s *BatchImageRuntimeState) WorkerRunning() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workerRunning
}

func (s *BatchImageRuntimeState) RecordError(err error) {
	if s == nil || err == nil {
		return
	}
	s.mu.Lock()
	s.lastError = sanitizeBatchImagePublicMessage(err.Error())
	now := time.Now().UTC()
	s.lastErrorAt = &now
	s.mu.Unlock()
}

func (s *BatchImageRuntimeState) LastError() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

func (s *BatchImageRuntimeState) Snapshot(ctx context.Context) BatchImageRuntimeSnapshot {
	snapshot := BatchImageRuntimeSnapshot{}
	if s == nil {
		return snapshot
	}
	if s.cfg != nil {
		snapshot.Enabled = s.cfg.BatchImage.Enabled
		snapshot.QueueEnabled = s.cfg.BatchImage.QueueEnabled
	}
	s.mu.RLock()
	snapshot.WorkerRunning = s.workerRunning
	snapshot.LastError = s.lastError
	if s.lastErrorAt != nil {
		lastErrorAt := *s.lastErrorAt
		snapshot.LastErrorAt = &lastErrorAt
	}
	s.mu.RUnlock()

	if s.database != nil {
		if err := s.database.PingContext(ctx); err == nil {
			snapshot.DatabaseReady = true
		} else {
			s.RecordError(err)
			snapshot.LastError = sanitizeBatchImagePublicMessage(err.Error())
			now := time.Now().UTC()
			snapshot.LastErrorAt = &now
		}
	}
	checker, checkerOK := s.queue.(BatchImageQueueHealthChecker)
	if checkerOK && checker != nil {
		if err := checker.Ping(ctx); err == nil {
			snapshot.RedisReady = true
		} else {
			s.RecordError(err)
			snapshot.LastError = sanitizeBatchImagePublicMessage(err.Error())
			now := time.Now().UTC()
			snapshot.LastErrorAt = &now
		}
	}
	if statsReader, ok := s.queue.(BatchImageQueueStatsReader); ok && statsReader != nil {
		stats, err := statsReader.Stats(ctx)
		if err != nil {
			s.RecordError(err)
			snapshot.LastError = sanitizeBatchImagePublicMessage(err.Error())
		} else {
			snapshot.Queue = stats
		}
	}
	snapshot.Ready = snapshot.Enabled && snapshot.QueueEnabled && snapshot.DatabaseReady && snapshot.RedisReady && snapshot.WorkerRunning
	return snapshot
}
