package handler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type imageStudioWorkerServiceStub struct {
	mu sync.Mutex

	job             *service.ImageStudioJob
	items           []*service.ImageStudioItem
	decryptErr      error
	claimCalls      int
	heartbeatCalls  int
	completedStatus []string
	completedCosts  []float64
	checkpointCalls int
	checkpointErr   error
	retryCalls      int
	settleCalls     int
}

func (s *imageStudioWorkerServiceStub) ClaimNextJob(context.Context, string, time.Time, time.Duration) (*service.ImageStudioJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	if s.job == nil {
		return nil, nil
	}
	job := s.job
	s.job = nil
	return job, nil
}

func (s *imageStudioWorkerServiceStub) HeartbeatJob(context.Context, string, string, time.Time, time.Duration) error {
	s.mu.Lock()
	s.heartbeatCalls++
	s.mu.Unlock()
	return nil
}

func (s *imageStudioWorkerServiceStub) DecryptJobRequest(*service.ImageStudioJob) (string, error) {
	if s.decryptErr != nil {
		return "", s.decryptErr
	}
	return `{"model":"gpt-image-1","prompt":"ciphertext was decrypted","n":2}`, nil
}

func (s *imageStudioWorkerServiceStub) ClaimNextItem(context.Context, string, string, time.Time) (*service.ImageStudioItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return nil, nil
	}
	item := s.items[0]
	s.items = s.items[1:]
	return item, nil
}

func (s *imageStudioWorkerServiceStub) CompleteWorkerItem(
	_ context.Context,
	_ *service.ImageStudioJob,
	_ *service.ImageStudioItem,
	_ string,
	image *service.ImageStudioImagePayload,
	actualCost float64,
	itemErr error,
	_ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := service.ImageStudioItemStatusFailed
	if itemErr == nil && image != nil {
		status = service.ImageStudioItemStatusSuccess
	}
	s.completedStatus = append(s.completedStatus, status)
	s.completedCosts = append(s.completedCosts, actualCost)
	return nil
}

func (s *imageStudioWorkerServiceStub) CheckpointWorkerItem(
	_ context.Context,
	_ *service.ImageStudioJob,
	item *service.ImageStudioItem,
	_ string,
	image *service.ImageStudioImagePayload,
	actualCost float64,
	_ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpointCalls++
	if s.checkpointErr != nil {
		return s.checkpointErr
	}
	item.Status = service.ImageStudioItemStatusPersisting
	item.CheckpointData = append([]byte(nil), image.Data...)
	item.CheckpointContentType = image.ContentType
	item.CheckpointActualCost = &actualCost
	return nil
}

func (s *imageStudioWorkerServiceStub) RetryWorkerItem(
	_ context.Context,
	_ *service.ImageStudioJob,
	item *service.ImageStudioItem,
	_ string,
	_ error,
	_ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retryCalls++
	item.Status = service.ImageStudioItemStatusPending
	item.AttemptCount++
	s.items = append([]*service.ImageStudioItem{item}, s.items...)
	return nil
}

func (s *imageStudioWorkerServiceStub) SettleJob(context.Context, string, string, time.Time) (*service.ImageStudioJob, error) {
	s.mu.Lock()
	s.settleCalls++
	s.mu.Unlock()
	return &service.ImageStudioJob{Status: service.ImageStudioJobStatusPartial}, nil
}

func TestImageStudioWorkerRuntimeProcessesItemsAndSettles(t *testing.T) {
	studio := &imageStudioWorkerServiceStub{
		job: &service.ImageStudioJob{
			ID:         "job-1",
			UserID:     10,
			Count:      2,
			LeaseOwner: "old-worker",
		},
		items: []*service.ImageStudioItem{
			{ID: "item-1", JobID: "job-1", SortOrder: 0},
			{ID: "item-2", JobID: "job-1", SortOrder: 1},
		},
	}
	var processorCalls int
	runtime := NewImageStudioWorkerRuntime(studio, func(
		_ context.Context,
		_ *service.ImageStudioJob,
		item *service.ImageStudioItem,
		body string,
	) (*service.ImageStudioImagePayload, float64, error) {
		processorCalls++
		require.Contains(t, body, `"n":1`)
		if item.SortOrder == 1 {
			return nil, 0, errors.New("upstream failed")
		}
		return &service.ImageStudioImagePayload{Data: []byte("png"), ContentType: "image/png"}, 0.04, nil
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-test",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Equal(t, 2, processorCalls)
	require.Equal(t, 1, studio.checkpointCalls)
	require.Equal(t, []string{
		service.ImageStudioItemStatusSuccess,
		service.ImageStudioItemStatusFailed,
	}, studio.completedStatus)
}

func TestImageStudioWorkerRuntimeRetriesOpenAIProviderErrorsThreeTimes(t *testing.T) {
	studio := &imageStudioWorkerServiceStub{
		job: &service.ImageStudioJob{
			ID:     "job-openai-retry",
			UserID: 10,
			Count:  1,
			Model:  "gpt-image-1",
		},
		items: []*service.ImageStudioItem{{
			ID:           "item-openai-retry",
			JobID:        "job-openai-retry",
			SortOrder:    0,
			Status:       service.ImageStudioItemStatusRunning,
			AttemptCount: 1,
		}},
	}
	var processorCalls int
	runtime := NewImageStudioWorkerRuntime(studio, func(
		context.Context,
		*service.ImageStudioJob,
		*service.ImageStudioItem,
		string,
	) (*service.ImageStudioImagePayload, float64, error) {
		processorCalls++
		return nil, 0, errors.New("temporary upstream failure")
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-openai-retry",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Equal(t, 3, processorCalls)
	require.Equal(t, 2, studio.retryCalls)
	require.Equal(t, []string{service.ImageStudioItemStatusFailed}, studio.completedStatus)
}

func TestImageStudioWorkerRuntimeResumesCheckpointWithoutCallingProvider(t *testing.T) {
	cost := 0.07
	studio := &imageStudioWorkerServiceStub{
		job: &service.ImageStudioJob{ID: "job-checkpoint", UserID: 10, Count: 1},
		items: []*service.ImageStudioItem{{
			ID:                    "item-checkpoint",
			JobID:                 "job-checkpoint",
			SortOrder:             0,
			Status:                service.ImageStudioItemStatusPersisting,
			CheckpointData:        []byte("checkpoint-png"),
			CheckpointContentType: "image/png",
			CheckpointActualCost:  &cost,
		}},
	}
	runtime := NewImageStudioWorkerRuntime(studio, func(
		context.Context,
		*service.ImageStudioJob,
		*service.ImageStudioItem,
		string,
	) (*service.ImageStudioImagePayload, float64, error) {
		t.Fatal("provider must not be called for a checkpointed item")
		return nil, 0, nil
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-checkpoint",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Zero(t, studio.checkpointCalls)
	require.Equal(t, []string{service.ImageStudioItemStatusSuccess}, studio.completedStatus)
}

func TestImageStudioWorkerRuntimeSettlesCostRecordedWhenCancelWinsCheckpointRace(t *testing.T) {
	studio := &imageStudioWorkerServiceStub{
		job: &service.ImageStudioJob{
			ID:     "job-cancel-before-checkpoint",
			UserID: 10,
			Count:  1,
			Model:  "gpt-image-1",
		},
		items: []*service.ImageStudioItem{{
			ID:           "item-cancel-before-checkpoint",
			JobID:        "job-cancel-before-checkpoint",
			Status:       service.ImageStudioItemStatusRunning,
			AttemptCount: 1,
		}},
		checkpointErr: service.ErrImageStudioCheckpointCancelled,
	}
	runtime := NewImageStudioWorkerRuntime(studio, func(
		context.Context,
		*service.ImageStudioJob,
		*service.ImageStudioItem,
		string,
	) (*service.ImageStudioImagePayload, float64, error) {
		return &service.ImageStudioImagePayload{
			Data:        []byte("generated"),
			ContentType: "image/png",
		}, 0.04, nil
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-cancel-before-checkpoint",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Equal(t, 1, studio.checkpointCalls)
	require.Empty(t, studio.completedStatus)
}

func TestImageStudioWorkerRuntimeDoesNotReplayUncheckpointedGrokRequest(t *testing.T) {
	studio := &imageStudioWorkerServiceStub{
		job: &service.ImageStudioJob{
			ID:     "job-grok-retry",
			UserID: 10,
			Count:  1,
			Model:  "grok-imagine-image",
		},
		items: []*service.ImageStudioItem{{
			ID:           "item-grok-retry",
			JobID:        "job-grok-retry",
			SortOrder:    0,
			Status:       service.ImageStudioItemStatusRunning,
			AttemptCount: 2,
		}},
	}
	var processorCalls int
	runtime := NewImageStudioWorkerRuntime(studio, func(
		context.Context,
		*service.ImageStudioJob,
		*service.ImageStudioItem,
		string,
	) (*service.ImageStudioImagePayload, float64, error) {
		processorCalls++
		return &service.ImageStudioImagePayload{Data: []byte("duplicate"), ContentType: "image/png"}, 0.04, nil
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-grok-retry",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Zero(t, processorCalls)
	require.Zero(t, studio.checkpointCalls)
	require.Equal(t, []string{service.ImageStudioItemStatusFailed}, studio.completedStatus)
}

func TestImageStudioWorkerRuntimeResumesCheckpointWhenPayloadCannotDecrypt(t *testing.T) {
	cost := 0.07
	studio := &imageStudioWorkerServiceStub{
		job:        &service.ImageStudioJob{ID: "job-checkpoint-poisoned", UserID: 10, Count: 1},
		decryptErr: errors.New("invalid ciphertext"),
		items: []*service.ImageStudioItem{{
			ID:                    "item-checkpoint-poisoned",
			JobID:                 "job-checkpoint-poisoned",
			SortOrder:             0,
			Status:                service.ImageStudioItemStatusPersisting,
			CheckpointData:        []byte("checkpoint-png"),
			CheckpointContentType: "image/png",
			CheckpointActualCost:  &cost,
		}},
	}
	runtime := NewImageStudioWorkerRuntime(studio, func(
		context.Context,
		*service.ImageStudioJob,
		*service.ImageStudioItem,
		string,
	) (*service.ImageStudioImagePayload, float64, error) {
		t.Fatal("provider must not be called for a checkpointed item")
		return nil, 0, nil
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-checkpoint-poisoned",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Zero(t, studio.checkpointCalls)
	require.Equal(t, []string{service.ImageStudioItemStatusSuccess}, studio.completedStatus)
	require.Len(t, studio.completedCosts, 1)
	require.InDelta(t, cost, studio.completedCosts[0], 0.000001)
}

func TestImageStudioWorkerRuntimeHeartbeatsWhileItemRunsAndStopsCleanly(t *testing.T) {
	studio := &imageStudioWorkerServiceStub{
		job: &service.ImageStudioJob{ID: "job-heartbeat", UserID: 10, Count: 1},
		items: []*service.ImageStudioItem{
			{ID: "item-1", JobID: "job-heartbeat", SortOrder: 0},
		},
	}
	release := make(chan struct{})
	runtime := NewImageStudioWorkerRuntime(studio, func(
		ctx context.Context,
		_ *service.ImageStudioJob,
		_ *service.ImageStudioItem,
		_ string,
	) (*service.ImageStudioImagePayload, float64, error) {
		select {
		case <-release:
			return &service.ImageStudioImagePayload{Data: []byte("png"), ContentType: "image/png"}, 0.04, nil
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 5 * time.Millisecond,
		Owner:             "worker-heartbeat",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.heartbeatCalls > 0
	}, time.Second, 5*time.Millisecond)
	close(release)
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()
	runtime.Stop()
}

func TestImageStudioWorkerRuntimeFailsPoisonedJobInsteadOfRetryingForever(t *testing.T) {
	studio := &imageStudioWorkerServiceStub{
		job:        &service.ImageStudioJob{ID: "job-poisoned", UserID: 10, Count: 2},
		decryptErr: errors.New("invalid ciphertext"),
		items: []*service.ImageStudioItem{
			{ID: "item-1", JobID: "job-poisoned", SortOrder: 0},
			{ID: "item-2", JobID: "job-poisoned", SortOrder: 1},
		},
	}
	runtime := NewImageStudioWorkerRuntime(studio, func(
		context.Context,
		*service.ImageStudioJob,
		*service.ImageStudioItem,
		string,
	) (*service.ImageStudioImagePayload, float64, error) {
		t.Fatal("processor must not run for an undecryptable job")
		return nil, 0, nil
	}, ImageStudioWorkerRuntimeOptions{
		WorkerCount:       1,
		PollInterval:      time.Millisecond,
		LeaseDuration:     100 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		Owner:             "worker-poisoned",
	})

	runtime.Start()
	require.Eventually(t, func() bool {
		studio.mu.Lock()
		defer studio.mu.Unlock()
		return studio.settleCalls == 1
	}, time.Second, 10*time.Millisecond)
	runtime.Stop()

	require.Equal(t, []string{
		service.ImageStudioItemStatusFailed,
		service.ImageStudioItemStatusFailed,
	}, studio.completedStatus)
}
