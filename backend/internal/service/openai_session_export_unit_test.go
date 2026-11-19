package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionExportResultStoreIsOneShot(t *testing.T) {
	store := newSessionExportResultStore()
	body := json.RawMessage(`{"accessToken":"token","sessionToken":"session","expires":"2099-01-01T00:00:00Z","user":{"id":"user-1","email":"owner@example.com"},"account":{"plan":"plus"},"unknown":[1,true,null]}`)
	store.put(7, body)
	body[0] = 'x'
	got, ok := store.consume(7)
	require.True(t, ok)
	require.JSONEq(t, `{"accessToken":"token","sessionToken":"session","expires":"2099-01-01T00:00:00Z","user":{"id":"user-1","email":"owner@example.com"},"account":{"plan":"plus"},"unknown":[1,true,null]}`, string(got))
	_, ok = store.consume(7)
	require.False(t, ok)
}

func TestSessionExportResultStoreExpiresEntries(t *testing.T) {
	store := newSessionExportResultStore()
	store.results[9] = sessionExportResult{body: json.RawMessage(`{}`), expiresAt: time.Now().Add(-time.Second)}
	_, ok := store.consume(9)
	require.False(t, ok)
}

func TestCompactSessionExportPayloadKeepsOnlyImportCredentials(t *testing.T) {
	body, err := compactSessionExportPayload("access", "session")
	require.NoError(t, err)
	require.JSONEq(t, `{"accessToken":"access","sessionToken":"session"}`, string(body))
}

func TestNormalizeSessionExportErrorCodeDoesNotStoreWorkerDetails(t *testing.T) {
	require.Equal(t, "totp_rejected", normalizeSessionExportErrorCode("totp_rejected"))
	require.Equal(t, "worker_failed", normalizeSessionExportErrorCode("password=secret; stack trace"))
	require.Equal(t, "worker_failed", normalizeSessionExportErrorCode(""))
}

type sessionExportFlowRepo struct {
	task *OpenAISessionExportTaskRecord
}

func (r *sessionExportFlowRepo) CreateSessionExportTask(_ context.Context, id int64, hash string) (*OpenAISessionExportTaskRecord, error) {
	r.task = &OpenAISessionExportTaskRecord{OpenAISessionExportTask: OpenAISessionExportTask{ID: 7, AccountID: id, Status: "queued", Stage: "queued", CreatedAt: time.Now(), UpdatedAt: time.Now()}, ExpectedCredentialsHash: hash}
	return r.task, nil
}
func (r *sessionExportFlowRepo) GetSessionExportTask(context.Context, int64) (*OpenAISessionExportTaskRecord, error) {
	return r.task, nil
}
func (r *sessionExportFlowRepo) ExpireActiveSessionExportTasks(context.Context) error { return nil }
func (r *sessionExportFlowRepo) ClaimSessionExportTask(_ context.Context, worker string, _ time.Duration) (*OpenAISessionExportTaskRecord, error) {
	r.task.WorkerID = worker
	r.task.Status = "running"
	return r.task, nil
}
func (r *sessionExportFlowRepo) UpdateSessionExportStage(_ context.Context, _ int64, worker, stage string) error {
	if r.task.WorkerID != worker {
		return errors.New("not owner")
	}
	r.task.Stage = stage
	return nil
}
func (r *sessionExportFlowRepo) FinishSessionExportTask(_ context.Context, _ int64, worker, status, code string) error {
	if r.task.WorkerID != worker {
		return errors.New("not owner")
	}
	r.task.Status = status
	r.task.ErrorCode = code
	return nil
}

func TestSessionExportServiceCompletesOneShotAndPreservesCodexCredentials(t *testing.T) {
	for _, scenario := range []string{"success", "wrong_identity", "changed_credentials"} {
		t.Run(scenario, func(t *testing.T) {
			svc, reader, _, updater, _, _ := newReauthTestService("acct-1")
			defer svc.oauth.Stop()
			savePasswordReauthConfig(t, svc)
			repo := &sessionExportFlowRepo{}
			svc.SetSessionExportRepository(repo)
			before := cloneReauthMap(reader.account.Credentials)
			task, err := svc.CreateSessionExportTask(context.Background(), 42)
			require.NoError(t, err)
			absent, err := svc.ClaimSessionExportTask(context.Background(), "worker", []string{"local_worker"})
			require.NoError(t, err)
			require.Nil(t, absent)
			claim, err := svc.ClaimSessionExportTask(context.Background(), "worker", []string{OpenAISessionExportOperation})
			require.NoError(t, err)
			require.Equal(t, "password-secret", claim.Password)
			require.Equal(t, "totp-secret", claim.TOTPSecret)
			userID := "user-1"
			if scenario == "wrong_identity" {
				userID = "other-user"
			}
			if scenario == "changed_credentials" {
				reader.account.Credentials["access_token"] = "rotated"
			}
			body, _ := json.Marshal(map[string]any{"accessToken": "export-access", "sessionToken": "export-session", "expires": time.Now().Add(time.Hour).Format(time.RFC3339), "user": map[string]any{"id": userID, "email": "user@example.com"}, "private_extra": "must-not-export"})
			require.Error(t, svc.SubmitSessionExport(context.Background(), task.ID, "other-worker", body))
			require.NoError(t, svc.SubmitSessionExport(context.Background(), task.ID, "worker", body))
			if scenario != "success" {
				require.Equal(t, "failed", repo.task.Status)
				_, err = svc.DownloadSessionExport(context.Background(), 42, task.ID)
				require.Error(t, err)
				return
			}
			require.Equal(t, "succeeded", repo.task.Status)
			_, err = svc.DownloadSessionExport(context.Background(), 43, task.ID)
			require.Error(t, err)
			downloaded, err := svc.DownloadSessionExport(context.Background(), 42, task.ID)
			require.NoError(t, err)
			require.JSONEq(t, `{"accessToken":"export-access","sessionToken":"export-session"}`, string(downloaded))
			_, err = svc.DownloadSessionExport(context.Background(), 42, task.ID)
			require.Error(t, err)
			require.Equal(t, before, reader.account.Credentials)
			require.False(t, updater.applied)
		})
	}
}
