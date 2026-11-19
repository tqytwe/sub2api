//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionExportRepositoryOwnershipCompletionAndRestart(t *testing.T) {
	ctx, _, cfg := totpRotationFixture(t)
	repo := NewOpenAISessionExportRepository(integrationDB)
	task, err := repo.CreateSessionExportTask(ctx, cfg.AccountID, "snapshot")
	require.NoError(t, err)
	_, err = repo.CreateSessionExportTask(ctx, cfg.AccountID, "snapshot")
	require.Error(t, err, "an account may have only one active export")
	claimed, err := repo.ClaimSessionExportTask(ctx, "owner", time.Minute)
	require.NoError(t, err)
	require.Equal(t, task.ID, claimed.ID)
	require.Equal(t, "snapshot", claimed.ExpectedCredentialsHash)
	require.Error(t, repo.UpdateSessionExportStage(ctx, task.ID, "other-worker", "session_request"))
	require.Error(t, repo.FinishSessionExportTask(ctx, task.ID, "other-worker", "succeeded", ""))
	require.NoError(t, repo.UpdateSessionExportStage(ctx, task.ID, "owner", "identity_verification"))
	require.NoError(t, repo.FinishSessionExportTask(ctx, task.ID, "owner", "succeeded", ""))
	completed, err := repo.GetSessionExportTask(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", completed.Status)
	require.NotNil(t, completed.FinishedAt)
	require.Error(t, repo.FinishSessionExportTask(ctx, task.ID, "owner", "failed", "worker_failed"))
	next, err := repo.CreateSessionExportTask(ctx, cfg.AccountID, "new-snapshot")
	require.NoError(t, err)
	require.NoError(t, repo.ExpireActiveSessionExportTasks(ctx))
	expired, err := repo.GetSessionExportTask(ctx, next.ID)
	require.NoError(t, err)
	require.Equal(t, "expired", expired.Status)
	require.Equal(t, "api_restarted", expired.ErrorCode)
}
