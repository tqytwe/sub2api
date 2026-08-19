package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// MobileVideoJobState is private worker state. It must not be confused with
// the public MobileTaskStatus projection.
type MobileVideoJobState string

const (
	MobileVideoJobStateQueued     MobileVideoJobState = "queued"
	MobileVideoJobStateSubmitting MobileVideoJobState = "submitting"
	MobileVideoJobStatePolling    MobileVideoJobState = "polling"
	MobileVideoJobStateCompleted  MobileVideoJobState = "completed"
	MobileVideoJobStateFailed     MobileVideoJobState = "failed"
	MobileVideoJobStateCancelled  MobileVideoJobState = "cancelled"
)

var (
	ErrMobileVideoJobNotFound         = errors.New("mobile video job not found")
	ErrMobileVideoJobStoreUnavailable = errors.New("mobile video job store is unavailable")
	ErrMobileVideoJobProviderID       = errors.New("mobile video provider request id is missing")
	ErrMobileVideoJobArtifact         = errors.New("mobile video provider returned no artifact")
)

// MobileVideoJob is deliberately private to the executor. Prompt and
// references never enter MobileTask.Resource or MobileTask.Artifacts until
// the worker has produced a server-owned artifact.
type MobileVideoJob struct {
	TaskID              string
	UserID              int64
	GroupID             int64
	Model               string
	Prompt              string
	Resolution          string
	Ratio               string
	DurationSeconds     int
	GenerateAudio       bool
	Watermark           bool
	ReferenceAssetIDs   []string
	Provider            string
	ProviderRequestID   string
	ProviderStatus      string
	State               MobileVideoJobState
	AttemptCount        int
	NextPollAt          time.Time
	LeaseOwner          string
	LeaseExpiresAt      *time.Time
	ArtifactStorageKey  string
	ArtifactURL         string
	ArtifactContentType string
	ArtifactByteSize    int64
	LastError           *MobileTaskError
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// CreateInput reconstructs only the private request fields needed to create a
// retry row. It is never serialized into the public task projection.
func (j MobileVideoJob) CreateInput() MobileVideoJobCreateInput {
	return MobileVideoJobCreateInput{
		GroupID:           j.GroupID,
		Model:             j.Model,
		Prompt:            j.Prompt,
		Resolution:        j.Resolution,
		Ratio:             j.Ratio,
		DurationSeconds:   j.DurationSeconds,
		GenerateAudio:     j.GenerateAudio,
		Watermark:         j.Watermark,
		ReferenceAssetIDs: append([]string(nil), j.ReferenceAssetIDs...),
	}
}

// MobileVideoJobStore owns the private request and worker lease. The
// interface also makes the executor unit-testable without a database.
type MobileVideoJobStore interface {
	Create(context.Context, int64, string, MobileVideoJobCreateInput, string) error
	Get(context.Context, int64, string) (*MobileVideoJob, error)
	ClaimNext(context.Context, string, time.Time, time.Duration) (*MobileVideoJob, error)
	Heartbeat(context.Context, string, string, time.Time, time.Duration) error
	MarkSubmitted(context.Context, string, string, string, time.Time) error
	MarkCompleted(context.Context, string, string, string, string, string, int64) error
	MarkFailed(context.Context, string, string, string, string, bool) error
	MarkCancelled(context.Context, string, string) error
}

// MobileVideoJobService is the Postgres-backed implementation used by the
// mobile handler and worker. It mirrors MobileTaskService's placeholder
// binding so local SQL tests can still exercise the public service.
type MobileVideoJobService struct {
	db      *sql.DB
	dialect string
	now     func() time.Time
}

func NewMobileVideoJobService(db *sql.DB, dialectName ...string) *MobileVideoJobService {
	dialect := ""
	if len(dialectName) > 0 {
		dialect = strings.ToLower(strings.TrimSpace(dialectName[0]))
	}
	if dialect == "" && db != nil {
		typeName := strings.ToLower(fmt.Sprintf("%T", db.Driver()))
		if strings.Contains(typeName, "postgres") || strings.Contains(typeName, "pq.") || strings.Contains(typeName, "pgx") {
			dialect = "postgres"
		}
	}
	return &MobileVideoJobService{db: db, dialect: dialect, now: time.Now}
}

func (s *MobileVideoJobService) Create(ctx context.Context, userID int64, taskID string, input MobileVideoJobCreateInput, provider string) error {
	if s == nil || s.db == nil || userID <= 0 {
		return ErrMobileVideoJobStoreUnavailable
	}
	if _, err := uuid.Parse(strings.TrimSpace(taskID)); err != nil {
		return ErrMobileVideoJobNotFound
	}
	if err := ValidateMobileVideoJobCreateInput(input); err != nil {
		return err
	}
	provider = normalizeMobileVideoProvider(provider)
	refs, err := json.Marshal(normalizeMobileVideoReferenceIDs(input.ReferenceAssetIDs))
	if err != nil {
		return err
	}
	now := s.now().UTC()
	_, err = s.db.ExecContext(ctx, s.bind(`INSERT INTO mobile_video_jobs
		(task_id, user_id, group_id, model, prompt, resolution, ratio, duration_seconds,
		 generate_audio, watermark, reference_asset_ids, provider, state, next_poll_at,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (task_id) DO NOTHING`),
		taskID, userID, input.GroupID, strings.TrimSpace(input.Model), strings.TrimSpace(input.Prompt),
		strings.TrimSpace(input.Resolution), nullableMobileVideoString(input.Ratio), input.DurationSeconds,
		input.GenerateAudio, input.Watermark, string(refs), provider, string(MobileVideoJobStateQueued), now, now, now)
	return err
}

func (s *MobileVideoJobService) Get(ctx context.Context, userID int64, taskID string) (*MobileVideoJob, error) {
	if s == nil || s.db == nil || userID <= 0 {
		return nil, ErrMobileVideoJobStoreUnavailable
	}
	if _, err := uuid.Parse(strings.TrimSpace(taskID)); err != nil {
		return nil, ErrMobileVideoJobNotFound
	}
	return s.queryOne(ctx, mobileVideoJobSelect+` WHERE user_id = ? AND task_id = ?`, userID, taskID)
}

func (s *MobileVideoJobService) ClaimNext(ctx context.Context, owner string, now time.Time, lease time.Duration) (*MobileVideoJob, error) {
	if s == nil || s.db == nil {
		return nil, ErrMobileVideoJobStoreUnavailable
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, errors.New("mobile video lease owner is required")
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	now = now.UTC()
	leaseUntil := now.Add(lease)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// A crashed submission must be retried only when its lease expired. The
	// provider request id is persisted before the lease is released, so a
	// polling job is never re-submitted as a fresh generation.
	if _, err := tx.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'queued', lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE state = 'submitting' AND lease_expires_at < ?`), now, now); err != nil {
		return nil, err
	}
	query := mobileVideoJobSelect + ` WHERE state IN ('queued', 'polling') AND next_poll_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at < ?) ORDER BY next_poll_at ASC, created_at ASC LIMIT 1`
	if s.dialect == "postgres" {
		query += " FOR UPDATE SKIP LOCKED"
	}
	job, err := scanMobileVideoJob(tx.QueryRowContext(ctx, s.bind(query), now, now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	nextState := string(MobileVideoJobStatePolling)
	if strings.TrimSpace(job.ProviderRequestID) == "" {
		nextState = string(MobileVideoJobStateSubmitting)
	}
	if _, err := tx.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = ?, lease_owner = ?, lease_expires_at = ?, attempt_count = attempt_count + 1, updated_at = ? WHERE task_id = ?`), nextState, owner, leaseUntil, now, job.TaskID); err != nil {
		return nil, err
	}
	job.State = MobileVideoJobState(nextState)
	job.LeaseOwner = owner
	job.LeaseExpiresAt = &leaseUntil
	job.AttemptCount++
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *MobileVideoJobService) Heartbeat(ctx context.Context, taskID, owner string, now time.Time, lease time.Duration) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET lease_expires_at = ?, updated_at = ? WHERE task_id = ? AND lease_owner = ? AND state IN ('submitting', 'polling')`), now.UTC().Add(lease), now.UTC(), taskID, owner)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrMobileVideoJobNotFound
	}
	return nil
}

func (s *MobileVideoJobService) MarkSubmitted(ctx context.Context, taskID, owner, providerRequestID string, nextPollAt time.Time) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	providerRequestID = strings.TrimSpace(providerRequestID)
	if providerRequestID == "" {
		return ErrMobileVideoJobProviderID
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'polling', provider_request_id = ?, provider_status = 'queued', next_poll_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND lease_owner = ?`), providerRequestID, nextPollAt.UTC(), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkCompleted(ctx context.Context, taskID, owner, storageKey, artifactURL, contentType string, byteSize int64) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'completed', provider_status = 'completed', artifact_storage_key = ?, artifact_url = ?, artifact_content_type = ?, artifact_byte_size = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND lease_owner = ?`), nullableMobileVideoString(storageKey), nullableMobileVideoString(artifactURL), nullableMobileVideoString(contentType), nullableMobileVideoInt64(byteSize), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkFailed(ctx context.Context, taskID, owner, code, message string, retryable bool) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	errJSON, _ := json.Marshal(MobileTaskError{Code: strings.TrimSpace(code), Message: strings.TrimSpace(message), Retryable: retryable})
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'failed', provider_status = 'failed', last_error = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND lease_owner = ?`), string(errJSON), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkCancelled(ctx context.Context, taskID, owner string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	// Public task cancellation is the source of truth. Do not require the
	// caller to know the worker lease owner: a queued request has no owner and
	// an in-flight request must be marked cancelled even while its lease is
	// active. The worker checks the public task again before publishing output.
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'cancelled', provider_status = 'cancelled', lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND state IN ('queued', 'submitting', 'polling')`), s.now().UTC(), taskID)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func mobileVideoRowsAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrMobileVideoJobNotFound
	}
	return nil
}

const mobileVideoJobSelect = `SELECT task_id, user_id, group_id, model, prompt, resolution, ratio, duration_seconds, generate_audio, watermark, reference_asset_ids, provider, provider_request_id, provider_status, state, attempt_count, next_poll_at, lease_owner, lease_expires_at, artifact_storage_key, artifact_url, artifact_content_type, artifact_byte_size, last_error, created_at, updated_at FROM mobile_video_jobs`

type mobileVideoJobScanner interface{ Scan(dest ...any) error }

func (s *MobileVideoJobService) queryOne(ctx context.Context, query string, args ...any) (*MobileVideoJob, error) {
	job, err := scanMobileVideoJob(s.db.QueryRowContext(ctx, s.bind(query), args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMobileVideoJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func scanMobileVideoJob(scanner mobileVideoJobScanner) (MobileVideoJob, error) {
	var job MobileVideoJob
	var ratio, providerRequestID, providerStatus, leaseOwner, storageKey, artifactURL, contentType sql.NullString
	var leaseExpires sql.NullTime
	var refsJSON, errorJSON []byte
	var byteSize sql.NullInt64
	var state string
	err := scanner.Scan(&job.TaskID, &job.UserID, &job.GroupID, &job.Model, &job.Prompt, &job.Resolution, &ratio, &job.DurationSeconds, &job.GenerateAudio, &job.Watermark, &refsJSON, &job.Provider, &providerRequestID, &providerStatus, &state, &job.AttemptCount, &job.NextPollAt, &leaseOwner, &leaseExpires, &storageKey, &artifactURL, &contentType, &byteSize, &errorJSON, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return MobileVideoJob{}, err
	}
	job.Ratio = ratio.String
	job.ProviderRequestID = providerRequestID.String
	job.ProviderStatus = providerStatus.String
	job.State = MobileVideoJobState(state)
	job.LeaseOwner = leaseOwner.String
	job.ArtifactStorageKey = storageKey.String
	job.ArtifactURL = artifactURL.String
	job.ArtifactContentType = contentType.String
	if byteSize.Valid {
		job.ArtifactByteSize = byteSize.Int64
	}
	if leaseExpires.Valid {
		value := leaseExpires.Time.UTC()
		job.LeaseExpiresAt = &value
	}
	if len(refsJSON) > 0 {
		_ = json.Unmarshal(refsJSON, &job.ReferenceAssetIDs)
	}
	if len(errorJSON) > 0 {
		var taskError MobileTaskError
		if json.Unmarshal(errorJSON, &taskError) == nil {
			job.LastError = &taskError
		}
	}
	job.CreatedAt = job.CreatedAt.UTC()
	job.UpdatedAt = job.UpdatedAt.UTC()
	return job, nil
}

func (s *MobileVideoJobService) bind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	position := 1
	for _, char := range query {
		if char == '?' {
			fmt.Fprintf(&b, "$%d", position)
			position++
		} else {
			_, _ = b.WriteRune(char)
		}
	}
	return b.String()
}

func nullableMobileVideoString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullableMobileVideoInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func normalizeMobileVideoReferenceIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeMobileVideoProvider(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), PlatformGrok) {
		return PlatformGrok
	}
	return PlatformOpenAI
}

// MobileVideoProvider is the narrow adapter around the existing Agnes/Grok
// gateway. Keeping it outside the DB worker allows provider calls to be
// retried and tested without exposing an API key or a gin request to clients.
type MobileVideoProvider interface {
	Create(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error)
	Poll(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error)
	Content(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error)
}

type MobileVideoProviderResult struct {
	RequestID     string
	Status        string
	Progress      int
	NextPollAfter time.Duration
	Artifact      []byte
	ArtifactURL   string
	ContentType   string
	ErrorCode     string
	ErrorMessage  string
	Retryable     bool
}

// MobileVideoTaskStore is the subset of the public task service needed by the
// worker. Keeping it as an interface avoids coupling provider acceptance tests
// to a live database while *MobileTaskService remains the production
// implementation.
type MobileVideoTaskStore interface {
	Get(context.Context, int64, string) (*MobileTask, error)
	Transition(context.Context, int64, string, MobileTaskTransitionInput) (*MobileTask, error)
}

type MobileVideoProviderError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *MobileVideoProviderError) Error() string {
	if e == nil {
		return "mobile video provider error"
	}
	return strings.TrimSpace(e.Message)
}

// MobileVideoWorker executes one private job at a time per claimed lease. It
// intentionally does not enqueue provider payloads in Redis; Postgres keeps
// the request and provider id durable across restarts.
type MobileVideoWorker struct {
	store    MobileVideoJobStore
	tasks    MobileVideoTaskStore
	provider MobileVideoProvider
	storage  ImageStorage
	workerID string
	lease    time.Duration
	poll     time.Duration
	now      func() time.Time

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	wg       sync.WaitGroup
	running  atomic.Bool
}

type MobileVideoWorkerOptions struct {
	WorkerID     string
	Lease        time.Duration
	PollInterval time.Duration
}

func NewMobileVideoWorker(store MobileVideoJobStore, tasks MobileVideoTaskStore, provider MobileVideoProvider, storage ImageStorage, options MobileVideoWorkerOptions) *MobileVideoWorker {
	if strings.TrimSpace(options.WorkerID) == "" {
		options.WorkerID = "mobile-video-" + uuid.NewString()
	}
	if options.Lease <= 0 {
		options.Lease = 2 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 2 * time.Second
	}
	return &MobileVideoWorker{store: store, tasks: tasks, provider: provider, storage: storage, workerID: options.WorkerID, lease: options.Lease, poll: options.PollInterval, now: time.Now}
}

func (w *MobileVideoWorker) Start() {
	if w == nil || w.store == nil || w.tasks == nil || w.provider == nil {
		return
	}
	if w.running.Swap(true) {
		return
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer w.running.Store(false)
		for {
			if w.ctx.Err() != nil {
				return
			}
			processed, _ := w.RunOnce(w.ctx)
			if processed {
				continue
			}
			timer := time.NewTimer(w.poll)
			select {
			case <-w.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (w *MobileVideoWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
		w.wg.Wait()
	})
}

func (w *MobileVideoWorker) Running() bool { return w != nil && w.running.Load() }

// RunOnce claims and processes at most one job. It is exported for a
// deterministic worker acceptance test and for installations that run workers
// from an external scheduler.
func (w *MobileVideoWorker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil || w.store == nil || w.tasks == nil || w.provider == nil {
		return false, ErrMobileVideoJobStoreUnavailable
	}
	job, err := w.store.ClaimNext(ctx, w.workerID, w.now().UTC(), w.lease)
	if errors.Is(err, ErrMobileVideoJobNotFound) || job == nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, w.process(ctx, job)
}

func (w *MobileVideoWorker) process(ctx context.Context, job *MobileVideoJob) error {
	task, err := w.tasks.Get(ctx, job.UserID, job.TaskID)
	if errors.Is(err, ErrMobileTaskNotFound) {
		_ = w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
		return nil
	}
	if err != nil {
		return err
	}
	if task.Status == MobileTaskStatusCancelled {
		return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
	}

	var result MobileVideoProviderResult
	if strings.TrimSpace(job.ProviderRequestID) == "" {
		result, err = w.provider.Create(ctx, job)
	} else {
		result, err = w.provider.Poll(ctx, job)
	}
	if err != nil {
		return w.fail(ctx, job, err)
	}
	status := strings.ToLower(strings.TrimSpace(result.Status))
	if status == "" {
		status = "processing"
	}
	if isMobileVideoProviderFailure(status) {
		return w.failResult(ctx, job, result)
	}
	if isMobileVideoProviderComplete(status) {
		if len(result.Artifact) == 0 && strings.TrimSpace(result.ArtifactURL) == "" {
			contentResult, contentErr := w.provider.Content(ctx, job)
			if contentErr != nil {
				return w.fail(ctx, job, contentErr)
			}
			result.Artifact = contentResult.Artifact
			result.ArtifactURL = contentResult.ArtifactURL
			if result.ContentType == "" {
				result.ContentType = contentResult.ContentType
			}
		}
		return w.complete(ctx, job, result)
	}
	if strings.TrimSpace(job.ProviderRequestID) == "" && strings.TrimSpace(result.RequestID) == "" {
		return w.fail(ctx, job, &MobileVideoProviderError{Code: "VIDEO_PROVIDER_REQUEST_ID_MISSING", Message: ErrMobileVideoJobProviderID.Error(), Retryable: false})
	}
	if strings.TrimSpace(result.RequestID) != "" {
		job.ProviderRequestID = strings.TrimSpace(result.RequestID)
	}
	nextPoll := result.NextPollAfter
	if nextPoll <= 0 {
		nextPoll = w.poll
	}
	if err := w.store.MarkSubmitted(ctx, job.TaskID, w.workerID, job.ProviderRequestID, w.now().UTC().Add(nextPoll)); err != nil {
		return err
	}
	if task.Status == MobileTaskStatusQueued {
		progress := normalizeMobileVideoProgress(result.Progress)
		_, err = w.tasks.Transition(ctx, job.UserID, job.TaskID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning, Progress: &progress})
		return err
	}
	return nil
}

func (w *MobileVideoWorker) complete(ctx context.Context, job *MobileVideoJob, result MobileVideoProviderResult) error {
	// Cancellation can win the race while the provider is finishing. Never
	// publish bytes or mark a cancelled public task as completed.
	if task, err := w.tasks.Get(ctx, job.UserID, job.TaskID); err == nil && task.Status == MobileTaskStatusCancelled {
		_ = w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
		return nil
	}
	if len(result.Artifact) == 0 {
		return w.fail(ctx, job, ErrMobileVideoJobArtifact)
	}
	contentType := strings.TrimSpace(result.ContentType)
	if contentType == "" {
		contentType = "video/mp4"
	}
	storageKey := ""
	artifactURL := strings.TrimSpace(result.ArtifactURL)
	if w.storage != nil {
		storageKey = fmt.Sprintf("mobile-video-results/%d/%s.mp4", job.UserID, job.TaskID)
		var err error
		artifactURL, err = w.storage.Save(ctx, storageKey, contentType, result.Artifact)
		if err != nil {
			return w.fail(ctx, job, err)
		}
	}
	if err := w.store.MarkCompleted(ctx, job.TaskID, w.workerID, storageKey, artifactURL, contentType, int64(len(result.Artifact))); err != nil {
		return err
	}
	artifact := MobileTaskArtifact{ID: job.TaskID, Kind: "video", Name: "video.mp4", ContentType: contentType, URL: artifactURL, ByteSize: int64(len(result.Artifact))}
	_, err := w.tasks.Transition(ctx, job.UserID, job.TaskID, MobileTaskTransitionInput{Status: MobileTaskStatusCompleted, Progress: intPtrMobileVideo(100), Artifacts: []MobileTaskArtifact{artifact}})
	return err
}

func (w *MobileVideoWorker) fail(ctx context.Context, job *MobileVideoJob, err error) error {
	providerErr := &MobileVideoProviderError{Code: "VIDEO_PROVIDER_ERROR", Message: err.Error(), Retryable: false}
	if typed := new(MobileVideoProviderError); errors.As(err, &typed) {
		providerErr = typed
	}
	return w.failResult(ctx, job, MobileVideoProviderResult{ErrorCode: providerErr.Code, ErrorMessage: providerErr.Message, Retryable: providerErr.Retryable, Status: "failed"})
}

func (w *MobileVideoWorker) failResult(ctx context.Context, job *MobileVideoJob, result MobileVideoProviderResult) error {
	code := strings.TrimSpace(result.ErrorCode)
	if code == "" {
		code = "VIDEO_PROVIDER_ERROR"
	}
	message := strings.TrimSpace(result.ErrorMessage)
	if message == "" {
		message = "视频生成失败"
	}
	if err := w.store.MarkFailed(ctx, job.TaskID, w.workerID, code, message, result.Retryable); err != nil {
		return err
	}
	_, err := w.tasks.Transition(ctx, job.UserID, job.TaskID, MobileTaskTransitionInput{Status: MobileTaskStatusFailed, Error: &MobileTaskError{Code: code, Message: message, Retryable: result.Retryable}})
	return err
}

func isMobileVideoProviderFailure(status string) bool {
	switch status {
	case "failed", "error", "cancelled", "canceled", "rejected":
		return true
	default:
		return false
	}
}

func isMobileVideoProviderComplete(status string) bool {
	switch status {
	case "completed", "complete", "succeeded", "success", "done":
		return true
	default:
		return false
	}
}

func normalizeMobileVideoProgress(value int) int {
	if value < 1 {
		return 1
	}
	if value > 99 {
		return 99
	}
	return value
}

func intPtrMobileVideo(value int) *int { return &value }
