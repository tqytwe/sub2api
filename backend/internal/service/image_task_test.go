package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageTaskMemoryStore struct {
	mu      sync.Mutex
	task    *ImageTaskRecord
	ttl     time.Duration
	saveErr error
	getErr  error
}

func (s *imageTaskMemoryStore) Save(_ context.Context, task *ImageTaskRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	copy := *task
	s.task = &copy
	s.ttl = ttl
	return nil
}

func (s *imageTaskMemoryStore) Get(_ context.Context, _ string) (*ImageTaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.task == nil {
		return nil, ErrImageTaskNotFound
	}
	copy := *s.task
	return &copy, nil
}

func (s *imageTaskMemoryStore) SaveIfStatus(_ context.Context, task *ImageTaskRecord, expectedStatus string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return false, s.saveErr
	}
	if s.task == nil {
		return false, ErrImageTaskNotFound
	}
	if s.task.Status != expectedStatus {
		return false, nil
	}
	copy := *task
	s.task = &copy
	s.ttl = ttl
	return true, nil
}

func (s *imageTaskMemoryStore) TouchHeartbeat(_ context.Context, _ string, heartbeatAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return s.getErr
	}
	if s.task == nil {
		return ErrImageTaskNotFound
	}
	if s.task.Status != ImageTaskStatusProcessing {
		return nil
	}
	value := heartbeatAt.UTC().Unix()
	s.task.HeartbeatAt = &value
	return nil
}

func TestImageTaskServiceLifecycleAndOwnership(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, 10*time.Minute)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}

	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusQueued, created.Status)
	require.Equal(t, created.ID, created.TaskID)
	require.Equal(t, "image.generation.task", created.Object)
	require.Equal(t, time.Hour, store.ttl)
	require.Equal(t, owner.UserID, store.task.UserID)
	require.Equal(t, owner.APIKeyID, store.task.APIKeyID)

	_, err = svc.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 10}, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)

	result := json.RawMessage(`{"created":123,"data":[{"url":"https://example.test/image.png"}]}`)
	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, result))

	completed, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, completed.Status)
	require.Equal(t, http.StatusOK, completed.HTTPStatus)
	require.Equal(t, "https://example.test/image.png", completed.ImageURL)
	require.JSONEq(t, string(result), string(completed.Result))
	require.NotNil(t, completed.CompletedAt)
}

func TestImageTaskServiceInvalidResultBecomesFailed(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	created, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.NoError(t, err)

	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, json.RawMessage(`not-json`)))
	got, err := svc.Get(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2}, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusBadGateway, got.HTTPStatus)
	require.Contains(t, string(got.Error), "non-JSON")
}

func TestImageTaskServiceMapsStoreFailures(t *testing.T) {
	store := &imageTaskMemoryStore{saveErr: errors.New("redis down")}
	svc := NewImageTaskService(store)

	_, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
}

func TestImageTaskServiceDoesNotResumeProcessingTask(t *testing.T) {
	now := time.Now().UTC().Unix()
	store := &imageTaskMemoryStore{task: &ImageTaskRecord{
		ID:          "imgtask_processing",
		Status:      ImageTaskStatusProcessing,
		StartedAt:   &now,
		HeartbeatAt: &now,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)

	err := svc.MarkProcessing(context.Background(), store.task.ID)

	require.ErrorContains(t, err, "cannot be safely resumed")
	require.Equal(t, ImageTaskStatusProcessing, store.task.Status)
}

func TestImageTaskServiceDoesNotRestartTerminalTask(t *testing.T) {
	for _, status := range []string{ImageTaskStatusCompleted, ImageTaskStatusFailed} {
		t.Run(status, func(t *testing.T) {
			store := &imageTaskMemoryStore{task: &ImageTaskRecord{
				ID:        "imgtask_terminal",
				Status:    status,
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			}}
			svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)

			err := svc.MarkProcessing(context.Background(), store.task.ID)

			require.ErrorIs(t, err, ErrImageTaskAlreadyTerminal)
			require.Equal(t, status, store.task.Status)
		})
	}
}

func TestImageTaskServicePersistsProcessingHeartbeat(t *testing.T) {
	startedAt := time.Now().UTC().Add(-time.Minute).Unix()
	store := &imageTaskMemoryStore{task: &ImageTaskRecord{
		ID:          "imgtask_heartbeat",
		Status:      ImageTaskStatusProcessing,
		StartedAt:   &startedAt,
		HeartbeatAt: &startedAt,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)

	require.NoError(t, svc.Heartbeat(context.Background(), store.task.ID))

	require.NotNil(t, store.task.HeartbeatAt)
	require.Greater(t, *store.task.HeartbeatAt, startedAt)
}

func TestImageTaskServiceTerminalStateIsImmutable(t *testing.T) {
	now := time.Now().UTC().Unix()
	completedResult := json.RawMessage(`{"data":[{"url":"https://example.test/image.png"}]}`)
	store := &imageTaskMemoryStore{task: &ImageTaskRecord{
		ID:          "imgtask_completed",
		Status:      ImageTaskStatusCompleted,
		HTTPStatus:  http.StatusOK,
		Result:      completedResult,
		CompletedAt: &now,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)

	require.NoError(t, svc.Fail(
		context.Background(),
		store.task.ID,
		http.StatusInternalServerError,
		json.RawMessage(`{"type":"api_error","message":"late failure"}`),
	))

	require.Equal(t, ImageTaskStatusCompleted, store.task.Status)
	require.Equal(t, http.StatusOK, store.task.HTTPStatus)
	require.JSONEq(t, string(completedResult), string(store.task.Result))
	require.Empty(t, store.task.Error)
}

func TestImageTaskServiceTerminalTransitionClearsRequestEnvelope(t *testing.T) {
	now := time.Now().UTC().Unix()
	store := &imageTaskMemoryStore{task: &ImageTaskRecord{
		ID:          "imgtask_clear_request",
		Status:      ImageTaskStatusProcessing,
		Request:     "encrypted prompt and uploads",
		RequestHash: "hash",
		CreatedAt:   now - 60,
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)

	require.NoError(t, svc.Complete(
		context.Background(),
		store.task.ID,
		http.StatusOK,
		json.RawMessage(`{"data":[{"url":"https://example.test/image.png"}]}`),
	))

	require.Equal(t, ImageTaskStatusCompleted, store.task.Status)
	require.Empty(t, store.task.Request)
}

func TestImageTaskServiceConcurrentTerminalTransitionsUseCAS(t *testing.T) {
	now := time.Now().UTC().Unix()
	store := &imageTaskMemoryStore{task: &ImageTaskRecord{
		ID:        "imgtask_terminal_cas",
		Status:    ImageTaskStatusProcessing,
		CreatedAt: now - 60,
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_ = svc.Complete(context.Background(), store.task.ID, http.StatusOK, json.RawMessage(`{"data":[]}`))
	}()
	go func() {
		defer wg.Done()
		<-start
		_ = svc.Fail(context.Background(), store.task.ID, http.StatusInternalServerError, json.RawMessage(`{"type":"api_error"}`))
	}()
	close(start)
	wg.Wait()

	require.Contains(t, []string{ImageTaskStatusCompleted, ImageTaskStatusFailed}, store.task.Status)
	if store.task.Status == ImageTaskStatusCompleted {
		require.Empty(t, store.task.Error)
	} else {
		require.Empty(t, store.task.Result)
	}
}

func TestImageTaskServiceRollsBackAssetsWhenFencedTerminalWriteDoesNotCommit(t *testing.T) {
	for _, tt := range []struct {
		name    string
		saved   bool
		saveErr error
		wantErr error
	}{
		{name: "lease_lost", saveErr: ErrImageTaskLeaseLost, wantErr: ErrImageTaskLeaseLost},
		{name: "terminal_cas_lost", saved: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().UTC().Unix()
			store := &imageTaskMemoryStore{task: &ImageTaskRecord{
				ID:        "imgtask_asset_rollback_" + tt.name,
				Status:    ImageTaskStatusProcessing,
				CreatedAt: now - 60,
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			}}
			storage := &rollbackImageStorage{}
			uploader := NewImageResultUploader(storage, "images/", 0, nil)
			svc := NewImageTaskServiceWithUploader(store, uploader, time.Hour, time.Minute)
			lock := &fencedImageTaskTestLock{saved: tt.saved, saveErr: tt.saveErr}
			ctx := withImageTaskJobLock(context.Background(), lock)
			result := json.RawMessage(`{"data":[{"b64_json":"iVBORw0KGgpmYWtlLXBuZy1wYXlsb2Fk"}]}`)

			err := svc.Complete(ctx, store.task.ID, http.StatusOK, result)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, storage.saved, 1)
			require.Equal(t, []string{"images/" + store.task.ID + "-0.png"}, storage.deleted)
			require.Equal(t, ImageTaskStatusProcessing, store.task.Status)
		})
	}
}

type fencedImageTaskTestLock struct {
	saved   bool
	saveErr error
}

func (*fencedImageTaskTestLock) Release(context.Context) error                { return nil }
func (*fencedImageTaskTestLock) Refresh(context.Context, time.Duration) error { return nil }
func (l *fencedImageTaskTestLock) SaveIfStatus(
	context.Context,
	*ImageTaskRecord,
	string,
	time.Duration,
) (bool, error) {
	return l.saved, l.saveErr
}
func (*fencedImageTaskTestLock) Ack(context.Context) error     { return nil }
func (*fencedImageTaskTestLock) Requeue(context.Context) error { return nil }
