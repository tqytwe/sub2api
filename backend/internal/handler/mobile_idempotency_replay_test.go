package handler

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// mobileReplayIdempotencyRepo is intentionally small: handler tests only need
// the successful replay path that the real PostgreSQL repository implements.
type mobileReplayIdempotencyRepo struct {
	mu      sync.Mutex
	nextID  int64
	records map[string]*service.IdempotencyRecord
}

func newMobileReplayIdempotencyRepo() *mobileReplayIdempotencyRepo {
	return &mobileReplayIdempotencyRepo{
		nextID:  1,
		records: make(map[string]*service.IdempotencyRecord),
	}
}

func (r *mobileReplayIdempotencyRepo) key(scope, keyHash string) string {
	return scope + "|" + keyHash
}

func cloneMobileReplayIdempotencyRecord(in *service.IdempotencyRecord) *service.IdempotencyRecord {
	if in == nil {
		return nil
	}
	out := *in
	if in.LockedUntil != nil {
		value := *in.LockedUntil
		out.LockedUntil = &value
	}
	if in.ResponseStatus != nil {
		value := *in.ResponseStatus
		out.ResponseStatus = &value
	}
	if in.ResponseBody != nil {
		value := *in.ResponseBody
		out.ResponseBody = &value
	}
	if in.ErrorReason != nil {
		value := *in.ErrorReason
		out.ErrorReason = &value
	}
	return &out
}

func (r *mobileReplayIdempotencyRepo) CreateProcessing(_ context.Context, record *service.IdempotencyRecord) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.key(record.Scope, record.IdempotencyKeyHash)
	if _, exists := r.records[key]; exists {
		return false, nil
	}
	copy := cloneMobileReplayIdempotencyRecord(record)
	copy.ID = r.nextID
	r.nextID++
	r.records[key] = copy
	record.ID = copy.ID
	return true, nil
}

func (r *mobileReplayIdempotencyRepo) GetByScopeAndKeyHash(_ context.Context, scope, keyHash string) (*service.IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneMobileReplayIdempotencyRecord(r.records[r.key(scope, keyHash)]), nil
}

func (r *mobileReplayIdempotencyRepo) TryReclaim(_ context.Context, _ int64, _ string, _ time.Time, _ time.Time, _ time.Time) (bool, error) {
	return false, nil
}

func (r *mobileReplayIdempotencyRepo) ExtendProcessingLock(_ context.Context, _ int64, _ string, _ time.Time, _ time.Time) (bool, error) {
	return false, nil
}

func (r *mobileReplayIdempotencyRepo) MarkSucceeded(_ context.Context, id int64, status int, body string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.records {
		if record.ID != id {
			continue
		}
		record.Status = service.IdempotencyStatusSucceeded
		record.LockedUntil = nil
		record.ExpiresAt = expiresAt
		record.ResponseStatus = &status
		record.ResponseBody = &body
		record.ErrorReason = nil
	}
	return nil
}

func (r *mobileReplayIdempotencyRepo) MarkFailedRetryable(_ context.Context, id int64, reason string, lockedUntil, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range r.records {
		if record.ID != id {
			continue
		}
		record.Status = service.IdempotencyStatusFailedRetryable
		record.LockedUntil = &lockedUntil
		record.ExpiresAt = expiresAt
		record.ErrorReason = &reason
	}
	return nil
}

func (r *mobileReplayIdempotencyRepo) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}
