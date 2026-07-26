package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type MobileTaskKind string

const (
	MobileTaskKindChat  MobileTaskKind = "chat"
	MobileTaskKindImage MobileTaskKind = "image"
	MobileTaskKindFile  MobileTaskKind = "file"
)

type MobileTaskStatus string

const (
	MobileTaskStatusQueued    MobileTaskStatus = "queued"
	MobileTaskStatusRunning   MobileTaskStatus = "running"
	MobileTaskStatusStreaming MobileTaskStatus = "streaming"
	MobileTaskStatusCompleted MobileTaskStatus = "completed"
	MobileTaskStatusPartial   MobileTaskStatus = "partial"
	MobileTaskStatusFailed    MobileTaskStatus = "failed"
	MobileTaskStatusCancelled MobileTaskStatus = "cancelled"
)

const MobileTaskProtocolVersion = 1

var (
	ErrMobileTaskInvalidID         = errors.New("mobile task id is invalid")
	ErrMobileTaskInvalidKind       = errors.New("mobile task kind is invalid")
	ErrMobileTaskInvalidStatus     = errors.New("mobile task status is invalid")
	ErrMobileTaskInvalidOperation  = errors.New("mobile task operation is invalid")
	ErrMobileTaskInvalidRequestID  = errors.New("mobile task client request id is invalid")
	ErrMobileTaskInvalidProgress   = errors.New("mobile task progress is invalid")
	ErrMobileTaskInvalidResource   = errors.New("mobile task resource is invalid")
	ErrMobileTaskInvalidArtifact   = errors.New("mobile task artifact is invalid")
	ErrMobileTaskInvalidTransition = errors.New("mobile task status transition is invalid")
	ErrMobileTaskNotCancellable    = errors.New("mobile task is not cancellable")
	ErrMobileTaskNotRetryable      = errors.New("mobile task is not retryable")
	ErrMobileTaskRetryIDReuse      = errors.New("mobile task retry must use a new id")
)

// MobileTaskError is safe to expose to clients. Details must not contain
// prompts, chat content, credentials, or other sensitive request data.
type MobileTaskError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

// MobileTaskResource links the protocol projection to the subsystem that owns
// execution, for example an Image Studio job or a file-processing job.
type MobileTaskResource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// MobileTaskArtifact describes an output without embedding its content.
type MobileTaskArtifact struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	URL         string         `json:"url,omitempty"`
	ByteSize    int64          `json:"byte_size,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// MobileTask is the shared client-facing projection for chat, image, and file
// work. It is deliberately execution- and storage-agnostic.
type MobileTask struct {
	ID              string               `json:"id"`
	Kind            MobileTaskKind       `json:"kind"`
	Operation       string               `json:"operation"`
	Status          MobileTaskStatus     `json:"status"`
	Progress        int                  `json:"progress"`
	Cancellable     bool                 `json:"cancellable"`
	Retryable       bool                 `json:"retryable"`
	ParentTaskID    string               `json:"parent_task_id,omitempty"`
	RetryOf         string               `json:"retry_of,omitempty"`
	ClientRequestID string               `json:"client_request_id"`
	Resource        *MobileTaskResource  `json:"resource,omitempty"`
	Artifacts       []MobileTaskArtifact `json:"artifacts"`
	Error           *MobileTaskError     `json:"error,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	StartedAt       *time.Time           `json:"started_at,omitempty"`
	FinishedAt      *time.Time           `json:"finished_at,omitempty"`
	Version         int                  `json:"version"`
}

func IsValidMobileTaskKind(kind MobileTaskKind) bool {
	switch kind {
	case MobileTaskKindChat, MobileTaskKindImage, MobileTaskKindFile:
		return true
	default:
		return false
	}
}

func IsValidMobileTaskStatus(status MobileTaskStatus) bool {
	switch status {
	case MobileTaskStatusQueued,
		MobileTaskStatusRunning,
		MobileTaskStatusStreaming,
		MobileTaskStatusCompleted,
		MobileTaskStatusPartial,
		MobileTaskStatusFailed,
		MobileTaskStatusCancelled:
		return true
	default:
		return false
	}
}

func IsTerminalMobileTaskStatus(status MobileTaskStatus) bool {
	switch status {
	case MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed, MobileTaskStatusCancelled:
		return true
	default:
		return false
	}
}

func CanTransitionMobileTask(from, to MobileTaskStatus) bool {
	if !IsValidMobileTaskStatus(from) || !IsValidMobileTaskStatus(to) || from == to || IsTerminalMobileTaskStatus(from) {
		return false
	}

	allowed := map[MobileTaskStatus]map[MobileTaskStatus]struct{}{
		MobileTaskStatusQueued: {
			MobileTaskStatusRunning:   {},
			MobileTaskStatusFailed:    {},
			MobileTaskStatusCancelled: {},
		},
		MobileTaskStatusRunning: {
			MobileTaskStatusStreaming: {},
			MobileTaskStatusCompleted: {},
			MobileTaskStatusPartial:   {},
			MobileTaskStatusFailed:    {},
			MobileTaskStatusCancelled: {},
		},
		MobileTaskStatusStreaming: {
			MobileTaskStatusCompleted: {},
			MobileTaskStatusPartial:   {},
			MobileTaskStatusFailed:    {},
			MobileTaskStatusCancelled: {},
		},
	}

	_, ok := allowed[from][to]
	return ok
}

func CanCancelMobileTask(task MobileTask) bool {
	if !IsValidMobileTaskStatus(task.Status) {
		return false
	}
	return task.Status == MobileTaskStatusQueued || task.Status == MobileTaskStatusRunning || task.Status == MobileTaskStatusStreaming
}

func CanRetryMobileTask(task MobileTask) bool {
	switch task.Status {
	case MobileTaskStatusCancelled, MobileTaskStatusPartial:
		return true
	case MobileTaskStatusFailed:
		return task.Error != nil && task.Error.Retryable
	default:
		return false
	}
}

func NewMobileTask(id string, kind MobileTaskKind, operation, clientRequestID string, createdAt time.Time) (MobileTask, error) {
	task := MobileTask{
		ID:              strings.TrimSpace(id),
		Kind:            kind,
		Operation:       strings.TrimSpace(operation),
		Status:          MobileTaskStatusQueued,
		Progress:        0,
		Cancellable:     true,
		Retryable:       false,
		ClientRequestID: strings.TrimSpace(clientRequestID),
		Artifacts:       make([]MobileTaskArtifact, 0),
		CreatedAt:       createdAt.UTC(),
		Version:         MobileTaskProtocolVersion,
	}
	if err := ValidateMobileTask(task); err != nil {
		return MobileTask{}, err
	}
	return task, nil
}

func ValidateMobileTask(task MobileTask) error {
	if strings.TrimSpace(task.ID) == "" {
		return ErrMobileTaskInvalidID
	}
	if !IsValidMobileTaskKind(task.Kind) {
		return fmt.Errorf("%w: %q", ErrMobileTaskInvalidKind, task.Kind)
	}
	if !IsValidMobileTaskStatus(task.Status) {
		return fmt.Errorf("%w: %q", ErrMobileTaskInvalidStatus, task.Status)
	}
	if strings.TrimSpace(task.Operation) == "" || len(task.Operation) > 100 {
		return ErrMobileTaskInvalidOperation
	}
	if strings.TrimSpace(task.ClientRequestID) == "" || len(task.ClientRequestID) > 128 {
		return ErrMobileTaskInvalidRequestID
	}
	if task.Progress < 0 || task.Progress > 100 {
		return ErrMobileTaskInvalidProgress
	}
	if task.ParentTaskID == task.ID || task.RetryOf == task.ID {
		return ErrMobileTaskRetryIDReuse
	}
	if task.Resource != nil && (strings.TrimSpace(task.Resource.Type) == "" || strings.TrimSpace(task.Resource.ID) == "") {
		return ErrMobileTaskInvalidResource
	}
	for _, artifact := range task.Artifacts {
		if strings.TrimSpace(artifact.ID) == "" || strings.TrimSpace(artifact.Kind) == "" || artifact.ByteSize < 0 {
			return ErrMobileTaskInvalidArtifact
		}
	}
	return nil
}

func TransitionMobileTask(task MobileTask, to MobileTaskStatus, at time.Time) (MobileTask, error) {
	if err := ValidateMobileTask(task); err != nil {
		return MobileTask{}, err
	}
	if !IsValidMobileTaskStatus(to) {
		return MobileTask{}, fmt.Errorf("%w: %q", ErrMobileTaskInvalidStatus, to)
	}
	if !CanTransitionMobileTask(task.Status, to) {
		return MobileTask{}, fmt.Errorf("%w: %s -> %s", ErrMobileTaskInvalidTransition, task.Status, to)
	}

	transitionedAt := at.UTC()
	task.Status = to
	if (to == MobileTaskStatusRunning || to == MobileTaskStatusStreaming) && task.StartedAt == nil {
		task.StartedAt = &transitionedAt
	}
	if IsTerminalMobileTaskStatus(to) {
		task.FinishedAt = &transitionedAt
		if to == MobileTaskStatusCompleted {
			task.Progress = 100
		}
	}
	task.Cancellable = CanCancelMobileTask(task)
	task.Retryable = CanRetryMobileTask(task)
	return task, nil
}

// CancelMobileTask is idempotent for an already-cancelled task. Other terminal
// states cannot be changed to cancelled.
func CancelMobileTask(task MobileTask, at time.Time) (MobileTask, error) {
	if err := ValidateMobileTask(task); err != nil {
		return MobileTask{}, err
	}
	if task.Status == MobileTaskStatusCancelled {
		return task, nil
	}
	if !CanCancelMobileTask(task) {
		return MobileTask{}, fmt.Errorf("%w: status %s", ErrMobileTaskNotCancellable, task.Status)
	}
	return TransitionMobileTask(task, MobileTaskStatusCancelled, at)
}

// RetryMobileTask creates a new queued task. The source task remains unchanged,
// and RetryOf always points to the immediate task being retried.
func RetryMobileTask(source MobileTask, newID, clientRequestID string, createdAt time.Time) (MobileTask, error) {
	if err := ValidateMobileTask(source); err != nil {
		return MobileTask{}, err
	}
	newID = strings.TrimSpace(newID)
	if newID == "" {
		return MobileTask{}, ErrMobileTaskInvalidID
	}
	if newID == source.ID {
		return MobileTask{}, ErrMobileTaskRetryIDReuse
	}
	if !CanRetryMobileTask(source) {
		return MobileTask{}, fmt.Errorf("%w: status %s", ErrMobileTaskNotRetryable, source.Status)
	}

	retry, err := NewMobileTask(newID, source.Kind, source.Operation, clientRequestID, createdAt)
	if err != nil {
		return MobileTask{}, err
	}
	retry.ParentTaskID = source.ParentTaskID
	if retry.ParentTaskID == "" {
		retry.ParentTaskID = source.ID
	}
	retry.RetryOf = source.ID
	return retry, nil
}
