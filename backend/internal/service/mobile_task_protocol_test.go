//go:build unit

package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMobileTaskKindAndStatusValidation(t *testing.T) {
	for _, kind := range []MobileTaskKind{MobileTaskKindChat, MobileTaskKindImage, MobileTaskKindFile, MobileTaskKindVideo} {
		require.True(t, IsValidMobileTaskKind(kind))
	}

	for _, status := range []MobileTaskStatus{
		MobileTaskStatusQueued, MobileTaskStatusRunning, MobileTaskStatusStreaming,
		MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed,
		MobileTaskStatusCancelled,
	} {
		require.True(t, IsValidMobileTaskStatus(status))
	}
	require.False(t, IsValidMobileTaskStatus("processing"))
}

func TestCanTransitionMobileTask(t *testing.T) {
	allowed := map[MobileTaskStatus][]MobileTaskStatus{
		MobileTaskStatusQueued: {
			MobileTaskStatusRunning, MobileTaskStatusFailed, MobileTaskStatusCancelled,
		},
		MobileTaskStatusRunning: {
			MobileTaskStatusStreaming, MobileTaskStatusCompleted, MobileTaskStatusPartial,
			MobileTaskStatusFailed, MobileTaskStatusCancelled,
		},
		MobileTaskStatusStreaming: {
			MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed,
			MobileTaskStatusCancelled,
		},
	}
	all := []MobileTaskStatus{
		MobileTaskStatusQueued, MobileTaskStatusRunning, MobileTaskStatusStreaming,
		MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed,
		MobileTaskStatusCancelled,
	}

	for _, from := range all {
		for _, to := range all {
			want := false
			for _, candidate := range allowed[from] {
				want = want || candidate == to
			}
			require.Equalf(t, want, CanTransitionMobileTask(from, to), "%s -> %s", from, to)
		}
	}
	require.False(t, CanTransitionMobileTask("unknown", MobileTaskStatusRunning))
	require.False(t, CanTransitionMobileTask(MobileTaskStatusQueued, "unknown"))
}

func TestMobileTaskTerminalStatusesAreIrreversible(t *testing.T) {
	terminal := []MobileTaskStatus{
		MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed, MobileTaskStatusCancelled,
	}
	all := []MobileTaskStatus{
		MobileTaskStatusQueued, MobileTaskStatusRunning, MobileTaskStatusStreaming,
		MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed,
		MobileTaskStatusCancelled,
	}
	for _, from := range terminal {
		require.True(t, IsTerminalMobileTaskStatus(from))
		for _, to := range all {
			require.Falsef(t, CanTransitionMobileTask(from, to), "%s -> %s", from, to)
		}
	}
	require.False(t, IsTerminalMobileTaskStatus(MobileTaskStatusStreaming))
}

func TestNewMobileTask(t *testing.T) {
	createdAt := time.Date(2026, 7, 26, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	task, err := NewMobileTask(" task-1 ", MobileTaskKindChat, " chat_completion ", " request-1 ", createdAt)
	require.NoError(t, err)
	require.Equal(t, "task-1", task.ID)
	require.Equal(t, "chat_completion", task.Operation)
	require.Equal(t, "request-1", task.ClientRequestID)
	require.Equal(t, MobileTaskStatusQueued, task.Status)
	require.True(t, task.Cancellable)
	require.False(t, task.Retryable)
	require.Empty(t, task.Artifacts)
	require.NotNil(t, task.Artifacts)
	require.Equal(t, createdAt.UTC(), task.CreatedAt)
	require.Equal(t, MobileTaskProtocolVersion, task.Version)
}

func TestNewMobileTaskRejectsInvalidIdentity(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		id        string
		kind      MobileTaskKind
		operation string
		requestID string
		wantErr   error
	}{
		{name: "id", kind: MobileTaskKindChat, operation: "chat", requestID: "req", wantErr: ErrMobileTaskInvalidID},
		{name: "kind", id: "id", kind: "unknown", operation: "chat", requestID: "req", wantErr: ErrMobileTaskInvalidKind},
		{name: "operation", id: "id", kind: MobileTaskKindChat, requestID: "req", wantErr: ErrMobileTaskInvalidOperation},
		{name: "request id", id: "id", kind: MobileTaskKindChat, operation: "chat", wantErr: ErrMobileTaskInvalidRequestID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMobileTask(tt.id, tt.kind, tt.operation, tt.requestID, now)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestNewMobileTaskAcceptsVideo(t *testing.T) {
	task, err := NewMobileTask("video-task-1", MobileTaskKindVideo, "video.generate", "video-request-1", time.Now())
	require.NoError(t, err)
	require.Equal(t, MobileTaskKindVideo, task.Kind)
}

func TestValidateMobileTaskDTOs(t *testing.T) {
	task := mustNewMobileTask(t, MobileTaskKindImage)
	task.Resource = &MobileTaskResource{Type: "image_studio_job", ID: "job-1"}
	task.Artifacts = []MobileTaskArtifact{{ID: "asset-1", Kind: "image", ContentType: "image/png", ByteSize: 42}}
	require.NoError(t, ValidateMobileTask(task))

	invalidResource := task
	invalidResource.Resource = &MobileTaskResource{Type: "image_studio_job"}
	require.ErrorIs(t, ValidateMobileTask(invalidResource), ErrMobileTaskInvalidResource)

	invalidArtifact := task
	invalidArtifact.Artifacts = []MobileTaskArtifact{{ID: "asset-1", ByteSize: -1}}
	require.ErrorIs(t, ValidateMobileTask(invalidArtifact), ErrMobileTaskInvalidArtifact)

	invalidProgress := task
	invalidProgress.Progress = 101
	require.ErrorIs(t, ValidateMobileTask(invalidProgress), ErrMobileTaskInvalidProgress)

	selfReference := task
	selfReference.RetryOf = task.ID
	require.ErrorIs(t, ValidateMobileTask(selfReference), ErrMobileTaskRetryIDReuse)
}

func TestTransitionMobileTaskUpdatesLifecycleFields(t *testing.T) {
	createdAt := time.Date(2026, 7, 26, 1, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(time.Second)
	finishedAt := startedAt.Add(time.Second)
	task, err := NewMobileTask("task-1", MobileTaskKindChat, "chat_completion", "request-1", createdAt)
	require.NoError(t, err)

	running, err := TransitionMobileTask(task, MobileTaskStatusRunning, startedAt)
	require.NoError(t, err)
	require.Equal(t, &startedAt, running.StartedAt)
	require.True(t, running.Cancellable)

	completed, err := TransitionMobileTask(running, MobileTaskStatusCompleted, finishedAt)
	require.NoError(t, err)
	require.Equal(t, 100, completed.Progress)
	require.Equal(t, &finishedAt, completed.FinishedAt)
	require.False(t, completed.Cancellable)
	require.False(t, completed.Retryable)

	_, err = TransitionMobileTask(completed, MobileTaskStatusRunning, finishedAt.Add(time.Second))
	require.ErrorIs(t, err, ErrMobileTaskInvalidTransition)
	_, err = TransitionMobileTask(task, "unknown", startedAt)
	require.ErrorIs(t, err, ErrMobileTaskInvalidStatus)
}

func TestCancelMobileTask(t *testing.T) {
	now := time.Now().UTC()
	for _, status := range []MobileTaskStatus{MobileTaskStatusQueued, MobileTaskStatusRunning, MobileTaskStatusStreaming} {
		t.Run(string(status), func(t *testing.T) {
			task := mustTaskInStatus(t, status)
			cancelled, err := CancelMobileTask(task, now)
			require.NoError(t, err)
			require.Equal(t, MobileTaskStatusCancelled, cancelled.Status)
			require.Equal(t, &now, cancelled.FinishedAt)
			require.False(t, cancelled.Cancellable)
			require.True(t, cancelled.Retryable)

			again, err := CancelMobileTask(cancelled, now.Add(time.Second))
			require.NoError(t, err)
			require.Equal(t, cancelled, again)
		})
	}

	for _, status := range []MobileTaskStatus{MobileTaskStatusCompleted, MobileTaskStatusPartial, MobileTaskStatusFailed} {
		task := mustTaskInStatus(t, status)
		_, err := CancelMobileTask(task, now)
		require.ErrorIs(t, err, ErrMobileTaskNotCancellable)
	}
}

func TestCanRetryMobileTask(t *testing.T) {
	completed := mustTaskInStatus(t, MobileTaskStatusCompleted)
	require.False(t, CanRetryMobileTask(completed))

	partial := mustTaskInStatus(t, MobileTaskStatusPartial)
	require.True(t, CanRetryMobileTask(partial))

	cancelled := mustTaskInStatus(t, MobileTaskStatusCancelled)
	require.True(t, CanRetryMobileTask(cancelled))

	failed := mustTaskInStatus(t, MobileTaskStatusFailed)
	failed.Error = &MobileTaskError{Code: "UPSTREAM_TIMEOUT", Message: "upstream timeout", Retryable: false}
	require.False(t, CanRetryMobileTask(failed))
	failed.Error.Retryable = true
	require.True(t, CanRetryMobileTask(failed))
}

func TestRetryMobileTaskCreatesCleanQueuedTaskAndPreservesSource(t *testing.T) {
	createdAt := time.Date(2026, 7, 26, 2, 0, 0, 0, time.UTC)
	source := mustTaskInStatus(t, MobileTaskStatusPartial)
	source.Resource = &MobileTaskResource{Type: "image_studio_job", ID: "job-old"}
	source.Artifacts = []MobileTaskArtifact{{ID: "asset-old", Kind: "image"}}
	source.Error = &MobileTaskError{Code: "PARTIAL", Message: "one item failed", Retryable: true}
	snapshot := source

	retry, err := RetryMobileTask(source, "task-2", "request-2", createdAt)
	require.NoError(t, err)
	require.Equal(t, snapshot, source, "retry must not mutate the source task")
	require.Equal(t, "task-2", retry.ID)
	require.Equal(t, source.ID, retry.ParentTaskID)
	require.Equal(t, source.ID, retry.RetryOf)
	require.Equal(t, source.Kind, retry.Kind)
	require.Equal(t, source.Operation, retry.Operation)
	require.Equal(t, MobileTaskStatusQueued, retry.Status)
	require.True(t, retry.Cancellable)
	require.False(t, retry.Retryable)
	require.Nil(t, retry.Resource)
	require.Empty(t, retry.Artifacts)
	require.Nil(t, retry.Error)
	require.Nil(t, retry.StartedAt)
	require.Nil(t, retry.FinishedAt)
}

func TestRetryMobileTaskMaintainsRetryChain(t *testing.T) {
	source := mustTaskInStatus(t, MobileTaskStatusCancelled)
	source.ParentTaskID = "root-task"
	retry, err := RetryMobileTask(source, "task-3", "request-3", time.Now())
	require.NoError(t, err)
	require.Equal(t, "root-task", retry.ParentTaskID)
	require.Equal(t, source.ID, retry.RetryOf)
}

func TestRetryMobileTaskRejectsInvalidRetry(t *testing.T) {
	source := mustTaskInStatus(t, MobileTaskStatusCompleted)
	_, err := RetryMobileTask(source, "task-2", "request-2", time.Now())
	require.ErrorIs(t, err, ErrMobileTaskNotRetryable)

	source = mustTaskInStatus(t, MobileTaskStatusCancelled)
	_, err = RetryMobileTask(source, source.ID, "request-2", time.Now())
	require.ErrorIs(t, err, ErrMobileTaskRetryIDReuse)
	_, err = RetryMobileTask(source, "", "request-2", time.Now())
	require.ErrorIs(t, err, ErrMobileTaskInvalidID)
	_, err = RetryMobileTask(source, "task-2", "", time.Now())
	require.ErrorIs(t, err, ErrMobileTaskInvalidRequestID)
}

func TestMobileTaskDTOJSONShape(t *testing.T) {
	task := mustNewMobileTask(t, MobileTaskKindFile)
	task.Resource = &MobileTaskResource{Type: "file_job", ID: "file-1"}
	task.Artifacts = []MobileTaskArtifact{{ID: "summary-1", Kind: "document", Name: "摘要.txt"}}
	task.Error = &MobileTaskError{Code: "WARNING", Message: "部分内容不可读", Retryable: true}

	data, err := json.Marshal(task)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, "file", payload["kind"])
	require.Equal(t, "queued", payload["status"])
	require.Equal(t, float64(MobileTaskProtocolVersion), payload["version"])
	require.Contains(t, payload, "resource")
	require.Contains(t, payload, "artifacts")
	require.Contains(t, payload, "error")
}

func mustNewMobileTask(t *testing.T, kind MobileTaskKind) MobileTask {
	t.Helper()
	task, err := NewMobileTask("task-1", kind, string(kind)+"_operation", "request-1", time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return task
}

func mustTaskInStatus(t *testing.T, status MobileTaskStatus) MobileTask {
	t.Helper()
	task := mustNewMobileTask(t, MobileTaskKindChat)
	now := task.CreatedAt.Add(time.Second)
	if status == MobileTaskStatusQueued {
		return task
	}
	if status == MobileTaskStatusFailed {
		task.Error = &MobileTaskError{Code: "FAILED", Message: "failed", Retryable: true}
	}
	var err error
	task, err = TransitionMobileTask(task, MobileTaskStatusRunning, now)
	require.NoError(t, err)
	if status == MobileTaskStatusRunning {
		return task
	}
	if status == MobileTaskStatusStreaming {
		task, err = TransitionMobileTask(task, status, now.Add(time.Second))
	} else {
		task, err = TransitionMobileTask(task, status, now.Add(time.Second))
	}
	require.NoError(t, err)
	return task
}

func TestMobileTaskErrorsSupportErrorsIs(t *testing.T) {
	task := mustNewMobileTask(t, MobileTaskKindChat)
	_, err := TransitionMobileTask(task, MobileTaskStatusCompleted, time.Now())
	require.True(t, errors.Is(err, ErrMobileTaskInvalidTransition))
}
