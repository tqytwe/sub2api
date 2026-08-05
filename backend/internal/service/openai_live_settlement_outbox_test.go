package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This is intentionally process-local only for the service contract test. The
// production repository uses PostgreSQL claims; constructing a second service
// against this same store models a process restart without using Redis state.
type liveTestSettlementOutbox struct {
	mu      sync.Mutex
	jobs    map[string]LiveSettlementJob
	nextID  int64
	claimed map[int64]string
}

func (r *liveTestSettlementOutbox) Enqueue(_ context.Context, record *LiveCallRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.jobs == nil {
		r.jobs = make(map[string]LiveSettlementJob)
	}
	if existing, ok := r.jobs[record.CallHash]; ok {
		if existing.Status == LiveSettlementClosing {
			existing.Record = cloneLiveCallRecord(record)
			r.jobs[record.CallHash] = existing
		}
		return nil
	}
	r.nextID++
	r.jobs[record.CallHash] = LiveSettlementJob{
		ID: r.nextID, CallHash: record.CallHash, Record: cloneLiveCallRecord(record),
		Status: LiveSettlementClosing,
	}
	return nil
}

func (r *liveTestSettlementOutbox) Activate(_ context.Context, callHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[callHash]
	if !ok {
		return errors.New("missing job")
	}
	job.Status = LiveSettlementReady
	job.AvailableAt = time.Now().UTC()
	r.jobs[callHash] = job
	return nil
}

func (r *liveTestSettlementOutbox) ListClosing(_ context.Context, _ int) ([]LiveSettlementJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	jobs := make([]LiveSettlementJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		if job.Status == LiveSettlementClosing {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (r *liveTestSettlementOutbox) Claim(_ context.Context, workerID string, _ int, _ time.Duration) ([]LiveSettlementJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed == nil {
		r.claimed = make(map[int64]string)
	}
	jobs := make([]LiveSettlementJob, 0, len(r.jobs))
	for callHash, job := range r.jobs {
		if job.Status != LiveSettlementReady || r.claimed[job.ID] != "" {
			continue
		}
		r.claimed[job.ID] = workerID
		job.ClaimedBy = workerID
		r.jobs[callHash] = job
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *liveTestSettlementOutbox) Ack(_ context.Context, id int64, workerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed[id] != workerID {
		return errors.New("lost claim")
	}
	for callHash, job := range r.jobs {
		if job.ID == id {
			delete(r.jobs, callHash)
			delete(r.claimed, id)
			return nil
		}
	}
	return errors.New("missing job")
}

func (r *liveTestSettlementOutbox) Retry(_ context.Context, id int64, workerID string, availableAt time.Time, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed[id] != workerID {
		return errors.New("lost claim")
	}
	for callHash, job := range r.jobs {
		if job.ID == id {
			job.Attempts++
			job.AvailableAt = availableAt
			job.ClaimedBy = ""
			r.jobs[callHash] = job
			delete(r.claimed, id)
			return nil
		}
	}
	return errors.New("missing job")
}

func cloneLiveCallRecord(record *LiveCallRecord) *LiveCallRecord {
	if record == nil {
		return nil
	}
	copy := *record
	return &copy
}

func TestLiveSettlementOutboxSurvivesRestartAndReleasesLeaseAfterSuccess(t *testing.T) {
	groupID := int64(44)
	record := &LiveCallRecord{
		CallID: "call_durable_settlement", CallHash: hashLiveCallID("call_durable_settlement"),
		AccountID: 11, APIKeyID: 22, UserID: 33, GroupID: groupID, LeaseID: "lease-1",
		Model: "gpt-live-test", CreatedAt: time.Now().Add(-time.Second), ExpiresAt: time.Now().Add(time.Hour),
		Usage: OpenAIUsage{InputTokens: 10, OutputTokens: 4},
	}
	store := &liveTestStore{}
	require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
	outbox := &liveTestSettlementOutbox{}
	concurrency := &liveTestConcurrencyCache{}

	firstAttempt := newLiveBillingServiceForLifecycleTest(
		record, store, concurrency, &openAIRecordUsageLogRepoStub{inserted: true},
		&openAIRecordUsageBillingRepoStub{err: errors.New("database temporarily unavailable")},
	)
	firstAttempt.liveSettlementOutbox = outbox
	firstAttempt.finalizeLiveCall(record)

	concurrency.mu.Lock()
	require.Zero(t, concurrency.releases, "failed settlement must retain the lease")
	concurrency.mu.Unlock()

	// A new service instance represents the original process having exited
	// immediately after the failed attempt. The durable job, not a goroutine,
	// must drive the eventual settlement.
	restarted := newLiveBillingServiceForLifecycleTest(
		record, store, concurrency, &openAIRecordUsageLogRepoStub{inserted: true},
		&openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}},
	)
	restarted.liveSettlementOutbox = outbox
	restarted.processLiveSettlementBatch(context.Background())

	concurrency.mu.Lock()
	require.Equal(t, 1, concurrency.releases)
	concurrency.mu.Unlock()
	outbox.mu.Lock()
	require.Empty(t, outbox.jobs)
	outbox.mu.Unlock()
}

func TestLiveSettlementRecoveryActivatesClosingRowAfterRedisClosed(t *testing.T) {
	groupID := int64(44)
	record := &LiveCallRecord{
		CallID: "call_closing_recovery", CallHash: hashLiveCallID("call_closing_recovery"),
		AccountID: 11, APIKeyID: 22, UserID: 33, GroupID: groupID, LeaseID: "lease-2",
		Model: "gpt-live-test", CreatedAt: time.Now().Add(-time.Second), ExpiresAt: time.Now().Add(time.Hour),
		Usage: OpenAIUsage{InputTokens: 8, OutputTokens: 3},
	}
	store := &liveTestStore{}
	require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
	first, err := store.MarkLiveCallClosed(context.Background(), record.CallHash, time.Hour)
	require.NoError(t, err)
	require.True(t, first)
	outbox := &liveTestSettlementOutbox{}
	require.NoError(t, outbox.Enqueue(context.Background(), record))
	concurrency := &liveTestConcurrencyCache{}
	restarted := newLiveBillingServiceForLifecycleTest(
		record, store, concurrency, &openAIRecordUsageLogRepoStub{inserted: true},
		&openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}},
	)
	restarted.liveSettlementOutbox = outbox

	restarted.recoverLiveSettlementClosures(context.Background())
	restarted.processLiveSettlementBatch(context.Background())

	concurrency.mu.Lock()
	require.Equal(t, 1, concurrency.releases)
	concurrency.mu.Unlock()
	outbox.mu.Lock()
	require.Empty(t, outbox.jobs)
	outbox.mu.Unlock()
}
