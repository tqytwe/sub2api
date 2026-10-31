package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestCredentialOperationEnvelopeFixtures(t *testing.T) {
	raw, err := os.ReadFile("../../../tools/fixtures/credential-operation-errors.json")
	require.NoError(t, err)
	var fixtures []struct {
		Name    string          `json:"name"`
		Payload json.RawMessage `json:"payload"`
		Code    string          `json:"code"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixtures))
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			require.Equal(t, fixture.Code, credentialOperationEnvelope(fixture.Payload))
		})
	}
	for _, raw := range []string{
		`{"error":{"code":"account_deleted","code":"account_deactivated"}}`,
		`{"error":{"code":"account_deleted"},"error":{"code":"account_retired"}}`,
		`{"Error":{"code":"account_deleted"}}`,
		`{"error":{"Code":"account_deleted"}}`,
		`{"error":{"code":"account_deleted"}} {}`,
		strings.Repeat("x", 32769),
	} {
		require.Equal(t, "unknown", credentialOperationEnvelope([]byte(raw)))
	}
}

func TestCredentialOperationWorkerFailurePersistsThroughSafeDTO(t *testing.T) {
	for _, code := range []string{"phone_verification_required", "account_deactivated", "account_deleted", "account_retired", "requires_relogin", "password_rejected", "mfa_required", "security_check", "authorization_denied", "rate_limited", "timeout", "transport_error", "service_error"} {
		t.Run(code, func(t *testing.T) {
			svc, _, repo, updater, _, _ := newReauthTestService("acct-1")
			ctx := context.Background()
			require.NoError(t, repo.UpsertConfig(ctx, reauthEmailConfig(42, "", "encrypted:synthetic")))
			// Reuse the real FailTask ownership boundary; no credentials are applied.
			repo.task = &OpenAIOAuthReauthTaskRecord{ID: 7, AccountID: 42, Status: OpenAIOAuthReauthStatusRunning, WorkerID: "worker-a"}
			raw := "local_protocol_failed stage=web_login reason=" + code + " elapsed_ms=120 exit=1"
			require.Error(t, svc.FailTask(ctx, 7, "worker-b", raw))
			require.NoError(t, svc.FailTask(ctx, 7, "worker-a", raw))
			require.Equal(t, raw, repo.task.Error)
			at := time.Date(2026, 10, 1, 1, 2, 3, 0, time.UTC)
			repo.task.FinishedAt = &at
			view := safeReauthTask(repo.task)
			require.Equal(t, code, view.Failure.Code)
			require.Equal(t, "web_login", view.Failure.Stage)
			require.Equal(t, &at, view.Failure.OccurredAt)
			require.NotEmpty(t, view.Failure.Action)
			require.False(t, updater.applied)
			encoded, err := json.Marshal(view)
			require.NoError(t, err)
			require.Contains(t, string(encoded), `"failure"`)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(encoded, &fields))
			failure, ok := fields["failure"].(map[string]any)
			require.True(t, ok)
			require.Len(t, failure, 5)
		})
	}
}

type credentialRefreshClient struct {
	OpenAIOAuthClient
	err error
}

func (c credentialRefreshClient) RefreshTokenWithClientID(context.Context, string, string, string) (*openai.TokenResponse, error) {
	return nil, c.err
}

type credentialRefreshRecorder struct {
	code  string
	at    time.Time
	calls int
}

func (r *credentialRefreshRecorder) RecordRefreshFailure(_ context.Context, _ int64, code string, at time.Time) error {
	r.code, r.at = code, at
	r.calls++
	return errors.New("synthetic recorder failure")
}

func TestCredentialOperationRefreshPreservesRetryPolicyAndSafeHistory(t *testing.T) {
	for _, rawCode := range []string{"invalid_grant", "invalid_refresh_token", "token_expired", "app_session_terminated", "refresh_token_reused", "refresh_token_invalidated", "invalid_client", "unauthorized_client", "access_denied", "invalid_scope"} {
		err := NewCredentialOperationRefreshError(400, []byte(`{"error":"`+rawCode+`","error_description":"synthetic-private"}`))
		require.True(t, isNonRetryableRefreshError(err), rawCode)
		require.Equal(t, rawCode == "invalid_grant", isInvalidGrantError(err))
		require.NotContains(t, err.Error(), "synthetic-private")
	}
	upstream := NewCredentialOperationRefreshError(400, []byte(`{"error":"account_retired","password":"synthetic-private","cookie":"synthetic-private"}`))
	recorder := &credentialRefreshRecorder{}
	svc := NewOpenAIOAuthService(nil, credentialRefreshClient{err: upstream})
	svc.SetRefreshFailureRecorder(recorder)
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"refresh_token": "synthetic"}}
	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Same(t, upstream, err, "recording must not change refresh success/failure or retry policy")
	require.Equal(t, "account_retired", recorder.code)
	require.False(t, recorder.at.IsZero())
	require.Equal(t, 1, recorder.calls)
	require.Equal(t, StatusActive, account.Status)
	require.Equal(t, "synthetic", account.GetCredential("refresh_token"))
}

func TestCredentialOperationUnknownAndSecretOmission(t *testing.T) {
	for _, secret := range []string{"token=synthetic-private", "password synthetic-private", "totp:synthetic-private", "cookie synthetic-private", "phone synthetic-private", "https://example.test/?private=synthetic-private", strings.Repeat("synthetic-private", 1000)} {
		task := safeReauthTask(&OpenAIOAuthReauthTaskRecord{Status: "failed", Stage: "failed", Error: secret})
		require.Equal(t, "unknown", task.Failure.Category)
		require.Equal(t, "unknown", task.Failure.Stage)
		raw, err := json.Marshal(task)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "synthetic-private")
		require.Less(t, len(raw), 1000)
	}
	require.NotNil(t, safeReauthTask(&OpenAIOAuthReauthTaskRecord{Status: "failed"}).Failure)
	require.Nil(t, safeReauthTask(&OpenAIOAuthReauthTaskRecord{Status: "succeeded"}).Failure)
}

func TestCredentialOperationGuardReadRedactsLegacyProbeDetails(t *testing.T) {
	ctx := context.Background()
	for _, fallback := range []bool{false, true} {
		for _, tc := range []struct {
			name, state, detail, wantDetail, wantCode string
		}{
			{"legacy error", AccountTokenGuardV2ProbeTransient, "Bearer synthetic-private password=synthetic-private", "credential_operation_failed stage=inspection code=unknown", "unknown"},
			{"safe classified error", AccountTokenGuardV2ProbeAuth, "credential_operation_failed stage=inspection code=requires_relogin; synthetic-private", "credential_operation_failed stage=inspection code=requires_relogin", "requires_relogin"},
			{"legacy accepted", AccountTokenGuardV2ProbeOK, "credential accepted token=synthetic-private", "credential accepted", ""},
			{"legacy pending", AccountTokenGuardV2ProbePending, "https://example.test/?token=synthetic-private", "", ""},
		} {
			t.Run(fmt.Sprintf("fallback=%t/%s", fallback, tc.name), func(t *testing.T) {
				reauth, reader, _, _, _, _ := newReauthTestService("acct-1")
				at := time.Date(2026, 10, 31, 1, 2, 3, 0, time.UTC)
				repo := &accountTokenGuardV2TestRepo{record: &AccountTokenGuardV2Record{
					AccountID: 42, ProbeState: tc.state, ProbeDetail: tc.detail, LastProbeAt: &at,
					RefreshFailureCode: "account_retired", LastRefreshFailureAt: &at,
					ReloginFailStreak: 3, LastCountedTaskID: 17, AutoReloginExhausted: true,
				}}
				if fallback {
					reader.account = nil
				}
				svc := NewAccountTokenGuardV2Service(repo, nil, reader, nil, reauth)
				views, err := svc.ListAccounts(ctx)
				require.NoError(t, err)
				require.Len(t, views, 1)
				view := views[0]
				raw, err := json.Marshal(view)
				require.NoError(t, err)
				require.NotContains(t, string(raw), "synthetic-private")
				require.Equal(t, tc.wantDetail, view.ProbeDetail)
				if tc.wantCode == "" {
					require.Nil(t, view.ProbeFailure)
				} else {
					require.NotNil(t, view.ProbeFailure)
					require.Equal(t, tc.wantCode, view.ProbeFailure.Code)
					require.Equal(t, &at, view.ProbeFailure.OccurredAt)
				}
				require.NotNil(t, view.RefreshFailure)
				require.Equal(t, "account_retired", view.RefreshFailure.Code)
				require.Equal(t, &at, view.RefreshFailure.OccurredAt)
				require.Equal(t, 3, view.ReloginFailStreak)
				require.EqualValues(t, 17, view.LastCountedTaskID)
				require.True(t, view.AutoReloginExhausted)
				require.Equal(t, tc.detail, repo.record.ProbeDetail, "read projection must not mutate historical data")
			})
		}
	}
}
