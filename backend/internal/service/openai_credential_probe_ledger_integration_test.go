//go:build integration

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerCredentialProbeFreshLifetime(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"success", 200, `{"models":[{"slug":"gpt-6-sol"}]}`, "succeeded"},
		{"unauthorized", 401, `{"error":{"code":"invalid_token"}}`, "failed"},
		{"policy", 403, `{"error":{"code":"session_blocked_by_cyber_policy"}}`, "failed"},
		{"not_modified", 304, "", "failed"},
		{"invalid_directory", 200, `{"models":null}`, "failed"},
		{"cancelled", 0, "", "cancelled"},
		{"timeout", 0, "", "timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, err := ledger.Begin(context.Background(), "/v1/models", "GET", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), parent)
			require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
			started, stopped := make(chan struct{}), make(chan struct{})
			var durableAtSend atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				var count int
				if err := db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE parent_id=$1 AND execution_state='inflight'`, parent.ID).Scan(&count); err != nil {
					durableAtSend.Store(-1)
				} else {
					durableAtSend.Store(int32(count))
				}
				close(started)
				if test.status == 0 {
					<-request.Context().Done()
					close(stopped)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, test.body)
			}))
			original := chatgptCodexModelsURL
			chatgptCodexModelsURL = server.URL
			t.Cleanup(func() { chatgptCodexModelsURL = original; server.Close() })
			var cancel context.CancelFunc
			if test.name == "timeout" {
				ctx, cancel = context.WithTimeout(ctx, time.Second)
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			t.Cleanup(cancel)
			gateway := &OpenAIGatewayService{}
			account := newCodexModelsTestAccount()
			done := make(chan error, 1)
			go func() {
				_, err := gateway.FetchOpenAIModelsListFresh(ctx, account)
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("probe did not reach disposable upstream")
			}
			if test.name == "cancelled" {
				cancel()
			}
			select {
			case err := <-done:
				if test.want == "succeeded" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
				if test.name == "cancelled" {
					require.ErrorIs(t, err, context.Canceled)
				}
				if test.name == "timeout" {
					require.ErrorIs(t, err, context.DeadlineExceeded)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("probe outlived caller")
			}
			if test.status == 0 {
				select {
				case <-stopped:
				case <-time.After(time.Second):
					t.Fatal("cancelled probe left a detached upstream request")
				}
			}
			require.EqualValues(t, 1, durableAtSend.Load(), "own execution must be durable before sending")
			var state, usage, settlement string
			var ended bool
			var attempts int
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,settlement_state,ended_at IS NOT NULL,attempt_count FROM gateway_requests WHERE parent_id=$1`, parent.ID).Scan(&state, &usage, &settlement, &ended, &attempts))
			require.Equal(t, test.want, state)
			require.Equal(t, "not_applicable", usage, "directory inspection must not bill user inference")
			require.Equal(t, "not_required", settlement)
			require.True(t, ended)
			require.Equal(t, 1, attempts)
			var scheduled, credential int64
			require.NoError(t, db.QueryRow(`SELECT a.account_id,a.credential_account_id FROM gateway_request_attempts a JOIN gateway_requests r ON r.id=a.request_id WHERE r.parent_id=$1`, parent.ID).Scan(&scheduled, &credential))
			require.Equal(t, account.ID, scheduled)
			require.Equal(t, account.ID, credential)
			var userID, keyID int64
			var route string
			require.NoError(t, db.QueryRow(`SELECT user_id,api_key_id,route FROM gateway_requests WHERE parent_id=$1`, parent.ID).Scan(&userID, &keyID, &route))
			require.EqualValues(t, 101, userID)
			require.EqualValues(t, 201, keyID)
			require.Equal(t, "/v1/models", route)
			var billingLinks int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_billing_links b JOIN gateway_requests r ON r.id=b.request_id WHERE r.parent_id=$1`, parent.ID).Scan(&billingLinks))
			require.Zero(t, billingLinks)
			require.NoError(t, db.QueryRow(`SELECT execution_state,attempt_count FROM gateway_requests WHERE id=$1`, parent.ID).Scan(&state, &attempts))
			require.Equal(t, "inflight", state, "probe must not finish its caller")
			require.Zero(t, attempts, "probe sends belong to their own execution")
			require.Zero(t, credentialProbeCacheSize(gateway))
			require.NoError(t, parent.Finish(context.Background(), "succeeded", 200, ""))
		})
	}
}

func TestRequestLedgerCredentialProbeAdmissionFailureStopsSend(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	parent, err := ledger.Begin(context.Background(), "/v1/models", "GET", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), parent)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	_, err = db.Exec(`ALTER TABLE gateway_requests ADD CONSTRAINT synthetic_probe_admission_failure CHECK (kind<>'async_execution') NOT VALID`)
	require.NoError(t, err)
	upstream := newCredentialProbeHTTPFixture(t)
	gateway := &OpenAIGatewayService{}
	response, err := gateway.FetchOpenAIModelsListFresh(ctx, newCodexModelsTestAccount())
	require.Error(t, err)
	require.Nil(t, response)
	require.Zero(t, upstream.calls.Load(), "no provider request without durable admission")
	require.Zero(t, credentialProbeCacheSize(gateway))
	require.NoError(t, parent.Finish(context.Background(), "failed", 503, "request_ledger_unavailable"))
}
