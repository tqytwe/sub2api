package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMobileTaskServiceCreateListAndUserIsolation(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	ctx := context.Background()

	created := make([]*MobileTask, 0, 3)
	for index, kind := range []MobileTaskKind{MobileTaskKindChat, MobileTaskKindImage, MobileTaskKindFile} {
		task, err := service.Create(ctx, 10, MobileTaskCreateInput{
			Kind:            kind,
			Operation:       string(kind) + "_operation",
			ClientRequestID: fmt.Sprintf("request-%d", index),
		})
		require.NoError(t, err)
		created = append(created, task)
	}

	again, err := service.Create(ctx, 10, MobileTaskCreateInput{Kind: MobileTaskKindChat, Operation: "chat_operation", ClientRequestID: "request-0"})
	require.NoError(t, err)
	require.Equal(t, created[0].ID, again.ID)
	_, err = service.Create(ctx, 10, MobileTaskCreateInput{Kind: MobileTaskKindImage, Operation: "image_operation", ClientRequestID: "request-0"})
	require.ErrorIs(t, err, ErrMobileTaskClientRequestConflict)

	page, err := service.List(ctx, 10, MobileTaskListFilter{Kind: MobileTaskKindImage, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, MobileTaskKindImage, page.Items[0].Kind)

	foreignPage, err := service.List(ctx, 11, MobileTaskListFilter{})
	require.NoError(t, err)
	require.Zero(t, foreignPage.Total)
	_, err = service.Get(ctx, 11, created[0].ID)
	require.ErrorIs(t, err, ErrMobileTaskNotFound)

	var stored int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = 10").Scan(&stored))
	require.Equal(t, 3, stored)
}

func TestMobileTaskServiceCancelIsTerminalAndIdempotent(t *testing.T) {
	service, _ := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	task, err := service.Create(ctx, 20, MobileTaskCreateInput{Kind: MobileTaskKindChat, Operation: "chat_completion", ClientRequestID: "cancel-1"})
	require.NoError(t, err)

	cancelled, err := service.Cancel(ctx, 20, task.ID)
	require.NoError(t, err)
	require.Equal(t, MobileTaskStatusCancelled, cancelled.Status)
	require.NotNil(t, cancelled.FinishedAt)

	again, err := service.Cancel(ctx, 20, task.ID)
	require.NoError(t, err)
	require.Equal(t, cancelled.FinishedAt, again.FinishedAt)
	_, err = service.Transition(ctx, 20, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusCompleted})
	require.ErrorIs(t, err, ErrMobileTaskInvalidTransition)

	completed, err := service.Create(ctx, 20, MobileTaskCreateInput{Kind: MobileTaskKindFile, Operation: "document_summary", ClientRequestID: "complete-1"})
	require.NoError(t, err)
	completed, err = service.Transition(ctx, 20, completed.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning})
	require.NoError(t, err)
	completed, err = service.Transition(ctx, 20, completed.ID, MobileTaskTransitionInput{Status: MobileTaskStatusCompleted})
	require.NoError(t, err)
	_, err = service.Cancel(ctx, 20, completed.ID)
	require.ErrorIs(t, err, ErrMobileTaskNotCancellable)
	terminalAgain, err := service.Transition(ctx, 20, completed.ID, MobileTaskTransitionInput{Status: MobileTaskStatusCompleted})
	require.NoError(t, err)
	require.Equal(t, completed.FinishedAt, terminalAgain.FinishedAt)
}

func TestMobileTaskServiceAllowsMonotonicProgressWithinRunningState(t *testing.T) {
	service, _ := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	task, err := service.Create(ctx, 21, MobileTaskCreateInput{Kind: MobileTaskKindFile, Operation: "document_summary", ClientRequestID: "progress-1"})
	require.NoError(t, err)
	task, err = service.Transition(ctx, 21, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning})
	require.NoError(t, err)
	progress := 35
	task, err = service.Transition(ctx, 21, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning, Progress: &progress})
	require.NoError(t, err)
	require.Equal(t, 35, task.Progress)
	progress = 20
	_, err = service.Transition(ctx, 21, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning, Progress: &progress})
	require.ErrorIs(t, err, ErrMobileTaskInvalidProgress)
}

func TestMobileTaskServiceRejectsProgressRegressionFromStaleWriter(t *testing.T) {
	service, _ := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	task, err := service.Create(ctx, 22, MobileTaskCreateInput{Kind: MobileTaskKindFile, Operation: "document_summary", ClientRequestID: "progress-race"})
	require.NoError(t, err)
	task, err = service.Transition(ctx, 22, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning})
	require.NoError(t, err)

	stale := *task
	high := stale
	high.Progress = 80
	_, err = service.persistTransition(ctx, 22, stale, high)
	require.NoError(t, err)

	low := stale
	low.Progress = 20
	_, err = service.persistTransition(ctx, 22, stale, low)
	require.ErrorIs(t, err, ErrMobileTaskInvalidProgress)

	latest, err := service.Get(ctx, 22, task.ID)
	require.NoError(t, err)
	require.Equal(t, 80, latest.Progress)
}

func TestMobileTaskServiceRetryAndSanitizedProjection(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	task, err := service.Create(ctx, 30, MobileTaskCreateInput{
		Kind:            MobileTaskKindImage,
		Operation:       "image_generation",
		ClientRequestID: "image-1",
		Resource:        &MobileTaskResource{Type: "image_job", ID: "job-1"},
	})
	require.NoError(t, err)
	_, err = service.Transition(ctx, 30, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning})
	require.NoError(t, err)
	failed, err := service.Transition(ctx, 30, task.ID, MobileTaskTransitionInput{
		Status: MobileTaskStatusFailed,
		Error: &MobileTaskError{
			Code:      "UPSTREAM_TIMEOUT",
			Message:   "access_token=top-secret prompt=private-prompt 请求超时",
			Retryable: true,
			Details: map[string]any{
				"api_key": "sk-secret",
				"prompt":  "private prompt",
				"status":  504,
			},
		},
		Artifacts: []MobileTaskArtifact{{
			ID:       "asset-1",
			Kind:     "image",
			Name:     "token=private image.png",
			URL:      "https://cdn.example.com/image.png?token=secret#private",
			ByteSize: 100,
			Metadata: map[string]any{"prompt": "private prompt", "width": 1024},
		}},
	})
	require.NoError(t, err)
	require.True(t, failed.Retryable)
	require.NotContains(t, failed.Error.Message, "top-secret")
	require.NotContains(t, failed.Error.Message, "private-prompt")
	require.Equal(t, "***", failed.Error.Details["api_key"])
	require.Equal(t, "***", failed.Error.Details["prompt"])
	require.Equal(t, "https://cdn.example.com/image.png", failed.Artifacts[0].URL)
	require.NotContains(t, failed.Artifacts[0].Name, "private")
	require.Equal(t, "***", failed.Artifacts[0].Metadata["prompt"])

	var storedError, storedArtifacts string
	require.NoError(t, db.QueryRow("SELECT error, artifacts FROM mobile_tasks WHERE id = ?", task.ID).Scan(&storedError, &storedArtifacts))
	require.NotContains(t, storedError, "top-secret")
	require.NotContains(t, storedError, "private prompt")
	require.NotContains(t, storedArtifacts, "?token=secret")

	retry, err := service.Retry(ctx, 30, task.ID, "image-retry-1")
	require.NoError(t, err)
	require.NotEqual(t, task.ID, retry.ID)
	require.Equal(t, task.ID, retry.RetryOf)
	require.Equal(t, task.ID, retry.ParentTaskID)
	require.Equal(t, MobileTaskStatusQueued, retry.Status)
	require.Empty(t, retry.Artifacts)
	require.Nil(t, retry.Error)

	retryAgain, err := service.Retry(ctx, 30, task.ID, "image-retry-1")
	require.NoError(t, err)
	require.Equal(t, retry.ID, retryAgain.ID)
	_, err = service.Retry(ctx, 31, task.ID, "foreign-retry")
	require.ErrorIs(t, err, ErrMobileTaskNotFound)
}

func TestMobileTaskServiceSoftDeleteHidesTask(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	task, err := service.Create(ctx, 40, MobileTaskCreateInput{
		Kind:            MobileTaskKindImage,
		Operation:       "image_generation",
		ClientRequestID: "delete-1",
	})
	require.NoError(t, err)

	result, err := service.Delete(ctx, 40, task.ID)
	require.NoError(t, err)
	require.True(t, result.Deleted)
	require.Equal(t, task.ID, result.ID)
	require.False(t, result.DeletedAt.IsZero())

	_, err = service.Get(ctx, 40, task.ID)
	require.ErrorIs(t, err, ErrMobileTaskNotFound)
	page, err := service.List(ctx, 40, MobileTaskListFilter{Kind: MobileTaskKindImage})
	require.NoError(t, err)
	require.Zero(t, page.Total)

	var deletedAt sql.NullTime
	require.NoError(t, db.QueryRow("SELECT deleted_at FROM mobile_tasks WHERE id = ?", task.ID).Scan(&deletedAt))
	require.True(t, deletedAt.Valid)
	_, err = service.Delete(ctx, 40, task.ID)
	require.ErrorIs(t, err, ErrMobileTaskNotFound)
}

func newMobileTaskServiceTestStore(t *testing.T) (*MobileTaskService, *sql.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:mobile_task_service_%s?mode=memory&cache=shared", uuidTestName(t.Name()))
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE mobile_tasks (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		kind TEXT NOT NULL,
		operation TEXT NOT NULL,
		status TEXT NOT NULL,
		progress INTEGER NOT NULL,
		parent_task_id TEXT NULL,
		retry_of TEXT NULL,
		client_request_id TEXT NOT NULL,
		resource JSON NULL,
		artifacts JSON NOT NULL,
		error JSON NULL,
		protocol_version INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		started_at TIMESTAMP NULL,
		finished_at TIMESTAMP NULL,
		deleted_at TIMESTAMP NULL,
		UNIQUE(user_id, client_request_id)
	)`)
	require.NoError(t, err)
	service := NewMobileTaskService(db, "sqlite")
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	return service, db
}

func uuidTestName(name string) string {
	name = strings.NewReplacer("/", "_", " ", "_").Replace(name)
	return name
}
