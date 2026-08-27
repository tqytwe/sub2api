package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// MobileVideoJobState is private executor state. It never replaces the public
// MobileTaskStatus projection returned to the app.
type MobileVideoJobState string

const (
	MobileVideoJobStateFunding           MobileVideoJobState = "funding"
	MobileVideoJobStateQueued            MobileVideoJobState = "queued"
	MobileVideoJobStateSubmitting        MobileVideoJobState = "submitting"
	MobileVideoJobStateSubmissionUnknown MobileVideoJobState = "submission_unknown"
	MobileVideoJobStatePolling           MobileVideoJobState = "polling"
	MobileVideoJobStateCompleted         MobileVideoJobState = "completed"
	MobileVideoJobStateFailed            MobileVideoJobState = "failed"
	MobileVideoJobStateCancelled         MobileVideoJobState = "cancelled"
)

// MobileVideoBillingState is independent from provider execution state. A
// worker may retry settlement idempotently after an upstream completion or a
// client-side cancellation without re-dispatching the provider request.
type MobileVideoBillingState string

const (
	MobileVideoBillingStateFunding     MobileVideoBillingState = "funding"
	MobileVideoBillingStateReserved    MobileVideoBillingState = "reserved"
	MobileVideoBillingStateReleasing   MobileVideoBillingState = "releasing"
	MobileVideoBillingStateCaptured    MobileVideoBillingState = "captured"
	MobileVideoBillingStateReleased    MobileVideoBillingState = "released"
	MobileVideoBillingStateNotRequired MobileVideoBillingState = "not_required"
)

var (
	ErrMobileVideoJobNotFound         = errors.New("mobile video job not found")
	ErrMobileVideoJobStoreUnavailable = errors.New("mobile video job store is unavailable")
	ErrMobileVideoJobAlreadyExists    = errors.New("mobile video job already exists")
	ErrMobileVideoJobProviderID       = errors.New("mobile video provider request id is missing")
	ErrMobileVideoJobArtifact         = errors.New("mobile video provider returned no artifact")
	ErrMobileVideoLeaseLost           = errors.New("mobile video worker lease was lost")
	ErrMobileVideoTaskCancelled       = errors.New("mobile video task was cancelled before completion")
	ErrMobileVideoFundingNotReady     = errors.New("mobile video funding is not ready")
)

// MobileVideoJob is the private execution row. Its prompt, model references,
// provider request ID, group-pinned execution key and storage key must never
// be serialized into a public MobileTask response.
type MobileVideoJob struct {
	TaskID              string
	UserID              int64
	GroupID             int64
	ExecutionAPIKeyID   int64
	ExecutionSnapshot   *MobileVideoExecutionSnapshot
	UnitPriceUSD        float64
	RateMultiplier      float64
	HoldAmount          float64
	BillingState        MobileVideoBillingState
	Adapter             string
	Model               string
	Prompt              string
	Resolution          string
	Ratio               string
	DurationSeconds     int
	GenerateAudio       bool
	Watermark           bool
	ReferenceAssetIDs   []string
	ProviderRequestID   string
	ProviderStatus      string
	State               MobileVideoJobState
	AttemptCount        int
	NextPollAt          time.Time
	LeaseOwner          string
	LeaseExpiresAt      *time.Time
	ArtifactStorageKey  string
	ArtifactContentType string
	ArtifactByteSize    int64
	ClientCancelledAt   *time.Time
	LastError           *MobileTaskError
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (j MobileVideoJob) CreateInput() MobileVideoJobCreateInput {
	return MobileVideoJobCreateInput{
		GroupID:           j.GroupID,
		ExecutionAPIKeyID: j.ExecutionAPIKeyID,
		ExecutionSnapshot: cloneMobileVideoExecutionSnapshot(j.ExecutionSnapshot),
		UnitPriceUSD:      j.UnitPriceUSD,
		RateMultiplier:    j.RateMultiplier,
		HoldAmount:        j.HoldAmount,
		Adapter:           j.Adapter,
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

// MobileVideoJobStore owns durable execution input and worker leases.
type MobileVideoJobStore interface {
	Create(context.Context, int64, string, MobileVideoJobCreateInput) error
	Get(context.Context, int64, string) (*MobileVideoJob, error)
	ClaimNext(context.Context, string, time.Time, time.Duration) (*MobileVideoJob, error)
	ClaimFundingRelease(context.Context, string, time.Time, time.Duration) (*MobileVideoJob, error)
	Heartbeat(context.Context, string, string, time.Time, time.Duration) error
	MarkSubmitted(context.Context, string, string, string, time.Time) error
	MarkSubmissionUnknown(context.Context, string, string, string) error
	MarkCompleted(context.Context, string, string, string, string, int64) error
	MarkFailed(context.Context, string, string, string, string, bool) error
	MarkClientCancelled(context.Context, string) error
	MarkFundingReserved(context.Context, string) error
	MarkFundingReleasePending(context.Context, string) error
	MarkFundingReleased(context.Context, string) error
	MarkCancelled(context.Context, string, string) error
	MarkBillingCaptured(context.Context, string, string) error
	MarkBillingReleasePending(context.Context, string, string) error
	MarkBillingReleased(context.Context, string, string) error
	ActivateReserved(context.Context, int64, string, int) error
	DiscardUnfunded(context.Context, int64, string) error
	ReleaseArtifact(context.Context, int64, string) error
}

// MobileVideoJobService is a Postgres-backed private job store. It uses a
// lease so multiple server replicas can resume polling after a restart.
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

func (s *MobileVideoJobService) Create(ctx context.Context, userID int64, taskID string, input MobileVideoJobCreateInput) error {
	if s == nil || s.db == nil || userID <= 0 {
		return ErrMobileVideoJobStoreUnavailable
	}
	return insertMobileVideoJob(ctx, s.db, s.bind, userID, taskID, input, s.now().UTC())
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

	// A create whose process died before its upstream task ID was persisted is
	// ambiguous. Never resubmit it automatically because that can bill twice.
	if _, err := tx.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET state = 'submission_unknown', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE state = 'submitting' AND lease_expires_at < ?`), now, now); err != nil {
		return nil, err
	}

	query := mobileVideoJobSelect + ` WHERE state IN ('queued', 'polling', 'submission_unknown') AND next_poll_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at < ?) ORDER BY next_poll_at ASC, created_at ASC LIMIT 1`
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
	nextState := MobileVideoJobStatePolling
	if job.State == MobileVideoJobStateSubmissionUnknown {
		// A process died after the upstream submit began but before a provider ID
		// was committed. Claim it only to surface a retryable terminal error; a
		// worker must never resubmit an ambiguous, potentially billed request.
		nextState = MobileVideoJobStateSubmissionUnknown
	} else if strings.TrimSpace(job.ProviderRequestID) == "" {
		nextState = MobileVideoJobStateSubmitting
	}
	if _, err := tx.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = ?, lease_owner = ?, lease_expires_at = ?, attempt_count = attempt_count + 1, updated_at = ? WHERE task_id = ?`), string(nextState), owner, leaseUntil, now, job.TaskID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	job.State = nextState
	job.LeaseOwner = owner
	job.LeaseExpiresAt = &leaseUntil
	job.AttemptCount++
	job.UpdatedAt = now
	return &job, nil
}

// ClaimFundingRelease recovers only a compensation that was already fenced by
// MarkFundingReleasePending. It must never claim ordinary `funding` rows:
// those can only become runnable after the request handler has durably
// reserved the balance and activated them.
func (s *MobileVideoJobService) ClaimFundingRelease(ctx context.Context, owner string, now time.Time, lease time.Duration) (*MobileVideoJob, error) {
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

	query := mobileVideoJobSelect + ` WHERE state = 'funding' AND billing_state = 'releasing'
		AND (lease_expires_at IS NULL OR lease_expires_at < ?)
		ORDER BY updated_at ASC, created_at ASC LIMIT 1`
	if s.dialect == "postgres" {
		query += " FOR UPDATE SKIP LOCKED"
	}
	job, err := scanMobileVideoJob(tx.QueryRowContext(ctx, s.bind(query), now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET lease_owner = ?, lease_expires_at = ?, attempt_count = attempt_count + 1, updated_at = ?
		WHERE task_id = ? AND state = 'funding' AND billing_state = 'releasing'`), owner, leaseUntil, now, job.TaskID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	job.LeaseOwner = owner
	job.LeaseExpiresAt = &leaseUntil
	job.AttemptCount++
	job.UpdatedAt = now
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
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkSubmitted(ctx context.Context, taskID, owner, providerRequestID string, nextPollAt time.Time) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	providerRequestID = strings.TrimSpace(providerRequestID)
	if providerRequestID == "" {
		return ErrMobileVideoJobProviderID
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'polling', provider_request_id = ?, provider_status = 'queued', next_poll_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND lease_owner = ? AND state = 'submitting'`), providerRequestID, nextPollAt.UTC(), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

// MarkSubmissionUnknown is used only after a create call failed before a
// provider request ID was durably recorded. The worker must never resubmit it;
// a later reconciliation captures the accepted hold conservatively.
func (s *MobileVideoJobService) MarkSubmissionUnknown(ctx context.Context, taskID, owner, message string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	errJSON, _ := json.Marshal(MobileTaskError{Code: "VIDEO_SUBMISSION_UNKNOWN", Message: strings.TrimSpace(message), Retryable: true})
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET state = 'submission_unknown', provider_status = 'submission_unknown', last_error = ?,
		    lease_owner = NULL, lease_expires_at = NULL, next_poll_at = ?, updated_at = ?
		WHERE task_id = ? AND lease_owner = ? AND state = 'submitting'`), string(errJSON), s.now().UTC(), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkCompleted(ctx context.Context, taskID, owner, storageKey, contentType string, byteSize int64) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'completed', provider_status = 'completed', artifact_storage_key = ?, artifact_content_type = ?, artifact_byte_size = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND lease_owner = ? AND state IN ('submitting', 'polling')`), nullableMobileVideoString(storageKey), nullableMobileVideoString(contentType), nullableMobileVideoInt64(byteSize), s.now().UTC(), taskID, owner)
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
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET state = 'failed', provider_status = 'failed', last_error = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ? WHERE task_id = ? AND (lease_owner = ? OR state = 'submission_unknown') AND state IN ('submitting', 'polling', 'submission_unknown')`), string(errJSON), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

// MarkClientCancelled records the user-visible cancellation without making a
// submitted provider task disappear from the reconciliation queue.
func (s *MobileVideoJobService) MarkClientCancelled(ctx context.Context, taskID string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET client_cancelled_at = COALESCE(client_cancelled_at, ?), updated_at = ?
		WHERE task_id = ? AND state IN ('funding', 'queued', 'submitting', 'polling', 'submission_unknown')`), s.now().UTC(), s.now().UTC(), taskID)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

// MarkFundingReserved advances a non-runnable job only after the balance hold
// committed. Repeating it is safe for a request retry after a process crash.
func (s *MobileVideoJobService) MarkFundingReserved(ctx context.Context, taskID string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET billing_state = 'reserved', updated_at = ?
		WHERE task_id = ? AND state = 'funding' AND billing_state IN ('funding', 'reserved')`), s.now().UTC(), taskID)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

// MarkFundingReleasePending durably fences a funding row before its balance
// hold is released. This must happen before the external balance write: a
// response loss after Release must never leave a `reserved` row eligible for
// activation without a hold.
func (s *MobileVideoJobService) MarkFundingReleasePending(ctx context.Context, taskID string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET billing_state = 'releasing', updated_at = ?
		WHERE task_id = ? AND state = 'funding' AND billing_state IN ('reserved', 'releasing')`), s.now().UTC(), taskID)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

// MarkFundingReleased records a completed compensation release before a
// funded job was made runnable. It is deliberately separate from
// MarkBillingReleased: there is no worker lease yet, and callers must first
// persist MarkFundingReleasePending before releasing an accepted hold.
func (s *MobileVideoJobService) MarkFundingReleased(ctx context.Context, taskID string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET billing_state = 'released', updated_at = ?
		WHERE task_id = ? AND state = 'funding' AND billing_state IN ('releasing', 'released')`), s.now().UTC(), taskID)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

// ActivateReserved makes a funded job claimable only after a durable hold is
// present. The user row lock turns the queue limit into a cross-replica bound.
func (s *MobileVideoJobService) ActivateReserved(ctx context.Context, userID int64, taskID string, maxActive int) error {
	if s == nil || s.db == nil || userID <= 0 {
		return ErrMobileVideoJobStoreUnavailable
	}
	if maxActive <= 0 {
		maxActive = MobileVideoActiveJobsPerUser
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	query := mobileVideoJobSelect + ` WHERE task_id = ? AND user_id = ?`
	if s.dialect == "postgres" {
		query += " FOR UPDATE"
	}
	job, err := queryMobileVideoJob(ctx, tx, s.bind(query), taskID, userID)
	if err != nil {
		return err
	}
	if job.State == MobileVideoJobStateQueued && job.BillingState == MobileVideoBillingStateReserved {
		return tx.Commit()
	}
	if job.ClientCancelledAt != nil {
		return ErrMobileVideoTaskCancelled
	}
	if job.State != MobileVideoJobStateFunding || job.BillingState != MobileVideoBillingStateReserved ||
		strings.TrimSpace(job.ProviderRequestID) != "" || !ValidMobileVideoExecutionSnapshot(job.ExecutionSnapshot, userID, job.GroupID, job.ExecutionAPIKeyID) {
		return ErrMobileVideoFundingNotReady
	}
	lockUser := `SELECT id FROM users WHERE id = ? AND deleted_at IS NULL`
	if s.dialect == "postgres" {
		lockUser += " FOR UPDATE"
	}
	var lockedUserID int64
	if err := tx.QueryRowContext(ctx, s.bind(lockUser), userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMobileVideoFundingNotReady
		}
		return err
	}
	var active int
	if err := tx.QueryRowContext(ctx, s.bind(`SELECT COUNT(*) FROM mobile_video_jobs
		WHERE user_id = ? AND task_id <> ?
		  AND state IN ('queued', 'submitting', 'polling', 'submission_unknown')`), userID, taskID).Scan(&active); err != nil {
		return err
	}
	if active >= maxActive {
		return ErrMobileVideoQueueLimit
	}
	result, err := tx.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET state = 'queued', next_poll_at = ?, updated_at = ?
		WHERE task_id = ? AND user_id = ? AND state = 'funding' AND billing_state = 'reserved'`), s.now().UTC(), s.now().UTC(), taskID, userID)
	if err != nil {
		return err
	}
	if err := mobileVideoRowsAffected(result); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardUnfunded removes the public/private pair only while no reservation or
// upstream request exists. It is the compensation path for a rejected hold.
func (s *MobileVideoJobService) DiscardUnfunded(ctx context.Context, userID int64, taskID string) error {
	if s == nil || s.db == nil || userID <= 0 {
		return ErrMobileVideoJobStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	query := mobileVideoJobSelect + ` WHERE task_id = ? AND user_id = ?`
	if s.dialect == "postgres" {
		query += " FOR UPDATE"
	}
	job, err := queryMobileVideoJob(ctx, tx, s.bind(query), taskID, userID)
	if err != nil {
		return err
	}
	if job.State != MobileVideoJobStateFunding ||
		(job.BillingState != MobileVideoBillingStateFunding && job.BillingState != MobileVideoBillingStateReleased) ||
		strings.TrimSpace(job.ProviderRequestID) != "" {
		return ErrMobileVideoFundingNotReady
	}
	result, err := tx.ExecContext(ctx, s.bind(`DELETE FROM mobile_tasks WHERE id = ? AND user_id = ?`), taskID, userID)
	if err != nil {
		return err
	}
	if err := mobileVideoRowsAffected(result); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MobileVideoJobService) MarkBillingCaptured(ctx context.Context, taskID, owner string) error {
	return s.markBillingState(ctx, taskID, owner, MobileVideoBillingStateCaptured)
}

// MarkBillingReleasePending fences a claimed worker job before its external
// release. A stale worker can no longer capture it if the release succeeds but
// this process loses the subsequent state-write response.
func (s *MobileVideoJobService) MarkBillingReleasePending(ctx context.Context, taskID, owner string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET billing_state = 'releasing', updated_at = ?
		WHERE task_id = ? AND lease_owner = ? AND billing_state IN ('reserved', 'releasing')
		  AND state IN ('submitting', 'polling', 'submission_unknown')`), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkBillingReleased(ctx context.Context, taskID, owner string) error {
	return s.markBillingState(ctx, taskID, owner, MobileVideoBillingStateReleased)
}

func (s *MobileVideoJobService) markBillingState(ctx context.Context, taskID, owner string, billingState MobileVideoBillingState) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	if billingState != MobileVideoBillingStateCaptured && billingState != MobileVideoBillingStateReleased {
		return ErrMobileVideoFundingNotReady
	}
	// A hold has exactly one terminal settlement. Allow a same-state retry for
	// idempotent recovery, but never let a stale worker turn a release into a
	// capture (or the reverse) after the balance ledger has settled.
	allowedPriorStates := "'reserved', 'captured'"
	if billingState == MobileVideoBillingStateReleased {
		allowedPriorStates = "'releasing', 'released'"
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs
		SET billing_state = ?, updated_at = ?
		WHERE task_id = ? AND lease_owner = ? AND billing_state IN (`+allowedPriorStates+`)
		  AND state IN ('submitting', 'polling', 'submission_unknown')`), string(billingState), s.now().UTC(), taskID, owner)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) MarkCancelled(ctx context.Context, taskID, owner string) error {
	if s == nil || s.db == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	owner = strings.TrimSpace(owner)
	query := `UPDATE mobile_video_jobs
		SET state = 'cancelled', provider_status = 'cancelled', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE task_id = ? AND state = 'funding'`
	args := []any{s.now().UTC(), taskID}
	if owner != "" {
		query = `UPDATE mobile_video_jobs
			SET state = 'cancelled', provider_status = 'cancelled', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
			WHERE task_id = ? AND lease_owner = ? AND state IN ('submitting', 'polling', 'submission_unknown')`
		args = append(args, owner)
	}
	result, err := s.db.ExecContext(ctx, s.bind(query), args...)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

func (s *MobileVideoJobService) ReleaseArtifact(ctx context.Context, userID int64, taskID string) error {
	if s == nil || s.db == nil || userID <= 0 {
		return ErrMobileVideoJobStoreUnavailable
	}
	result, err := s.db.ExecContext(ctx, s.bind(`UPDATE mobile_video_jobs SET artifact_storage_key = NULL, artifact_content_type = NULL, artifact_byte_size = NULL, updated_at = ? WHERE user_id = ? AND task_id = ? AND state = 'completed'`), s.now().UTC(), userID, taskID)
	if err != nil {
		return err
	}
	return mobileVideoRowsAffected(result)
}

const mobileVideoJobSelect = `SELECT task_id, user_id, group_id, execution_api_key_id, adapter, model, prompt, resolution, ratio, duration_seconds, generate_audio, watermark, reference_asset_ids, execution_snapshot, unit_price_usd, rate_multiplier, hold_amount, billing_state, provider_request_id, provider_status, state, attempt_count, next_poll_at, lease_owner, lease_expires_at, artifact_storage_key, artifact_content_type, artifact_byte_size, client_cancelled_at, last_error, created_at, updated_at FROM mobile_video_jobs`

func (s *MobileVideoJobService) queryOne(ctx context.Context, query string, args ...any) (*MobileVideoJob, error) {
	return queryMobileVideoJob(ctx, s.db, s.bind(query), args...)
}

type mobileVideoJobQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type mobileVideoJobExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func queryMobileVideoJob(ctx context.Context, queryer mobileVideoJobQueryer, query string, args ...any) (*MobileVideoJob, error) {
	job, err := scanMobileVideoJob(queryer.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMobileVideoJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// insertMobileVideoJob is shared by the standalone store and the mobile task
// transaction. Keeping the public task projection and this private row in the
// same transaction prevents a queue entry that has no executable request.
func insertMobileVideoJob(
	ctx context.Context,
	executor mobileVideoJobExecutor,
	bind func(string) string,
	userID int64,
	taskID string,
	input MobileVideoJobCreateInput,
	now time.Time,
) error {
	if executor == nil || userID <= 0 {
		return ErrMobileVideoJobStoreUnavailable
	}
	if _, err := uuid.Parse(strings.TrimSpace(taskID)); err != nil {
		return ErrMobileVideoJobNotFound
	}
	if err := ValidateMobileVideoJobCreateInput(input); err != nil {
		return err
	}
	refs, err := json.Marshal(normalizeMobileVideoReferenceIDs(input.ReferenceAssetIDs))
	if err != nil {
		return fmt.Errorf("encode mobile video references: %w", err)
	}
	state, billingState, snapshotJSON, unitPrice, rateMultiplier, holdAmount, err := mobileVideoInitialFunding(input)
	if err != nil {
		return err
	}
	now = now.UTC()
	result, err := executor.ExecContext(ctx, bind(`INSERT INTO mobile_video_jobs
		(task_id, user_id, group_id, execution_api_key_id, adapter, model, prompt, resolution, ratio,
		 duration_seconds, generate_audio, watermark, reference_asset_ids, execution_snapshot, unit_price_usd,
		 rate_multiplier, hold_amount, billing_state, state, next_poll_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (task_id) DO NOTHING`),
		taskID, userID, input.GroupID, input.ExecutionAPIKeyID, strings.TrimSpace(input.Adapter), strings.TrimSpace(input.Model),
		strings.TrimSpace(input.Prompt), strings.TrimSpace(input.Resolution), nullableMobileVideoString(input.Ratio),
		input.DurationSeconds, input.GenerateAudio, input.Watermark, string(refs), snapshotJSON, unitPrice,
		rateMultiplier, holdAmount, string(billingState), string(state), now, now, now)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrMobileVideoJobAlreadyExists
	}
	return nil
}

func mobileVideoInitialFunding(input MobileVideoJobCreateInput) (MobileVideoJobState, MobileVideoBillingState, any, any, any, any, error) {
	if input.ExecutionSnapshot == nil {
		return MobileVideoJobStateQueued, MobileVideoBillingStateNotRequired, nil, nil, nil, nil, nil
	}
	if !ValidMobileVideoExecutionSnapshot(input.ExecutionSnapshot, input.ExecutionSnapshot.APIKey.UserID, input.GroupID, input.ExecutionAPIKeyID) ||
		math.IsNaN(input.UnitPriceUSD) || math.IsInf(input.UnitPriceUSD, 0) ||
		math.IsNaN(input.RateMultiplier) || math.IsInf(input.RateMultiplier, 0) ||
		math.IsNaN(input.HoldAmount) || math.IsInf(input.HoldAmount, 0) ||
		input.UnitPriceUSD < 0 || input.RateMultiplier < 0 || input.HoldAmount < 0 ||
		QuantizeUsageBillingAmount(input.RateMultiplier) != QuantizeUsageBillingAmount(input.ExecutionSnapshot.EffectiveVideoRateMultiplier) {
		return "", "", nil, nil, nil, nil, ErrMobileVideoFundingInvalid
	}
	expectedHold, holdErr := ValidateMobileVideoHoldAmount(input.UnitPriceUSD, input.DurationSeconds, input.RateMultiplier)
	if holdErr != nil || QuantizeUsageBillingAmount(input.HoldAmount) != expectedHold {
		return "", "", nil, nil, nil, nil, ErrMobileVideoFundingInvalid
	}
	payload, err := json.Marshal(input.ExecutionSnapshot)
	if err != nil {
		return "", "", nil, nil, nil, nil, fmt.Errorf("encode mobile video execution snapshot: %w", err)
	}
	return MobileVideoJobStateFunding, MobileVideoBillingStateFunding, string(payload),
		QuantizeUsageBillingAmount(input.UnitPriceUSD), QuantizeUsageBillingAmount(input.RateMultiplier), QuantizeUsageBillingAmount(input.HoldAmount), nil
}

func mobileVideoJobRequiresFunding(job *MobileVideoJob) bool {
	return job != nil && job.BillingState != "" && job.BillingState != MobileVideoBillingStateNotRequired
}

func sameMobileVideoJobCreate(existing *MobileVideoJob, input MobileVideoJobCreateInput) bool {
	if existing == nil || existing.GroupID != input.GroupID ||
		!strings.EqualFold(strings.TrimSpace(existing.Model), strings.TrimSpace(input.Model)) ||
		strings.TrimSpace(existing.Prompt) != strings.TrimSpace(input.Prompt) ||
		!strings.EqualFold(strings.TrimSpace(existing.Resolution), strings.TrimSpace(input.Resolution)) ||
		strings.TrimSpace(existing.Ratio) != strings.TrimSpace(input.Ratio) ||
		existing.DurationSeconds != input.DurationSeconds ||
		existing.GenerateAudio != input.GenerateAudio ||
		existing.Watermark != input.Watermark {
		return false
	}
	existingRefs := normalizeMobileVideoReferenceIDs(existing.ReferenceAssetIDs)
	inputRefs := normalizeMobileVideoReferenceIDs(input.ReferenceAssetIDs)
	if len(existingRefs) != len(inputRefs) {
		return false
	}
	for index, referenceID := range existingRefs {
		if referenceID != inputRefs[index] {
			return false
		}
	}
	return true
}

type mobileVideoJobScanner interface{ Scan(dest ...any) error }

func scanMobileVideoJob(scanner mobileVideoJobScanner) (MobileVideoJob, error) {
	var job MobileVideoJob
	var ratio, requestID, providerStatus, leaseOwner, storageKey, contentType, billingState sql.NullString
	var leaseExpires, clientCancelledAt sql.NullTime
	var byteSize sql.NullInt64
	var unitPrice, rateMultiplier, holdAmount sql.NullFloat64
	var refsJSON, snapshotJSON, errorJSON []byte
	var state string
	err := scanner.Scan(&job.TaskID, &job.UserID, &job.GroupID, &job.ExecutionAPIKeyID, &job.Adapter, &job.Model, &job.Prompt, &job.Resolution, &ratio, &job.DurationSeconds, &job.GenerateAudio, &job.Watermark, &refsJSON, &snapshotJSON, &unitPrice, &rateMultiplier, &holdAmount, &billingState, &requestID, &providerStatus, &state, &job.AttemptCount, &job.NextPollAt, &leaseOwner, &leaseExpires, &storageKey, &contentType, &byteSize, &clientCancelledAt, &errorJSON, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return MobileVideoJob{}, err
	}
	job.Ratio = ratio.String
	job.ProviderRequestID = requestID.String
	job.ProviderStatus = providerStatus.String
	job.State = MobileVideoJobState(state)
	job.LeaseOwner = leaseOwner.String
	job.ArtifactStorageKey = storageKey.String
	job.ArtifactContentType = contentType.String
	job.ArtifactByteSize = byteSize.Int64
	job.UnitPriceUSD = unitPrice.Float64
	job.RateMultiplier = rateMultiplier.Float64
	job.HoldAmount = holdAmount.Float64
	job.BillingState = MobileVideoBillingState(billingState.String)
	if leaseExpires.Valid {
		value := leaseExpires.Time.UTC()
		job.LeaseExpiresAt = &value
	}
	if clientCancelledAt.Valid {
		value := clientCancelledAt.Time.UTC()
		job.ClientCancelledAt = &value
	}
	if len(refsJSON) > 0 {
		_ = json.Unmarshal(refsJSON, &job.ReferenceAssetIDs)
	}
	if len(snapshotJSON) > 0 {
		var snapshot MobileVideoExecutionSnapshot
		if json.Unmarshal(snapshotJSON, &snapshot) == nil {
			job.ExecutionSnapshot = &snapshot
		}
	}
	if len(errorJSON) > 0 {
		var taskError MobileTaskError
		if json.Unmarshal(errorJSON, &taskError) == nil {
			job.LastError = &taskError
		}
	}
	job.NextPollAt = job.NextPollAt.UTC()
	job.CreatedAt = job.CreatedAt.UTC()
	job.UpdatedAt = job.UpdatedAt.UTC()
	return job, nil
}

func (s *MobileVideoJobService) bind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var out strings.Builder
	position := 1
	for _, char := range query {
		if char == '?' {
			fmt.Fprintf(&out, "$%d", position)
			position++
			continue
		}
		_, _ = out.WriteRune(char)
	}
	return out.String()
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
	seen := make(map[string]struct{}, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func cloneMobileVideoExecutionSnapshot(snapshot *MobileVideoExecutionSnapshot) *MobileVideoExecutionSnapshot {
	if snapshot == nil {
		return nil
	}
	// JSON round-trip avoids sharing nested whitelist, pricing-map, or model
	// routing slices between a stored job and a retry input.
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}
	var cloned MobileVideoExecutionSnapshot
	if json.Unmarshal(payload, &cloned) != nil {
		return nil
	}
	return &cloned
}

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

type MobileVideoTaskStore interface {
	Get(context.Context, int64, string) (*MobileTask, error)
	Transition(context.Context, int64, string, MobileTaskTransitionInput) (*MobileTask, error)
	FinalizeVideoTask(context.Context, int64, string, MobileVideoTaskFinalizeInput) (*MobileTask, error)
}

// MobileVideoWorker keeps upstream execution out of request handlers. It is
// restart-safe because private request data and leases live in Postgres.
type MobileVideoWorker struct {
	store    MobileVideoJobStore
	tasks    MobileVideoTaskStore
	provider MobileVideoProvider
	storage  ImageStorage
	billing  UsageBillingRepository
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
	Billing      UsageBillingRepository
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
	return &MobileVideoWorker{store: store, tasks: tasks, provider: provider, storage: storage, billing: options.Billing, workerID: options.WorkerID, lease: options.Lease, poll: options.PollInterval, now: time.Now}
}

func (w *MobileVideoWorker) Start() {
	if w == nil || w.store == nil || w.tasks == nil || w.provider == nil || w.storage == nil || w.running.Swap(true) {
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

// RunOnce is deterministic for tests and lets an external scheduler execute
// the same durable job lifecycle when an installation chooses not to run it
// in-process.
func (w *MobileVideoWorker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil || w.store == nil || w.tasks == nil || w.provider == nil || w.storage == nil {
		return false, ErrMobileVideoJobStoreUnavailable
	}
	job, err := w.store.ClaimNext(ctx, w.workerID, w.now().UTC(), w.lease)
	if err != nil {
		if !errors.Is(err, ErrMobileVideoJobNotFound) {
			return false, err
		}
		job = nil
	}
	if job != nil {
		return true, w.process(ctx, job)
	}
	releasing, releaseErr := w.store.ClaimFundingRelease(ctx, w.workerID, w.now().UTC(), w.lease)
	if errors.Is(releaseErr, ErrMobileVideoJobNotFound) || releasing == nil {
		return false, nil
	}
	if releaseErr != nil {
		return false, releaseErr
	}
	return true, w.recoverFundingRelease(ctx, releasing)
}

// recoverFundingRelease is the only worker path allowed to inspect a funding
// row. The handler already persisted `releasing`, so retrying the stable
// release ledger key is safe and cannot turn this task into free work.
func (w *MobileVideoWorker) recoverFundingRelease(ctx context.Context, job *MobileVideoJob) error {
	if job == nil || job.State != MobileVideoJobStateFunding || job.BillingState != MobileVideoBillingStateReleasing ||
		!ValidMobileVideoExecutionSnapshot(job.ExecutionSnapshot, job.UserID, job.GroupID, job.ExecutionAPIKeyID) || w.billing == nil {
		return ErrMobileVideoFundingNotReady
	}
	if err := ReleaseMobileVideoBalance(ctx, w.billing, job); err != nil {
		return MobileVideoFundingError("release", err)
	}
	if err := w.store.MarkFundingReleased(ctx, job.TaskID); err != nil {
		return err
	}
	task, err := w.tasks.Get(ctx, job.UserID, job.TaskID)
	if errors.Is(err, ErrMobileTaskNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if task.Status == MobileTaskStatusCancelled || job.ClientCancelledAt != nil {
		return w.store.MarkCancelled(ctx, job.TaskID, "")
	}
	return w.store.DiscardUnfunded(ctx, job.UserID, job.TaskID)
}

func (w *MobileVideoWorker) process(ctx context.Context, job *MobileVideoJob) error {
	task, err := w.tasks.Get(ctx, job.UserID, job.TaskID)
	if errors.Is(err, ErrMobileTaskNotFound) {
		return w.cancelWithoutPublishing(ctx, job)
	}
	if err != nil {
		return err
	}
	// A release is a durable compensation fence. It can be observed after this
	// worker acquired the execution lease when the process lost a state-write
	// acknowledgement. Never call the provider again from that fence: retry the
	// stable release ledger key, then finish the local task instead.
	if job.BillingState == MobileVideoBillingStateReleasing || job.BillingState == MobileVideoBillingStateReleased {
		return w.resumeAcceptedRelease(ctx, job, task)
	}
	if mobileVideoJobRequiresFunding(job) && !ValidMobileVideoExecutionSnapshot(job.ExecutionSnapshot, job.UserID, job.GroupID, job.ExecutionAPIKeyID) {
		return w.fail(ctx, job, &MobileVideoProviderError{Code: "VIDEO_EXECUTION_SNAPSHOT_INVALID", Message: "视频任务执行快照无效", Retryable: false})
	}
	if job.State == MobileVideoJobStateSubmissionUnknown {
		return w.settleSubmissionUnknown(ctx, job, task)
	}
	if task.Status == MobileTaskStatusCancelled && strings.TrimSpace(job.ProviderRequestID) == "" {
		// The cancellation won the race before an upstream request ID exists.
		// Do not call Create; release the accepted hold and end locally.
		return w.cancelWithoutPublishing(ctx, job)
	}

	var result MobileVideoProviderResult
	if strings.TrimSpace(job.ProviderRequestID) == "" {
		result, err = w.callProviderWithHeartbeat(ctx, job, w.provider.Create)
	} else {
		result, err = w.callProviderWithHeartbeat(ctx, job, w.provider.Poll)
	}
	if errors.Is(err, ErrMobileVideoLeaseLost) {
		return nil
	}
	if err != nil {
		if strings.TrimSpace(job.ProviderRequestID) == "" && job.State == MobileVideoJobStateSubmitting {
			return w.store.MarkSubmissionUnknown(ctx, job.TaskID, w.workerID, "视频提交结果无法确认，请勿重复提交")
		}
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
			content, contentErr := w.callProviderWithHeartbeat(ctx, job, w.provider.Content)
			if errors.Is(contentErr, ErrMobileVideoLeaseLost) {
				return nil
			}
			if contentErr != nil {
				return w.fail(ctx, job, contentErr)
			}
			result.Artifact = content.Artifact
			result.ArtifactURL = content.ArtifactURL
			if result.ContentType == "" {
				result.ContentType = content.ContentType
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
	}
	return err
}

// resumeAcceptedRelease completes a previously fenced compensation without
// ever resubmitting or polling the upstream request. A released hold means the
// original attempt cannot continue; non-cancelled callers receive a retryable
// terminal task and must create a new request with a new execution snapshot.
func (w *MobileVideoWorker) resumeAcceptedRelease(ctx context.Context, job *MobileVideoJob, task *MobileTask) error {
	if job == nil || (job.BillingState != MobileVideoBillingStateReleasing && job.BillingState != MobileVideoBillingStateReleased) {
		return ErrMobileVideoFundingNotReady
	}
	if err := w.releaseAcceptedHold(ctx, job); err != nil {
		return err
	}
	if task == nil || task.Status == MobileTaskStatusCancelled || job.ClientCancelledAt != nil {
		return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
	}

	const (
		code    = "VIDEO_RELEASE_RECOVERED"
		message = "视频任务已退款，请重新提交"
	)
	if err := w.store.MarkFailed(ctx, job.TaskID, w.workerID, code, message, true); err != nil {
		return err
	}
	_, err := w.tasks.Transition(ctx, job.UserID, job.TaskID, MobileTaskTransitionInput{
		Status: MobileTaskStatusFailed,
		Error:  &MobileTaskError{Code: code, Message: message, Retryable: true},
	})
	return err
}

type mobileVideoProviderCall func(context.Context, *MobileVideoJob) (MobileVideoProviderResult, error)

func (w *MobileVideoWorker) callProviderWithHeartbeat(ctx context.Context, job *MobileVideoJob, call mobileVideoProviderCall) (MobileVideoProviderResult, error) {
	if w == nil || w.store == nil || job == nil || call == nil {
		return MobileVideoProviderResult{}, ErrMobileVideoJobStoreUnavailable
	}
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopped := make(chan struct{})
	heartbeatErr := make(chan error, 1)
	interval := mobileVideoHeartbeatInterval(w.lease)
	go func() {
		defer close(stopped)
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			select {
			case <-callCtx.Done():
				return
			case <-timer.C:
				if err := w.store.Heartbeat(ctx, job.TaskID, w.workerID, w.now().UTC(), w.lease); err != nil {
					select {
					case heartbeatErr <- err:
					default:
					}
					cancel()
					return
				}
				timer.Reset(interval)
			}
		}
	}()
	result, err := call(callCtx, job)
	cancel()
	<-stopped
	select {
	case heartbeatFailure := <-heartbeatErr:
		return MobileVideoProviderResult{}, fmt.Errorf("%w: %v", ErrMobileVideoLeaseLost, heartbeatFailure)
	default:
		return result, err
	}
}

func mobileVideoHeartbeatInterval(lease time.Duration) time.Duration {
	interval := lease / 3
	if interval <= 0 {
		return time.Millisecond
	}
	if interval > 30*time.Second {
		return 30 * time.Second
	}
	return interval
}

func (w *MobileVideoWorker) complete(ctx context.Context, job *MobileVideoJob, result MobileVideoProviderResult) error {
	if task, err := w.tasks.Get(ctx, job.UserID, job.TaskID); errors.Is(err, ErrMobileTaskNotFound) || (err == nil && task.Status == MobileTaskStatusCancelled) {
		// A local cancellation must never discard a completed provider request.
		// Capture its accepted fixed price, then keep the public task cancelled
		// and do not download/persist an artifact.
		if err := w.captureAcceptedHold(ctx, job); err != nil {
			return err
		}
		return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
	} else if err != nil {
		return err
	}
	data := result.Artifact
	contentType := strings.TrimSpace(result.ContentType)
	if len(data) == 0 && strings.TrimSpace(result.ArtifactURL) != "" {
		var err error
		data, contentType, err = FetchMobileVideoRemoteURL(ctx, result.ArtifactURL)
		if err != nil {
			return w.fail(ctx, job, err)
		}
	}
	if len(data) == 0 {
		return w.fail(ctx, job, ErrMobileVideoJobArtifact)
	}
	if contentType == "" {
		contentType = "video/mp4"
	}
	storageKey := fmt.Sprintf("mobile-video-results/%d/%s%s", job.UserID, job.TaskID, mobileVideoExtension(contentType))
	if _, err := w.storage.Save(ctx, storageKey, contentType, data); err != nil {
		return w.fail(ctx, job, err)
	}
	if err := w.captureAcceptedHold(ctx, job); err != nil {
		if deleter, ok := w.storage.(ImageAssetDeleter); ok {
			_ = deleter.Delete(ctx, storageKey)
		}
		return err
	}
	artifact := MobileTaskArtifact{
		ID: job.TaskID, Kind: "video", Name: "video" + mobileVideoExtension(contentType), ContentType: contentType,
		URL: "/api/v1/mobile/video/jobs/" + url.PathEscape(job.TaskID) + "/content", ByteSize: int64(len(data)),
	}
	_, err := w.tasks.FinalizeVideoTask(ctx, job.UserID, job.TaskID, MobileVideoTaskFinalizeInput{
		LeaseOwner: w.workerID, StorageKey: storageKey, ContentType: contentType, ByteSize: int64(len(data)), Artifact: artifact,
	})
	if errors.Is(err, ErrMobileVideoTaskCancelled) {
		if deleter, ok := w.storage.(ImageAssetDeleter); ok {
			_ = deleter.Delete(ctx, storageKey)
		}
		return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
	}
	return err
}

func (w *MobileVideoWorker) cancelWithoutPublishing(ctx context.Context, job *MobileVideoJob) error {
	if job == nil {
		return ErrMobileVideoJobStoreUnavailable
	}
	if err := w.releaseAcceptedHold(ctx, job); err != nil {
		return err
	}
	return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
}

// settleSubmissionUnknown is deliberately conservative. A transport failure
// after dispatch can leave an upstream task running without a recoverable ID;
// releasing its hold would create an unbounded free-work path. We capture the
// accepted fixed price, mark the public task retryable, and never resubmit the
// ambiguous request automatically.
func (w *MobileVideoWorker) settleSubmissionUnknown(ctx context.Context, job *MobileVideoJob, task *MobileTask) error {
	if err := w.captureAcceptedHold(ctx, job); err != nil {
		return err
	}
	if task == nil || task.Status == MobileTaskStatusCancelled {
		return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
	}
	const code = "VIDEO_SUBMISSION_UNKNOWN"
	const message = "视频提交结果无法确认，请重新提交"
	if err := w.store.MarkFailed(ctx, job.TaskID, w.workerID, code, message, true); err != nil {
		return err
	}
	_, err := w.tasks.Transition(ctx, job.UserID, job.TaskID, MobileTaskTransitionInput{
		Status: MobileTaskStatusFailed,
		Error:  &MobileTaskError{Code: code, Message: message, Retryable: true},
	})
	return err
}

func (w *MobileVideoWorker) captureAcceptedHold(ctx context.Context, job *MobileVideoJob) error {
	if !mobileVideoJobRequiresFunding(job) || job.BillingState == MobileVideoBillingStateCaptured {
		return nil
	}
	if job.BillingState != MobileVideoBillingStateReserved || w.billing == nil {
		return ErrMobileVideoFundingNotReady
	}
	if err := CaptureMobileVideoBalance(ctx, w.billing, job); err != nil {
		return MobileVideoFundingError("capture", err)
	}
	if err := w.store.MarkBillingCaptured(ctx, job.TaskID, w.workerID); err != nil {
		return err
	}
	job.BillingState = MobileVideoBillingStateCaptured
	return nil
}

func (w *MobileVideoWorker) releaseAcceptedHold(ctx context.Context, job *MobileVideoJob) error {
	if !mobileVideoJobRequiresFunding(job) || job.BillingState == MobileVideoBillingStateReleased {
		return nil
	}
	// A row can still be in funding when an old pre-snapshot job is migrated or
	// when the request process failed before Reserve committed. There is no
	// proven hold to release in either case; issuing a release would be able to
	// conflict with a later recovery. Mark the task terminal without creating a
	// synthetic ledger entry.
	if job.BillingState == MobileVideoBillingStateFunding {
		return nil
	}
	if job.BillingState != MobileVideoBillingStateReserved && job.BillingState != MobileVideoBillingStateReleasing || w.billing == nil {
		return ErrMobileVideoFundingNotReady
	}
	if job.BillingState == MobileVideoBillingStateReserved {
		if err := w.store.MarkBillingReleasePending(ctx, job.TaskID, w.workerID); err != nil {
			return err
		}
		job.BillingState = MobileVideoBillingStateReleasing
	}
	if err := ReleaseMobileVideoBalance(ctx, w.billing, job); err != nil {
		return MobileVideoFundingError("release", err)
	}
	if err := w.store.MarkBillingReleased(ctx, job.TaskID, w.workerID); err != nil {
		return err
	}
	job.BillingState = MobileVideoBillingStateReleased
	return nil
}

func mobileVideoExtension(contentType string) string {
	switch {
	case strings.Contains(strings.ToLower(contentType), "webm"):
		return ".webm"
	case strings.Contains(strings.ToLower(contentType), "quicktime"):
		return ".mov"
	default:
		return ".mp4"
	}
}

func (w *MobileVideoWorker) fail(ctx context.Context, job *MobileVideoJob, err error) error {
	providerErr := &MobileVideoProviderError{Code: "VIDEO_PROVIDER_ERROR", Message: "视频生成失败", Retryable: false}
	if typed := new(MobileVideoProviderError); errors.As(err, &typed) {
		providerErr = typed
	}
	return w.failResult(ctx, job, MobileVideoProviderResult{Status: "failed", ErrorCode: providerErr.Code, ErrorMessage: providerErr.Message, Retryable: providerErr.Retryable})
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
	if err := w.releaseAcceptedHold(ctx, job); err != nil {
		return err
	}
	if task, getErr := w.tasks.Get(ctx, job.UserID, job.TaskID); errors.Is(getErr, ErrMobileTaskNotFound) || (getErr == nil && task.Status == MobileTaskStatusCancelled) {
		return w.store.MarkCancelled(ctx, job.TaskID, w.workerID)
	} else if getErr != nil {
		return getErr
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
