package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type asyncImageZeroReader struct{}

func (asyncImageZeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type asyncImageMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*service.ImageTaskRecord
}

type asyncImagePlainEncryptor struct{}

func (asyncImagePlainEncryptor) Encrypt(value string) (string, error) {
	return "encrypted:" + value, nil
}
func (asyncImagePlainEncryptor) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "encrypted:"), nil
}

type asyncImageMemoryQueue struct {
	mu     sync.Mutex
	store  *asyncImageMemoryStore
	ready  []string
	active map[string]bool
	idem   map[string]struct {
		taskID string
		hash   string
	}
}

func (q *asyncImageMemoryQueue) Submit(_ context.Context, task *service.ImageTaskRecord, ttl time.Duration, idempotencyKey string) (string, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if idempotencyKey != "" {
		if existing, ok := q.idem[idempotencyKey]; ok {
			if existing.hash != task.RequestHash {
				return existing.taskID, false, service.ErrImageTaskIdempotency
			}
			return existing.taskID, false, nil
		}
		q.idem[idempotencyKey] = struct {
			taskID string
			hash   string
		}{taskID: task.ID, hash: task.RequestHash}
	}
	if err := q.store.Save(context.Background(), task, ttl); err != nil {
		return "", false, err
	}
	q.ready = append(q.ready, task.ID)
	return task.ID, true, nil
}

func (q *asyncImageMemoryQueue) Reserve(context.Context, time.Duration) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.ready) == 0 {
		return "", service.ErrImageTaskQueueEmpty
	}
	taskID := q.ready[0]
	q.ready = q.ready[1:]
	q.active[taskID] = true
	return taskID, nil
}

func (q *asyncImageMemoryQueue) Ack(_ context.Context, taskID string) error {
	q.mu.Lock()
	delete(q.active, taskID)
	q.mu.Unlock()
	return nil
}

func (q *asyncImageMemoryQueue) Requeue(_ context.Context, taskID string) error {
	q.mu.Lock()
	delete(q.active, taskID)
	q.ready = append(q.ready, taskID)
	q.mu.Unlock()
	return nil
}

func (q *asyncImageMemoryQueue) Heartbeat(context.Context, string) error { return nil }
func (q *asyncImageMemoryQueue) RecoverStaleActive(context.Context, time.Duration, int) (int, error) {
	return 0, nil
}
func (q *asyncImageMemoryQueue) TryAcquireJobLock(_ context.Context, taskID string, _ time.Duration) (service.ImageTaskJobLock, bool, error) {
	return asyncImageMemoryLock{queue: q, taskID: taskID}, true, nil
}
func (q *asyncImageMemoryQueue) Ping(context.Context) error { return nil }
func (q *asyncImageMemoryQueue) Stats(context.Context) (service.ImageTaskQueueStats, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return service.ImageTaskQueueStats{Ready: int64(len(q.ready)), Active: int64(len(q.active))}, nil
}

type asyncImageMemoryLock struct {
	queue  *asyncImageMemoryQueue
	taskID string
}

func (asyncImageMemoryLock) Release(context.Context) error                { return nil }
func (asyncImageMemoryLock) Refresh(context.Context, time.Duration) error { return nil }
func (l asyncImageMemoryLock) SaveIfStatus(
	ctx context.Context,
	task *service.ImageTaskRecord,
	expectedStatus string,
	ttl time.Duration,
) (bool, error) {
	if l.queue == nil || l.queue.store == nil || task == nil || task.ID != l.taskID {
		return false, service.ErrImageTaskLeaseLost
	}
	return l.queue.store.SaveIfStatus(ctx, task, expectedStatus, ttl)
}
func (l asyncImageMemoryLock) Ack(ctx context.Context) error {
	return l.queue.Ack(ctx, l.taskID)
}
func (l asyncImageMemoryLock) Requeue(ctx context.Context) error {
	return l.queue.Requeue(ctx, l.taskID)
}

func newAsyncImageQueuedTestService(store *asyncImageMemoryStore) (*service.ImageTaskService, *asyncImageMemoryQueue) {
	queue := &asyncImageMemoryQueue{
		store:  store,
		active: make(map[string]bool),
		idem: make(map[string]struct {
			taskID string
			hash   string
		}),
	}
	state := service.NewImageTaskRuntimeState(queue, true, true, true)
	state.SetWorkerRunning(true)
	tasks := service.NewQueuedImageTaskService(store, queue, nil, asyncImagePlainEncryptor{}, state, time.Hour, time.Minute)
	return tasks, queue
}

type asyncImageAPIKeyLoader struct {
	key *service.APIKey
	err error
}

func (l asyncImageAPIKeyLoader) GetByID(context.Context, int64) (*service.APIKey, error) {
	return l.key, l.err
}

type asyncImageSubscriptionLoader struct {
	subscription *service.UserSubscription
	err          error
}

func (l asyncImageSubscriptionLoader) GetActiveSubscription(context.Context, int64, int64) (*service.UserSubscription, error) {
	return l.subscription, l.err
}

func TestAsyncImageHandlerReloadRejectsExpiredAPIKey(t *testing.T) {
	expiredAt := time.Now().Add(-time.Minute)
	groupID := int64(3)
	h := &AsyncImageHandler{apiKeys: asyncImageAPIKeyLoader{key: &service.APIKey{
		ID:        9,
		UserID:    7,
		Status:    service.StatusAPIKeyActive,
		ExpiresAt: &expiredAt,
		User:      &service.User{ID: 7, Status: service.StatusActive},
		GroupID:   &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}}}

	_, err := h.reloadImageTaskAPIKey(context.Background(), &service.ImageTaskRecord{
		UserID:   7,
		APIKeyID: 9,
		Platform: service.PlatformOpenAI,
	})

	require.ErrorContains(t, err, "no longer eligible")
}

type asyncImageMemoryAssetStore struct {
	data        []byte
	contentType string
	err         error
}

type asyncImageResultMemoryStore struct {
	record *service.OpenAIImageResultRecord
}

func (s *asyncImageResultMemoryStore) Save(_ context.Context, record *service.OpenAIImageResultRecord, _ time.Duration) error {
	cloned := *record
	cloned.Assets = append([]service.OpenAIImageResultAsset(nil), record.Assets...)
	s.record = &cloned
	return nil
}

func (s *asyncImageResultMemoryStore) Get(_ context.Context, id string) (*service.OpenAIImageResultRecord, error) {
	if s.record == nil || s.record.ID != id {
		return nil, service.ErrOpenAIImageResultNotFound
	}
	cloned := *s.record
	cloned.Assets = append([]service.OpenAIImageResultAsset(nil), s.record.Assets...)
	return &cloned, nil
}

func (s *asyncImageMemoryAssetStore) Save(_ context.Context, _ string, _ string, _ []byte) (string, error) {
	return "", nil
}

func (s *asyncImageMemoryAssetStore) Open(_ context.Context, _ string) (io.ReadCloser, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return io.NopCloser(bytes.NewReader(s.data)), s.contentType, nil
}

func (s *asyncImageMemoryStore) Save(_ context.Context, task *service.ImageTaskRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	s.tasks[task.ID] = &copy
	return nil
}

func (s *asyncImageMemoryStore) Get(_ context.Context, id string) (*service.ImageTaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task := s.tasks[id]
	if task == nil {
		return nil, service.ErrImageTaskNotFound
	}
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	return &copy, nil
}

func (s *asyncImageMemoryStore) SaveIfStatus(_ context.Context, task *service.ImageTaskRecord, expectedStatus string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.tasks[task.ID]
	if current == nil {
		return false, service.ErrImageTaskNotFound
	}
	if current.Status != expectedStatus {
		return false, nil
	}
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	s.tasks[task.ID] = &copy
	return true, nil
}

func (s *asyncImageMemoryStore) TouchHeartbeat(_ context.Context, id string, heartbeatAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.tasks[id]
	if task == nil {
		return service.ErrImageTaskNotFound
	}
	if task.Status != service.ImageTaskStatusProcessing {
		return nil
	}
	value := heartbeatAt.UTC().Unix()
	task.HeartbeatAt = &value
	return nil
}

func TestAsyncImageHandlerSubmitAndPoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, _ := newAsyncImageQueuedTestService(store)
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "3", w.Header().Get("Retry-After"))

	var accepted struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		PollURL string `json:"poll_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.Equal(t, service.ImageTaskStatusQueued, accepted.Status)
	require.Equal(t, "/v1/images/tasks/"+accepted.TaskID, accepted.PollURL)
	require.Equal(t, accepted.PollURL, w.Header().Get("Location"))

	pollReq := httptest.NewRequest(http.MethodGet, accepted.PollURL, nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Equal(t, "no-store", pollWriter.Header().Get("Cache-Control"))
	require.Equal(t, "3", pollWriter.Header().Get("Retry-After"))
	require.Contains(t, pollWriter.Body.String(), `"status":"queued"`)

	require.NoError(t, tasks.MarkProcessing(context.Background(), accepted.TaskID))
	require.NoError(t, tasks.Complete(context.Background(), accepted.TaskID, http.StatusOK, json.RawMessage(`{"created":123,"data":[{"url":"https://example.test/image.png"}]}`)))
	pollWriter = httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Empty(t, pollWriter.Header().Get("Retry-After"))
	require.Contains(t, pollWriter.Body.String(), "https://example.test/image.png")
}

func TestAsyncImageHandlerStripsUTF8BOMBeforeHashingAndPersistingEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, _ := newAsyncImageQueuedTestService(store)
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)

	normalized := []byte(`{"model":"gpt-image-2","prompt":"cat"}`)
	body := append([]byte{0xef, 0xbb, 0xbf}, normalized...)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	taskID := gjson.GetBytes(recorder.Body.Bytes(), "task_id").String()
	_, envelope, err := tasks.RequestEnvelope(context.Background(), taskID)
	require.NoError(t, err)
	require.Equal(t, normalized, envelope.Body)
}

func TestAsyncImageHandlerIdempotencyReplayAndConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, queue := newAsyncImageQueuedTestService(store)
	h := NewAsyncImageHandler(tasks, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 9, UserID: 7, GroupID: &groupID,
			Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)

	submit := func(prompt string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2","prompt":"`+prompt+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "stable-request")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	first := submit("cat")
	require.Equal(t, http.StatusAccepted, first.Code)
	firstID := gjson.GetBytes(first.Body.Bytes(), "task_id").String()

	replay := submit("cat")
	require.Equal(t, http.StatusAccepted, replay.Code)
	require.Equal(t, "true", replay.Header().Get("Idempotent-Replayed"))
	require.Equal(t, firstID, gjson.GetBytes(replay.Body.Bytes(), "task_id").String())

	conflict := submit("dog")
	require.Equal(t, http.StatusConflict, conflict.Code)
	require.Equal(t, "IMAGE_TASK_IDEMPOTENCY_CONFLICT", gjson.GetBytes(conflict.Body.Bytes(), "error.code").String())

	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Ready)
}

func TestAsyncImageHandlerRejectsOversizedIdempotencyKeyBeforeQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, queue := newAsyncImageQueuedTestService(store)
	h := NewAsyncImageHandler(tasks, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 9, UserID: 7, GroupID: &groupID,
			Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", strings.Repeat("x", 256))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "IMAGE_TASK_IDEMPOTENCY_KEY_INVALID", gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
	stats, err := queue.Stats(context.Background())
	require.NoError(t, err)
	require.Zero(t, stats.Ready)
}

func TestAsyncImageHandlerPreservesImagesValidationErrorCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{
			name:     "blank_prompt",
			body:     `{"model":"gpt-image-2","prompt":"   "}`,
			wantCode: "IMAGE_PROMPT_REQUIRED",
		},
		{
			name:     "invalid_response_format",
			body:     `{"model":"gpt-image-2","prompt":"cat","response_format":"data_url"}`,
			wantCode: "IMAGE_RESPONSE_FORMAT_INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
			tasks, queue := newAsyncImageQueuedTestService(store)
			openAI := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}}
			h := NewAsyncImageHandler(tasks, openAI, nil)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				groupID := int64(3)
				c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
					ID: 9, UserID: 7, GroupID: &groupID,
					Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
				})
				c.Next()
			})
			router.POST("/v1/images/generations/async", h.Submit)

			req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, "invalid_request_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
			require.Equal(t, tt.wantCode, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
			stats, err := queue.Stats(context.Background())
			require.NoError(t, err)
			require.Zero(t, stats.Ready)
		})
	}
}

func TestAsyncImageWorkerForcesBase64BeforeGatewayExecution(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, _ := newAsyncImageQueuedTestService(store)
	groupID := int64(3)
	apiKey := &service.APIKey{
		ID: 9, UserID: 7, Status: service.StatusAPIKeyActive,
		User:    &service.User{ID: 7, Status: service.StatusActive, Role: "user", Concurrency: 2},
		GroupID: &groupID,
		Group: &service.Group{
			ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true,
		},
	}
	h := NewAsyncImageHandler(tasks, nil, nil)
	h.apiKeys = asyncImageAPIKeyLoader{key: apiKey}
	executed := false
	submittedTaskID := ""
	h.execute = func(_ string, c *gin.Context) {
		executed = true
		clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		require.Equal(t, submittedTaskID, clientRequestID)
		raw, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, "b64_json", gjson.GetBytes(raw, "response_format").String())
		require.False(t, gjson.GetBytes(raw, "stream").Bool())
		c.JSON(http.StatusOK, gin.H{"created": 1, "data": []gin.H{{"b64_json": "aW1hZ2U="}}})
	}

	task, _, err := tasks.Submit(context.Background(), service.ImageTaskSubmission{
		Owner:    service.ImageTaskOwner{UserID: 7, APIKeyID: 9},
		Platform: service.PlatformOpenAI,
		Envelope: service.ImageTaskRequestEnvelope{
			Method:      http.MethodPost,
			Path:        "/v1/images/generations",
			ContentType: "application/json",
			Body:        []byte(`{"model":"gpt-image-2","prompt":"cat","response_format":"url"}`),
		},
	})
	require.NoError(t, err)
	submittedTaskID = task.ID
	require.NoError(t, tasks.MarkProcessing(context.Background(), task.ID))
	require.NoError(t, h.ProcessImageTask(context.Background(), task.ID))
	require.True(t, executed)

	completed, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusCompleted, completed.Status)
	require.Equal(t, "aW1hZ2U=", gjson.GetBytes(completed.Result, "data.0.b64_json").String())
}

func TestAsyncImageWorkerFailsClosedWhenSubscriptionCannotBeRestored(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, _ := newAsyncImageQueuedTestService(store)
	groupID := int64(3)
	apiKey := &service.APIKey{
		ID: 9, UserID: 7, Status: service.StatusAPIKeyActive,
		User:    &service.User{ID: 7, Status: service.StatusActive, Role: "user", Concurrency: 2},
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			SubscriptionType:     service.SubscriptionTypeSubscription,
			AllowImageGeneration: true,
		},
	}
	h := NewAsyncImageHandler(tasks, nil, nil)
	h.apiKeys = asyncImageAPIKeyLoader{key: apiKey}
	h.subscriptions = asyncImageSubscriptionLoader{err: errors.New("subscription database unavailable")}
	executed := false
	h.execute = func(string, *gin.Context) {
		executed = true
	}

	task, _, err := tasks.Submit(context.Background(), service.ImageTaskSubmission{
		Owner:    service.ImageTaskOwner{UserID: 7, APIKeyID: 9},
		Platform: service.PlatformOpenAI,
		Envelope: service.ImageTaskRequestEnvelope{
			Method:      http.MethodPost,
			Path:        "/v1/images/generations",
			ContentType: "application/json",
			Body:        []byte(`{"model":"gpt-image-2","prompt":"cat"}`),
		},
	})
	require.NoError(t, err)
	require.NoError(t, tasks.MarkProcessing(context.Background(), task.ID))

	require.NoError(t, h.ProcessImageTask(context.Background(), task.ID))

	require.False(t, executed)
	failed, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageTaskStatusFailed, failed.Status)
	require.Equal(t, "IMAGE_TASK_SUBSCRIPTION_UNAVAILABLE", gjson.GetBytes(failed.Error, "code").String())
}

// When result storage is not configured the feature is fully disabled: the
// endpoints must return 404 without creating a task or writing to Redis.
func TestAsyncImageHandlerDisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithOptions(store, time.Hour, time.Minute) // enabled == false
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not enabled")

	pollReq := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/imgtask_missing", nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusNotFound, pollWriter.Code)

	// No task was created / persisted.
	require.Empty(t, store.tasks)
}

func TestAsyncImageHandlerGetAssetRequiresTaskOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	assetStore := &asyncImageMemoryAssetStore{data: []byte("image-bytes"), contentType: "image/png"}
	h := NewAsyncImageHandler(tasks, nil, assetStore)

	owner := service.ImageTaskOwner{UserID: 7, APIKeyID: 9}
	task, err := tasks.Create(context.Background(), owner)
	require.NoError(t, err)
	require.NoError(t, tasks.Complete(context.Background(), task.ID, http.StatusOK, json.RawMessage(`{"data":[{"url":"/v1/images/task-assets/images/`+task.ID+`-0.png"}]}`)))

	router := gin.New()
	apiKeyID := int64(9)
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      apiKeyID,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.GET("/v1/images/task-assets/*filepath", h.GetAsset)

	req := httptest.NewRequest(http.MethodGet, "/v1/images/task-assets/images/"+task.ID+"-0.png", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, "image-bytes", w.Body.String())

	apiKeyID = 10
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAsyncImageHandlerGetResultRequiresAPIKeyOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageResultMemoryStore{record: &service.OpenAIImageResultRecord{
		ID:        "imgres_owned",
		UserID:    7,
		APIKeyID:  9,
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		Assets: []service.OpenAIImageResultAsset{{
			Key:         "images/results/imgres_owned-0.png",
			ContentType: "image/png",
		}},
	}}
	assets := &asyncImageMemoryAssetStore{data: []byte("private-image"), contentType: "image/png"}
	results := service.NewOpenAIImageResultService(store, assets, assets, "images/", time.Hour)
	h := &AsyncImageHandler{imageResults: results}

	router := gin.New()
	apiKeyID := int64(9)
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: apiKeyID, UserID: 7})
		c.Next()
	})
	router.GET("/v1/images/results/:result_id/:index", h.GetResult)

	req := httptest.NewRequest(http.MethodGet, "/v1/images/results/imgres_owned/0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, "private-image", w.Body.String())

	apiKeyID = 10
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAsyncImageHandlerMultipartPartTooLargeReturnsOpenAICompatible413(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
			tasks, _ := newAsyncImageQueuedTestService(store)
			openAI := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}}
			h := NewAsyncImageHandler(tasks, openAI, nil)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				groupID := int64(3)
				c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
					ID:      9,
					UserID:  7,
					GroupID: &groupID,
					Group: &service.Group{
						ID:                   groupID,
						Platform:             platform,
						AllowImageGeneration: true,
					},
				})
				c.Next()
			})
			router.POST("/v1/images/edits/async", h.Submit)

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			model := "gpt-image-2"
			if platform == service.PlatformGrok {
				model = "grok-imagine-edit"
			}
			require.NoError(t, writer.WriteField("model", model))
			part, err := writer.CreateFormFile("image", "source.png")
			require.NoError(t, err)
			_, err = io.CopyN(part, asyncImageZeroReader{}, (20<<20)+1)
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			req := httptest.NewRequest(http.MethodPost, "/v1/images/edits/async", bytes.NewReader(body.Bytes()))
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			require.Equal(t, "invalid_request_error", gjson.GetBytes(w.Body.Bytes(), "error.type").String())
			require.Equal(t, "Multipart field image exceeds 20 MiB limit", gjson.GetBytes(w.Body.Bytes(), "error.message").String())
			require.Empty(t, store.tasks)
		})
	}
}

func TestAsyncImageHandlerGrokMalformedMultipartJSONBodyReturnsOpenAICompatible400(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks, _ := newAsyncImageQueuedTestService(store)
	openAI := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}}
	h := NewAsyncImageHandler(tasks, openAI, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group: &service.Group{
				ID:                   groupID,
				Platform:             service.PlatformGrok,
				AllowImageGeneration: true,
			},
		})
		c.Next()
	})
	router.POST("/v1/images/edits/async", h.Submit)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/images/edits/async",
		strings.NewReader(`{"model":"grok-imagine-edit","prompt":"test"}`),
	)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(w.Body.Bytes(), "error.type").String())
	require.Equal(t, "multipart boundary is required", gjson.GetBytes(w.Body.Bytes(), "error.message").String())
	require.Empty(t, store.tasks)
}
