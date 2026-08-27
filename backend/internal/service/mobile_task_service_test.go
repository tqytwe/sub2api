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

func TestMobileTaskServiceKeepsOnlyCanonicalPrivateVideoContentPath(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	ctx := context.Background()
	input := MobileVideoJobCreateInput{
		GroupID: 9, ExecutionAPIKeyID: 17, Adapter: MobileVideoAdapterGrok,
		Model: "video-alpha", Prompt: "private prompt", Resolution: "720p", Ratio: "16:9",
		DurationSeconds: 8, ClientRequestID: "video-content-1",
	}
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	task, err := service.CreateVideoTask(ctx, 31, taskInput, input)
	require.NoError(t, err)
	_, err = service.Transition(ctx, 31, task.ID, MobileTaskTransitionInput{Status: MobileTaskStatusRunning})
	require.NoError(t, err)
	completed, err := service.Transition(ctx, 31, task.ID, MobileTaskTransitionInput{
		Status: MobileTaskStatusCompleted,
		Artifacts: []MobileTaskArtifact{
			{ID: task.ID, Kind: "video", URL: "/api/v1/mobile/video/jobs/" + task.ID + "/content"},
			{ID: "bad-relative", Kind: "video", URL: "/api/v1/mobile/video/jobs/" + task.ID + "/content?token=private"},
			{ID: "bad-path", Kind: "video", URL: "/api/v1/mobile/video/jobs/other/content"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/api/v1/mobile/video/jobs/"+task.ID+"/content", completed.Artifacts[0].URL)
	require.Empty(t, completed.Artifacts[1].URL)
	require.Empty(t, completed.Artifacts[2].URL)
}

func TestMobileTaskServiceFinalizeVideoTaskIsAtomic(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	ctx := context.Background()
	input := MobileVideoJobCreateInput{
		GroupID: 9, ExecutionAPIKeyID: 17, Adapter: MobileVideoAdapterGrok,
		Model: "video-alpha", Prompt: "private prompt", Resolution: "720p", Ratio: "16:9",
		DurationSeconds: 8, ClientRequestID: "video-finalize-atomic",
	}
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	task, err := service.CreateVideoTask(ctx, 31, taskInput, input)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE mobile_video_jobs
		SET state = 'polling', lease_owner = 'worker', provider_request_id = 'provider-1'
		WHERE task_id = ?`, task.ID)
	require.NoError(t, err)

	// The private job transition must roll back when the public projection
	// cannot be committed. A queued task is a valid fast-provider completion
	// path and must become running/completed inside the same transaction.
	_, err = db.Exec(`CREATE TRIGGER mobile_task_finalization_failure
		BEFORE UPDATE ON mobile_tasks BEGIN
			SELECT RAISE(ABORT, 'public task transition failed');
		END`)
	require.NoError(t, err)
	_, err = service.FinalizeVideoTask(ctx, 31, task.ID, MobileVideoTaskFinalizeInput{
		LeaseOwner: "worker", StorageKey: "mobile-video-results/31/" + task.ID + ".mp4",
		ContentType: "video/mp4", ByteSize: 10,
		Artifact: MobileTaskArtifact{ID: task.ID, Kind: "video", URL: "/api/v1/mobile/video/jobs/" + task.ID + "/content", ByteSize: 10},
	})
	require.Error(t, err)
	var jobState, leaseOwner string
	require.NoError(t, db.QueryRow(`SELECT state, COALESCE(lease_owner, '') FROM mobile_video_jobs WHERE task_id = ?`, task.ID).Scan(&jobState, &leaseOwner))
	require.Equal(t, "polling", jobState)
	require.Equal(t, "worker", leaseOwner)
	queued, err := service.Get(ctx, 31, task.ID)
	require.NoError(t, err)
	require.Equal(t, MobileTaskStatusQueued, queued.Status)

	_, err = db.Exec(`DROP TRIGGER mobile_task_finalization_failure`)
	require.NoError(t, err)
	completed, err := service.FinalizeVideoTask(ctx, 31, task.ID, MobileVideoTaskFinalizeInput{
		LeaseOwner: "worker", StorageKey: "mobile-video-results/31/" + task.ID + ".mp4",
		ContentType: "video/mp4", ByteSize: 10,
		Artifact: MobileTaskArtifact{ID: task.ID, Kind: "video", URL: "/api/v1/mobile/video/jobs/" + task.ID + "/content", ByteSize: 10},
	})
	require.NoError(t, err)
	require.Equal(t, MobileTaskStatusCompleted, completed.Status)
	require.NotNil(t, completed.StartedAt)
	require.NotNil(t, completed.FinishedAt)

	var storageKey string
	require.NoError(t, db.QueryRow(`SELECT state, COALESCE(artifact_storage_key, '') FROM mobile_video_jobs WHERE task_id = ?`, task.ID).Scan(&jobState, &storageKey))
	require.Equal(t, "completed", jobState)
	require.Equal(t, "mobile-video-results/31/"+task.ID+".mp4", storageKey)
}

func TestMobileTaskServiceFinalizeVideoTaskHonorsCancellation(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	ctx := context.Background()
	input := MobileVideoJobCreateInput{
		GroupID: 9, ExecutionAPIKeyID: 17, Adapter: MobileVideoAdapterGrok,
		Model: "video-alpha", Prompt: "private prompt", Resolution: "720p", Ratio: "16:9",
		DurationSeconds: 8, ClientRequestID: "video-finalize-cancelled",
	}
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	task, err := service.CreateVideoTask(ctx, 32, taskInput, input)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE mobile_video_jobs
		SET state = 'polling', lease_owner = 'worker', provider_request_id = 'provider-1'
		WHERE task_id = ?`, task.ID)
	require.NoError(t, err)
	_, err = service.Cancel(ctx, 32, task.ID)
	require.NoError(t, err)

	_, err = service.FinalizeVideoTask(ctx, 32, task.ID, MobileVideoTaskFinalizeInput{
		LeaseOwner: "worker", StorageKey: "mobile-video-results/32/" + task.ID + ".mp4",
		ContentType: "video/mp4", ByteSize: 10,
		Artifact: MobileTaskArtifact{ID: task.ID, Kind: "video", URL: "/api/v1/mobile/video/jobs/" + task.ID + "/content", ByteSize: 10},
	})
	require.ErrorIs(t, err, ErrMobileVideoTaskCancelled)
	var jobState string
	require.NoError(t, db.QueryRow(`SELECT state FROM mobile_video_jobs WHERE task_id = ?`, task.ID).Scan(&jobState))
	// A cancellation wins the public projection, but the private provider task
	// remains pollable so the worker can capture a late completion or release a
	// provider failure. It owns the eventual private terminal state.
	require.Equal(t, "polling", jobState)
	cancelled, err := service.Get(ctx, 32, task.ID)
	require.NoError(t, err)
	require.Equal(t, MobileTaskStatusCancelled, cancelled.Status)
}

func TestMobileTaskServiceRequiresDedicatedVideoCreateAndRetry(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	input := MobileVideoJobCreateInput{
		GroupID: 9, ExecutionAPIKeyID: 17, Adapter: MobileVideoAdapterGrok,
		Model: "video-alpha", Prompt: "private prompt", Resolution: "720p", Ratio: "16:9",
		DurationSeconds: 8, ClientRequestID: "video-source-1",
	}
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)

	_, err = service.Create(ctx, 32, taskInput)
	require.ErrorIs(t, err, ErrMobileTaskVideoRequiresDedicatedEndpoint)
	var taskCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = ?", 32).Scan(&taskCount))
	require.Zero(t, taskCount)

	installMobileVideoJobsTestTable(t, db)
	source, err := service.CreateVideoTask(ctx, 32, taskInput, input)
	require.NoError(t, err)
	_, err = service.Transition(ctx, 32, source.ID, MobileTaskTransitionInput{Status: MobileTaskStatusFailed, Error: &MobileTaskError{
		Code: "UPSTREAM_TIMEOUT", Message: "timeout", Retryable: true,
	}})
	require.NoError(t, err)

	_, err = service.Retry(ctx, 32, source.ID, "video-generic-retry")
	require.ErrorIs(t, err, ErrMobileTaskVideoRequiresDedicatedEndpoint)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = ?", 32).Scan(&taskCount))
	require.Equal(t, 1, taskCount)

	retryInput := input
	retryInput.ClientRequestID = "video-dedicated-retry"
	dedicatedRetry, err := service.RetryVideoTask(ctx, 32, source.ID, retryInput.ClientRequestID, retryInput)
	require.NoError(t, err)
	require.Equal(t, source.ID, dedicatedRetry.RetryOf)
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

func TestMobileTaskServiceCreateVideoTaskIsAtomicAndIdempotent(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	ctx := context.Background()
	input := MobileVideoJobCreateInput{
		GroupID: 9, ExecutionAPIKeyID: 17, Adapter: MobileVideoAdapterGrok,
		Model: "video-alpha", Prompt: "private prompt", Resolution: "720p", Ratio: "16:9",
		DurationSeconds: 8, ClientRequestID: "video-create-1",
	}
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)

	// The private table is deliberately absent. The task insert must roll back,
	// otherwise a queued public task could never be executed.
	_, err = service.CreateVideoTask(ctx, 50, taskInput, input)
	require.Error(t, err)
	var taskCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = ?", 50).Scan(&taskCount))
	require.Zero(t, taskCount)

	installMobileVideoJobsTestTable(t, db)
	created, err := service.CreateVideoTask(ctx, 50, taskInput, input)
	require.NoError(t, err)
	require.Equal(t, MobileTaskKindVideo, created.Kind)
	var jobCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_video_jobs WHERE task_id = ? AND user_id = ?", created.ID, 50).Scan(&jobCount))
	require.Equal(t, 1, jobCount)

	// Replaying the same client request must reuse the durable job. A newly
	// issued execution key must not turn a retrying HTTP request into a second
	// upstream submission or charge.
	replayedInput := input
	replayedInput.ExecutionAPIKeyID = 18
	replayed, err := service.CreateVideoTask(ctx, 50, taskInput, replayedInput)
	require.NoError(t, err)
	require.Equal(t, created.ID, replayed.ID)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = ?", 50).Scan(&taskCount))
	require.Equal(t, 1, taskCount)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_video_jobs WHERE user_id = ?", 50).Scan(&jobCount))
	require.Equal(t, 1, jobCount)

	changed := replayedInput
	changed.Prompt = "different private prompt"
	changedTaskInput, err := BuildMobileVideoTaskCreateInput(changed)
	require.NoError(t, err)
	_, err = service.CreateVideoTask(ctx, 50, changedTaskInput, changed)
	require.ErrorIs(t, err, ErrMobileTaskClientRequestConflict)
}

func TestMobileTaskServiceRetryVideoTaskIsAtomicAndIdempotent(t *testing.T) {
	service, db := newMobileTaskServiceTestStore(t)
	installMobileVideoJobsTestTable(t, db)
	ctx := context.Background()
	input := MobileVideoJobCreateInput{
		GroupID: 9, ExecutionAPIKeyID: 17, Adapter: MobileVideoAdapterGrok,
		Model: "video-alpha", Prompt: "private prompt", Resolution: "720p", Ratio: "16:9",
		DurationSeconds: 8, ClientRequestID: "video-source-1",
	}
	taskInput, err := BuildMobileVideoTaskCreateInput(input)
	require.NoError(t, err)
	source, err := service.CreateVideoTask(ctx, 51, taskInput, input)
	require.NoError(t, err)
	_, err = service.Transition(ctx, 51, source.ID, MobileTaskTransitionInput{Status: MobileTaskStatusFailed, Error: &MobileTaskError{
		Code: "UPSTREAM_TIMEOUT", Message: "timeout", Retryable: true,
	}})
	require.NoError(t, err)

	retryInput := input
	retryInput.ClientRequestID = "video-retry-1"
	retryInput.ExecutionAPIKeyID = 19
	retry, err := service.RetryVideoTask(ctx, 51, source.ID, retryInput.ClientRequestID, retryInput)
	require.NoError(t, err)
	require.Equal(t, source.ID, retry.RetryOf)
	require.Equal(t, MobileTaskStatusQueued, retry.Status)
	expectedRetryTask, err := BuildMobileVideoTaskCreateInput(retryInput)
	require.NoError(t, err)
	require.Equal(t, expectedRetryTask.Resource, retry.Resource)

	replayed, err := service.RetryVideoTask(ctx, 51, source.ID, retryInput.ClientRequestID, retryInput)
	require.NoError(t, err)
	require.Equal(t, retry.ID, replayed.ID)
	var taskCount, jobCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = ?", 51).Scan(&taskCount))
	require.Equal(t, 2, taskCount)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_video_jobs WHERE user_id = ?", 51).Scan(&jobCount))
	require.Equal(t, 2, jobCount)

	// Retry creation must use the same transaction. Once the private table is
	// unavailable, no additional public retry row may escape the rollback.
	_, err = db.Exec("DROP TABLE mobile_video_jobs")
	require.NoError(t, err)
	failedRetryInput := input
	failedRetryInput.ClientRequestID = "video-retry-private-store-down"
	_, err = service.RetryVideoTask(ctx, 51, source.ID, failedRetryInput.ClientRequestID, failedRetryInput)
	require.Error(t, err)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM mobile_tasks WHERE user_id = ?", 51).Scan(&taskCount))
	require.Equal(t, 2, taskCount)
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

func installMobileVideoJobsTestTable(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE mobile_video_jobs (
		task_id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		group_id INTEGER NOT NULL,
		execution_api_key_id INTEGER NOT NULL,
		adapter TEXT NOT NULL,
		model TEXT NOT NULL,
		prompt TEXT NOT NULL,
		resolution TEXT NOT NULL,
		ratio TEXT NULL,
		duration_seconds INTEGER NOT NULL,
		generate_audio BOOLEAN NOT NULL,
		watermark BOOLEAN NOT NULL,
		reference_asset_ids JSON NOT NULL,
		execution_snapshot JSON NULL,
		unit_price_usd REAL NULL,
		rate_multiplier REAL NULL,
		hold_amount REAL NULL,
		billing_state TEXT NOT NULL DEFAULT 'funding',
		provider_request_id TEXT NULL,
		provider_status TEXT NULL,
		state TEXT NOT NULL,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		next_poll_at TIMESTAMP NOT NULL,
		lease_owner TEXT NULL,
		lease_expires_at TIMESTAMP NULL,
		artifact_storage_key TEXT NULL,
		artifact_content_type TEXT NULL,
		artifact_byte_size INTEGER NULL,
		client_cancelled_at TIMESTAMP NULL,
		last_error JSON NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)`)
	require.NoError(t, err)
}

func uuidTestName(name string) string {
	name = strings.NewReplacer("/", "_", " ", "_").Replace(name)
	return name
}
