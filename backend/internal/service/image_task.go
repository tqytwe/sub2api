package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	ImageTaskStatusQueued     = "queued"
	ImageTaskStatusProcessing = "processing"
	ImageTaskStatusCompleted  = "completed"
	ImageTaskStatusFailed     = "failed"

	defaultImageTaskTTL              = 24 * time.Hour
	defaultImageTaskExecutionTimeout = 30 * time.Minute
)

var (
	ErrImageTaskNotFound              = infraerrors.New(http.StatusNotFound, "IMAGE_TASK_NOT_FOUND", "image task not found")
	ErrImageTaskForbidden             = infraerrors.New(http.StatusForbidden, "IMAGE_TASK_FORBIDDEN", "image task does not belong to this API key")
	ErrImageTaskUnavailable           = infraerrors.New(http.StatusServiceUnavailable, "IMAGE_TASK_UNAVAILABLE", "image task storage is unavailable")
	ErrImageTaskNotReady              = infraerrors.New(http.StatusServiceUnavailable, "IMAGE_ASYNC_NOT_READY", "asynchronous image runtime is not ready")
	ErrImageTaskIdempotency           = infraerrors.New(http.StatusConflict, "IMAGE_TASK_IDEMPOTENCY_CONFLICT", "Idempotency-Key was already used with a different request")
	ErrImageTaskIdempotencyKeyInvalid = infraerrors.New(http.StatusBadRequest, "IMAGE_TASK_IDEMPOTENCY_KEY_INVALID", "Idempotency-Key must be at most 255 bytes")
	ErrImageTaskQueueEmpty            = errors.New("image task queue is empty")
	ErrImageTaskUnsafeResume          = errors.New("processing image task cannot be safely resumed")
	ErrImageTaskAlreadyTerminal       = errors.New("image task is already terminal")
	ErrImageTaskLeaseLost             = errors.New("image task lease was lost")
)

// ImageTaskRecord is the private Redis representation of an asynchronous image
// request. Ownership fields are intentionally omitted from the public view.
type ImageTaskRecord struct {
	ID          string          `json:"id"`
	UserID      int64           `json:"user_id"`
	APIKeyID    int64           `json:"api_key_id"`
	Status      string          `json:"status"`
	HTTPStatus  int             `json:"http_status,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       json.RawMessage `json:"error,omitempty"`
	Platform    string          `json:"platform,omitempty"`
	RequestHash string          `json:"request_hash,omitempty"`
	Request     string          `json:"request_envelope_encrypted,omitempty"`
	CreatedAt   int64           `json:"created_at"`
	QueuedAt    *int64          `json:"queued_at,omitempty"`
	StartedAt   *int64          `json:"started_at,omitempty"`
	HeartbeatAt *int64          `json:"heartbeat_at,omitempty"`
	CompletedAt *int64          `json:"completed_at,omitempty"`
	ExpiresAt   int64           `json:"expires_at"`
}

// ImageTask is the API-safe task representation returned to callers.
type ImageTask struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	Object      string          `json:"object"`
	Status      string          `json:"status"`
	HTTPStatus  int             `json:"http_status,omitempty"`
	ImageURL    string          `json:"image_url,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       json.RawMessage `json:"error,omitempty"`
	CreatedAt   int64           `json:"created_at"`
	CompletedAt *int64          `json:"completed_at,omitempty"`
	ExpiresAt   int64           `json:"expires_at"`
}

type ImageTaskOwner struct {
	UserID   int64
	APIKeyID int64
}

type ImageTaskRequestEnvelope struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        []byte            `json:"body"`
}

type ImageTaskSubmission struct {
	Owner          ImageTaskOwner
	Platform       string
	Envelope       ImageTaskRequestEnvelope
	IdempotencyKey string
}

type ImageTaskStore interface {
	Save(ctx context.Context, task *ImageTaskRecord, ttl time.Duration) error
	SaveIfStatus(ctx context.Context, task *ImageTaskRecord, expectedStatus string, ttl time.Duration) (bool, error)
	Get(ctx context.Context, id string) (*ImageTaskRecord, error)
	TouchHeartbeat(ctx context.Context, id string, heartbeatAt time.Time) error
}

type ImageTaskRuntimeTask struct {
	ID        string
	Status    string
	CreatedAt time.Time
}

type ImageTaskQueueStats struct {
	Ready      int64
	Active     int64
	OldestTask *ImageTaskRuntimeTask
}

type ImageTaskJobLock interface {
	Release(ctx context.Context) error
	Refresh(ctx context.Context, ttl time.Duration) error
	SaveIfStatus(ctx context.Context, task *ImageTaskRecord, expectedStatus string, ttl time.Duration) (bool, error)
	Ack(ctx context.Context) error
	Requeue(ctx context.Context) error
}

type ImageTaskQueue interface {
	Submit(ctx context.Context, task *ImageTaskRecord, ttl time.Duration, idempotencyKey string) (taskID string, created bool, err error)
	Reserve(ctx context.Context, blockTimeout time.Duration) (string, error)
	Ack(ctx context.Context, taskID string) error
	Requeue(ctx context.Context, taskID string) error
	Heartbeat(ctx context.Context, taskID string) error
	RecoverStaleActive(ctx context.Context, staleAfter time.Duration, limit int) (int, error)
	TryAcquireJobLock(ctx context.Context, taskID string, ttl time.Duration) (ImageTaskJobLock, bool, error)
	Ping(ctx context.Context) error
	Stats(ctx context.Context) (ImageTaskQueueStats, error)
}

type ImageTaskLegacyRecovery interface {
	FailUnrecoverableProcessing(ctx context.Context, before time.Time, limit int) (int, error)
}

type imageTaskJobLockContextKey struct{}

func withImageTaskJobLock(ctx context.Context, lock ImageTaskJobLock) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, imageTaskJobLockContextKey{}, lock)
}

func imageTaskJobLockFromContext(ctx context.Context) ImageTaskJobLock {
	if ctx == nil {
		return nil
	}
	lock, _ := ctx.Value(imageTaskJobLockContextKey{}).(ImageTaskJobLock)
	return lock
}

type ImageTaskRuntimeSnapshot struct {
	APIEnabled    bool
	QueueEnabled  bool
	StorageReady  bool
	RedisReady    bool
	WorkerRunning bool
	Ready         bool
	Queue         ImageTaskQueueStats
	LastError     string
	LastErrorAt   *time.Time
}

type ImageTaskRuntimeState struct {
	queue        ImageTaskQueue
	apiEnabled   bool
	queueEnabled bool
	storageReady bool

	mu            sync.RWMutex
	workerRunning bool
	lastError     string
	lastErrorAt   *time.Time
}

func NewImageTaskRuntimeState(queue ImageTaskQueue, apiEnabled, queueEnabled, storageReady bool) *ImageTaskRuntimeState {
	return &ImageTaskRuntimeState{
		queue:        queue,
		apiEnabled:   apiEnabled,
		queueEnabled: queueEnabled,
		storageReady: storageReady,
	}
}

func (s *ImageTaskRuntimeState) SetWorkerRunning(running bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.workerRunning = running
	s.mu.Unlock()
}

func (s *ImageTaskRuntimeState) RecordError(err error) {
	if s == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.lastError = err.Error()
	s.lastErrorAt = &now
	s.mu.Unlock()
}

func (s *ImageTaskRuntimeState) Snapshot(ctx context.Context) ImageTaskRuntimeSnapshot {
	if s == nil {
		return ImageTaskRuntimeSnapshot{}
	}
	s.mu.RLock()
	snapshot := ImageTaskRuntimeSnapshot{
		APIEnabled:    s.apiEnabled,
		QueueEnabled:  s.queueEnabled,
		StorageReady:  s.storageReady,
		WorkerRunning: s.workerRunning,
		LastError:     s.lastError,
		LastErrorAt:   s.lastErrorAt,
	}
	s.mu.RUnlock()
	if s.queue != nil && s.queueEnabled {
		if err := s.queue.Ping(ctx); err == nil {
			snapshot.RedisReady = true
			stats, statsErr := s.queue.Stats(ctx)
			if statsErr != nil {
				s.RecordError(statsErr)
				now := time.Now().UTC()
				snapshot.LastError = statsErr.Error()
				snapshot.LastErrorAt = &now
				snapshot.RedisReady = false
			} else {
				snapshot.Queue = stats
			}
		} else {
			s.RecordError(err)
			now := time.Now().UTC()
			snapshot.LastError = err.Error()
			snapshot.LastErrorAt = &now
		}
	}
	snapshot.Ready = snapshot.APIEnabled && snapshot.QueueEnabled && snapshot.StorageReady && snapshot.RedisReady && snapshot.WorkerRunning
	return snapshot
}

// ImageStorageResolver reports the currently effective object-storage binding.
// It exists so the async image feature can be switched on and off from the admin
// UI without a restart: the wiring below is fixed at startup, but the answer to
// "is object storage configured right now" is re-read (and cached) per call.
type ImageStorageResolver func() (uploader *ImageResultUploader, enabled bool)

type ImageTaskService struct {
	store            ImageTaskStore
	queue            ImageTaskQueue
	uploader         *ImageResultUploader
	encryptor        SecretEncryptor
	runtime          *ImageTaskRuntimeState
	enabled          bool
	resolve          ImageStorageResolver
	ttl              time.Duration
	executionTimeout time.Duration
}

func NewImageTaskService(store ImageTaskStore) *ImageTaskService {
	return NewImageTaskServiceWithOptions(store, defaultImageTaskTTL, defaultImageTaskExecutionTimeout)
}

func NewImageTaskServiceWithOptions(store ImageTaskStore, ttl, executionTimeout time.Duration) *ImageTaskService {
	if ttl <= 0 {
		ttl = defaultImageTaskTTL
	}
	if executionTimeout <= 0 {
		executionTimeout = defaultImageTaskExecutionTimeout
	}
	return &ImageTaskService{store: store, ttl: ttl, executionTimeout: executionTimeout}
}

// NewImageTaskServiceWithUploader 构造一个已启用的图片任务服务：结果会先经 uploader
// 转存到对象存储再落 Redis。uploader 为 nil 时不做转存（仅用于测试）。
func NewImageTaskServiceWithUploader(store ImageTaskStore, uploader *ImageResultUploader, ttl, executionTimeout time.Duration) *ImageTaskService {
	s := NewImageTaskServiceWithOptions(store, ttl, executionTimeout)
	s.uploader = uploader
	s.enabled = true
	return s
}

func NewQueuedImageTaskService(
	store ImageTaskStore,
	queue ImageTaskQueue,
	uploader *ImageResultUploader,
	encryptor SecretEncryptor,
	runtime *ImageTaskRuntimeState,
	ttl, executionTimeout time.Duration,
) *ImageTaskService {
	s := NewImageTaskServiceWithOptions(store, ttl, executionTimeout)
	s.queue = queue
	s.uploader = uploader
	s.encryptor = encryptor
	s.runtime = runtime
	s.enabled = true
	return s
}

// NewImageTaskServiceWithResolver 构造一个由 resolver 决定启用状态的服务：
// 开关与凭证来自后台设置，保存后立即生效，无需重启。
func NewImageTaskServiceWithResolver(store ImageTaskStore, resolve ImageStorageResolver, ttl, executionTimeout time.Duration) *ImageTaskService {
	s := NewImageTaskServiceWithOptions(store, ttl, executionTimeout)
	s.resolve = resolve
	return s
}

func NewQueuedImageTaskServiceWithResolver(
	store ImageTaskStore,
	queue ImageTaskQueue,
	resolve ImageStorageResolver,
	encryptor SecretEncryptor,
	runtime *ImageTaskRuntimeState,
	ttl, executionTimeout time.Duration,
) *ImageTaskService {
	s := NewImageTaskServiceWithResolver(store, resolve, ttl, executionTimeout)
	s.queue = queue
	s.encryptor = encryptor
	s.runtime = runtime
	return s
}

// current 返回当前生效的 uploader 与启用状态。
// 注入了 resolver 时以 resolver 为准（后台设置可热切换），否则回落到构造时固定的值。
func (s *ImageTaskService) current() (*ImageResultUploader, bool) {
	if s == nil {
		return nil, false
	}
	if s.resolve != nil {
		return s.resolve()
	}
	return s.uploader, s.enabled
}

// Enabled 表示异步图片任务功能是否可用（总开关 + 凭证齐全）。
// 关闭时 handler 直接返回 404，不创建任务、不写 Redis。
func (s *ImageTaskService) Enabled() bool {
	if s == nil || s.store == nil {
		return false
	}
	_, enabled := s.current()
	return enabled
}

// Pollable 表示已创建的任务能否被查询。
// 比 Enabled 弱：只要 store 可用即可，从而在功能被关掉后仍能取回进行中的任务结果。
func (s *ImageTaskService) Pollable() bool {
	return s != nil && s.store != nil
}

func (s *ImageTaskService) SubmissionReady(ctx context.Context) bool {
	if s == nil || !s.Enabled() || s.queue == nil || s.encryptor == nil || s.runtime == nil {
		return false
	}
	return s.RuntimeSnapshot(ctx).Ready
}

func (s *ImageTaskService) RuntimeSnapshot(ctx context.Context) ImageTaskRuntimeSnapshot {
	if s == nil || s.runtime == nil {
		return ImageTaskRuntimeSnapshot{}
	}
	snapshot := s.runtime.Snapshot(ctx)
	if s.resolve != nil {
		_, storageReady := s.current()
		snapshot.StorageReady = storageReady
		snapshot.Ready = snapshot.APIEnabled && snapshot.QueueEnabled && snapshot.StorageReady && snapshot.RedisReady && snapshot.WorkerRunning
	}
	return snapshot
}

func (s *ImageTaskService) ExecutionTimeout() time.Duration {
	if s == nil || s.executionTimeout <= 0 {
		return defaultImageTaskExecutionTimeout
	}
	return s.executionTimeout
}

func (s *ImageTaskService) Create(ctx context.Context, owner ImageTaskOwner) (*ImageTask, error) {
	if s == nil || s.store == nil {
		return nil, ErrImageTaskUnavailable
	}
	now := time.Now().UTC()
	task := &ImageTaskRecord{
		ID:        "imgtask_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:    owner.UserID,
		APIKeyID:  owner.APIKeyID,
		Status:    ImageTaskStatusQueued,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(s.ttl).Unix(),
	}
	queuedAt := now.Unix()
	task.QueuedAt = &queuedAt
	if err := s.store.Save(ctx, task, s.ttl); err != nil {
		return nil, ErrImageTaskUnavailable.WithCause(err)
	}
	return imageTaskToPublic(task), nil
}

func (s *ImageTaskService) Submit(ctx context.Context, submission ImageTaskSubmission) (*ImageTask, bool, error) {
	if s == nil || s.store == nil || s.queue == nil || s.encryptor == nil {
		return nil, false, ErrImageTaskUnavailable
	}
	if len(strings.TrimSpace(submission.IdempotencyKey)) > 255 {
		return nil, false, ErrImageTaskIdempotencyKeyInvalid
	}
	if !s.SubmissionReady(ctx) {
		return nil, false, ErrImageTaskNotReady
	}
	rawEnvelope, err := json.Marshal(submission.Envelope)
	if err != nil {
		return nil, false, ErrImageTaskUnavailable.WithCause(err)
	}
	encrypted, err := s.encryptor.Encrypt(string(rawEnvelope))
	if err != nil {
		return nil, false, ErrImageTaskUnavailable.WithCause(err)
	}
	hashInput, err := json.Marshal(struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		ContentType string `json:"content_type"`
		Body        []byte `json:"body"`
	}{
		Method:      submission.Envelope.Method,
		Path:        submission.Envelope.Path,
		ContentType: submission.Envelope.ContentType,
		Body:        submission.Envelope.Body,
	})
	if err != nil {
		return nil, false, ErrImageTaskUnavailable.WithCause(err)
	}
	hash := sha256.Sum256(hashInput)
	requestHash := fmt.Sprintf("%x", hash[:])
	now := time.Now().UTC()
	queuedAt := now.Unix()
	task := &ImageTaskRecord{
		ID:          "imgtask_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:      submission.Owner.UserID,
		APIKeyID:    submission.Owner.APIKeyID,
		Status:      ImageTaskStatusQueued,
		Platform:    submission.Platform,
		RequestHash: requestHash,
		Request:     encrypted,
		CreatedAt:   now.Unix(),
		QueuedAt:    &queuedAt,
		ExpiresAt:   now.Add(s.ttl).Unix(),
	}
	idempotencyKey := imageTaskIdempotencyScope(submission.Owner, submission.IdempotencyKey)
	taskID, created, err := s.queue.Submit(ctx, task, s.ttl, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrImageTaskIdempotency) {
			return nil, false, ErrImageTaskIdempotency
		}
		return nil, false, ErrImageTaskUnavailable.WithCause(err)
	}
	if !created {
		task, err = s.store.Get(ctx, taskID)
		if err != nil {
			return nil, false, ErrImageTaskUnavailable.WithCause(err)
		}
	}
	return imageTaskToPublic(task), !created, nil
}

func imageTaskIdempotencyScope(owner ImageTaskOwner, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s", owner.UserID, owner.APIKeyID, key)
}

func (s *ImageTaskService) RequestEnvelope(ctx context.Context, id string) (*ImageTaskRecord, *ImageTaskRequestEnvelope, error) {
	if s == nil || s.store == nil || s.encryptor == nil {
		return nil, nil, ErrImageTaskUnavailable
	}
	task, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(task.Request) == "" {
		return task, nil, errors.New("image task request envelope is missing")
	}
	plaintext, err := s.encryptor.Decrypt(task.Request)
	if err != nil {
		return task, nil, fmt.Errorf("decrypt image task request envelope: %w", err)
	}
	var envelope ImageTaskRequestEnvelope
	if err := json.Unmarshal([]byte(plaintext), &envelope); err != nil {
		return task, nil, fmt.Errorf("decode image task request envelope: %w", err)
	}
	return task, &envelope, nil
}

func (s *ImageTaskService) MarkProcessing(ctx context.Context, id string) error {
	if s == nil || s.store == nil {
		return ErrImageTaskUnavailable
	}
	task, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if task.Status == ImageTaskStatusCompleted || task.Status == ImageTaskStatusFailed {
		return ErrImageTaskAlreadyTerminal
	}
	if task.Status == ImageTaskStatusProcessing {
		return ErrImageTaskUnsafeResume
	}
	now := time.Now().UTC().Unix()
	task.Status = ImageTaskStatusProcessing
	task.StartedAt = &now
	task.HeartbeatAt = &now
	saved, err := s.saveIfStatus(ctx, task, ImageTaskStatusQueued, time.Until(time.Unix(task.ExpiresAt, 0)))
	if err != nil {
		return ErrImageTaskUnavailable.WithCause(err)
	}
	if saved {
		return nil
	}
	current, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status == ImageTaskStatusCompleted || current.Status == ImageTaskStatusFailed {
		return ErrImageTaskAlreadyTerminal
	}
	return ErrImageTaskUnsafeResume
}

func (s *ImageTaskService) Heartbeat(ctx context.Context, id string) error {
	if s == nil || s.store == nil {
		return ErrImageTaskUnavailable
	}
	if err := s.store.TouchHeartbeat(ctx, id, time.Now().UTC()); err != nil {
		if errors.Is(err, ErrImageTaskNotFound) {
			return ErrImageTaskNotFound
		}
		return ErrImageTaskUnavailable.WithCause(err)
	}
	return nil
}

func (s *ImageTaskService) Get(ctx context.Context, owner ImageTaskOwner, id string) (*ImageTask, error) {
	if s == nil || s.store == nil {
		return nil, ErrImageTaskUnavailable
	}
	task, err := s.store.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, ErrImageTaskNotFound) {
			return nil, ErrImageTaskNotFound
		}
		return nil, ErrImageTaskUnavailable.WithCause(err)
	}
	if task.UserID != owner.UserID || task.APIKeyID != owner.APIKeyID {
		// Do not reveal whether a random task ID exists for another caller.
		return nil, ErrImageTaskNotFound
	}
	return imageTaskToPublic(task), nil
}

func (s *ImageTaskService) Complete(ctx context.Context, id string, statusCode int, result json.RawMessage) error {
	if !json.Valid(result) {
		return s.Fail(ctx, id, http.StatusBadGateway, imageTaskErrorJSON("api_error", "upstream returned a non-JSON image response"))
	}
	var rollback func()
	if uploader, _ := s.current(); uploader != nil {
		rewritten, cleanup, err := uploader.RewriteWithRollback(ctx, id, result)
		if err != nil {
			// 转存失败不回退存 base64，避免大 blob 撑爆 Redis：直接把任务标记为失败。
			logger.L().Error("image_task.offload_failed", zap.String("task_id", id), zap.Error(err))
			return s.Fail(ctx, id, http.StatusBadGateway, imageTaskErrorJSON("api_error", "failed to store generated image result"))
		}
		result = rewritten
		rollback = cleanup
	}
	saved, err := s.finish(ctx, id, ImageTaskStatusCompleted, statusCode, result, nil)
	if (err != nil || !saved) && rollback != nil {
		rollback()
	}
	return err
}

func (s *ImageTaskService) Fail(ctx context.Context, id string, statusCode int, taskErr json.RawMessage) error {
	if !json.Valid(taskErr) {
		taskErr = imageTaskErrorJSON("api_error", "image generation failed")
	}
	_, err := s.finish(ctx, id, ImageTaskStatusFailed, statusCode, nil, taskErr)
	return err
}

func (s *ImageTaskService) finish(ctx context.Context, id, status string, statusCode int, result, taskErr json.RawMessage) (bool, error) {
	if s == nil || s.store == nil {
		return false, ErrImageTaskUnavailable
	}
	task, err := s.store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrImageTaskNotFound) {
			return false, ErrImageTaskNotFound
		}
		return false, ErrImageTaskUnavailable.WithCause(err)
	}
	if task.Status == ImageTaskStatusCompleted || task.Status == ImageTaskStatusFailed {
		return false, nil
	}
	previousStatus := task.Status
	now := time.Now().UTC()
	completedAt := now.Unix()
	task.Status = status
	task.HTTPStatus = statusCode
	task.Result = result
	task.Error = taskErr
	task.Request = ""
	task.CompletedAt = &completedAt
	task.HeartbeatAt = &completedAt
	task.ExpiresAt = now.Add(s.ttl).Unix()
	expectedStatus := ImageTaskStatusProcessing
	if s.queue == nil {
		expectedStatus = previousStatus
	}
	saved, err := s.saveIfStatus(ctx, task, expectedStatus, s.ttl)
	if err != nil {
		if errors.Is(err, ErrImageTaskLeaseLost) {
			return false, ErrImageTaskLeaseLost
		}
		return false, ErrImageTaskUnavailable.WithCause(err)
	}
	return saved, nil
}

func (s *ImageTaskService) saveIfStatus(
	ctx context.Context,
	task *ImageTaskRecord,
	expectedStatus string,
	ttl time.Duration,
) (bool, error) {
	if lock := imageTaskJobLockFromContext(ctx); lock != nil {
		return lock.SaveIfStatus(ctx, task, expectedStatus, ttl)
	}
	return s.store.SaveIfStatus(ctx, task, expectedStatus, ttl)
}

func imageTaskToPublic(task *ImageTaskRecord) *ImageTask {
	if task == nil {
		return nil
	}
	return &ImageTask{
		ID:          task.ID,
		TaskID:      task.ID,
		Object:      "image.generation.task",
		Status:      task.Status,
		HTTPStatus:  task.HTTPStatus,
		ImageURL:    firstImageTaskURL(task.Result),
		Result:      task.Result,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt,
		CompletedAt: task.CompletedAt,
		ExpiresAt:   task.ExpiresAt,
	}
}

func firstImageTaskURL(result json.RawMessage) string {
	if len(result) == 0 || !json.Valid(result) {
		return ""
	}
	var response struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if json.Unmarshal(result, &response) != nil || len(response.Data) == 0 {
		return ""
	}
	return strings.TrimSpace(response.Data[0].URL)
}

func imageTaskErrorJSON(errorType, message string) json.RawMessage {
	data, _ := json.Marshal(map[string]string{"type": errorType, "message": message})
	return data
}

func imageTaskErrorCodeJSON(errorType, code, message string) json.RawMessage {
	data, _ := json.Marshal(map[string]string{"type": errorType, "code": code, "message": message})
	return data
}
