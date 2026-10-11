//go:build integration

package acceptance

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerHTTPWSEndSnapshotsAdminPagingAndFailure(t *testing.T) {
	r, ledger, db := ledgerFixtureRouter(t)
	handler := &handler.RequestLedgerHandler{Ledger: ledger}
	r.GET("/api/v1/admin/requests/:id/ws-end", handler.WSEndAdmin)
	server := httptest.NewServer(r)
	defer server.Close()
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	a, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	a.Receipt().ObserveWSEnd(requestledger.WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit", Reason: "unexpected_eof", ReasonSource: "finish_cause", JoinedUnfinished: true})
	require.Eventually(t, func() bool {
		var snapshot string
		err := db.QueryRow(`SELECT snapshot::text FROM gateway_request_ws_end_evidence WHERE request_id=$1`, h.ID).Scan(&snapshot)
		return err == nil && snapshot != "{}"
	}, 3*time.Second, 10*time.Millisecond)
	_, err = db.Exec(`INSERT INTO gateway_request_attempts(request_id,attempt_no,account_id,credential_account_id) SELECT $1,n,234,234 FROM generate_series(2,130) n`, h.ID)
	require.NoError(t, err)
	// A corrupt/future extra property is never passed through the whitelist DTO.
	_, err = db.Exec(`UPDATE gateway_request_ws_end_evidence SET snapshot=snapshot||'{"body":"body-secret-sentinel","header":"header-secret-sentinel"}'::jsonb WHERE request_id=$1`, h.ID)
	require.NoError(t, err)
	read := func(user, path string) (int, http.Header, []byte) {
		req, err := http.NewRequest("GET", server.URL+path, nil)
		require.NoError(t, err)
		req.Header.Set("X-Ledger-Test-User", user)
		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, resp.Header, body
	}
	path := "/api/v1/admin/requests/" + h.ID + "/ws-end"
	for _, tc := range []struct {
		user, path string
		status     int
	}{
		{"", path, 401}, {"101", path, 403}, {"102", path, 403},
		{"999", "/api/v1/admin/requests/not-uuid/ws-end", 400},
		{"999", "/api/v1/admin/requests/" + uuid.NewString() + "/ws-end", 404},
		{"999", path + "?after_attempt=-1", 400}, {"999", path + "?after_attempt=no", 400},
		{"999", path + "?after_attempt=2147483648", 400},
	} {
		status, _, body := read(tc.user, tc.path)
		require.Equal(t, tc.status, status)
		require.NotContains(t, string(body), "req_first")
	}
	seen := map[int]bool{}
	cursor := 0
	for {
		status, headers, body := read("999", path+"?after_attempt="+strconv.Itoa(cursor))
		require.Equal(t, 200, status)
		require.Equal(t, "no-store", headers.Get("Cache-Control"))
		require.NotContains(t, string(body), "secret-sentinel")
		var envelope struct {
			Data requestledger.WSEndEvidencePage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &envelope))
		page := envelope.Data
		require.Equal(t, "unknown", page.Completeness)
		require.LessOrEqual(t, len(page.Attempts), requestledger.WSEndEvidencePageSize)
		for _, attempt := range page.Attempts {
			require.False(t, seen[attempt.AttemptNo])
			seen[attempt.AttemptNo] = true
			if attempt.AttemptNo == 1 {
				require.Equal(t, "persisted_snapshot", attempt.Status)
				require.Equal(t, "unexpected_eof", attempt.Snapshot.Reason)
				require.Equal(t, a.ID, *attempt.AttemptID)
				require.Equal(t, "not_observed_in_scope", attempt.Snapshot.Terminal.State)
			} else {
				require.Equal(t, "not_recorded", attempt.Status)
				require.Nil(t, attempt.Snapshot)
			}
		}
		if !page.HasMore {
			require.Nil(t, page.NextAfterAttempt)
			break
		}
		require.NotNil(t, page.NextAfterAttempt)
		cursor = *page.NextAfterAttempt
	}
	require.Len(t, seen, 130)
	_, err = db.Exec(`ALTER TABLE gateway_request_ws_end_evidence RENAME COLUMN snapshot TO fixture_missing_ws_end_diagnostics`)
	require.NoError(t, err)
	status, _, body := read("999", path)
	require.Equal(t, 503, status)
	require.NotContains(t, string(body), "fixture_missing")
	for _, prefix := range []string{"/api/v1/admin/requests/", "/api/v1/requests/"} {
		status, _, _ = read("999", prefix+h.ID+"?view=summary")
		if prefix == "/api/v1/requests/" {
			status, _, _ = read("101", prefix+h.ID+"?view=summary")
		}
		require.Equal(t, 200, status, "independent diagnostic failure must preserve request detail")
	}
}
