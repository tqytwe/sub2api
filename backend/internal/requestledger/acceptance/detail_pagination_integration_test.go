//go:build integration

package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerHTTPDetailPagingContract(t *testing.T) {
	r, ledger, db := ledgerFixtureRouter(t)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	_, err = db.Exec(`INSERT INTO gateway_request_attempts(request_id,attempt_no,account_id,credential_account_id)
 SELECT $1,n,234,22 FROM generate_series(1,105) n`, h.ID)
	require.NoError(t, err)
	read := func(user, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Ledger-Test-User", user)
		r.ServeHTTP(w, req)
		return w
	}
	path := "/api/v1/requests/" + h.ID
	t.Run("shared GET timezone parameter preserves summary pages and legacy", func(t *testing.T) {
		for _, query := range []string{"view=summary&timezone=Asia%2FShanghai", "section=attempts&timezone=UTC", "section=billing&page_size=20&timezone=UTC", "section=wallet&timezone=UTC"} {
			require.Equal(t, 200, read("101", path+"?"+query).Code, query)
		}
		require.Equal(t, 409, read("101", path+"?timezone=UTC").Code)
		small, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
		require.NoError(t, err)
		require.NoError(t, requestledger.BindIdentity(requestledger.WithHandle(context.Background(), small), 101, 201))
		legacy := read("101", "/api/v1/requests/"+small.ID+"?timezone=UTC")
		require.Equal(t, 200, legacy.Code)
		for _, collection := range []string{"attempts", "billing", "wallet"} {
			require.Contains(t, legacy.Body.String(), `"`+collection+`":[]`)
		}
	})
	t.Run("summary does not read or return child collections", func(t *testing.T) {
		w := read("101", path+"?view=summary")
		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), h.ID)
		for _, field := range []string{`"attempts"`, `"billing"`, `"wallet"`, `"diagnostics"`} {
			require.NotContains(t, w.Body.String(), field)
		}
	})
	t.Run("legacy is explicit instead of silently incomplete", func(t *testing.T) {
		w := read("101", path)
		require.Equal(t, 409, w.Code)
		require.Contains(t, w.Body.String(), `"reason":"PAGINATION_REQUIRED"`)
		require.NotContains(t, w.Body.String(), `"attempts"`)
	})
	t.Run("bounded attempt pages traverse every stable row once", func(t *testing.T) {
		cursor := ""
		seen := make(map[int]bool)
		for page := 0; page < 10; page++ {
			w := read("101", path+"?section=attempts&page_size=20&cursor="+cursor)
			require.Equal(t, 200, w.Code)
			var envelope struct {
				Data struct {
					Items    []requestledger.AttemptRecord `json:"items"`
					Next     string                        `json:"next_cursor"`
					HasMore  bool                          `json:"has_more"`
					PageSize int                           `json:"page_size"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			require.Equal(t, 20, envelope.Data.PageSize)
			require.LessOrEqual(t, len(envelope.Data.Items), 20)
			for _, a := range envelope.Data.Items {
				require.False(t, seen[a.Number], "duplicate attempt %d", a.Number)
				seen[a.Number] = true
				require.Nil(t, a.AccountID)
				require.Nil(t, a.CredentialAccountID)
			}
			require.Equal(t, envelope.Data.Next != "", envelope.Data.HasMore)
			cursor = envelope.Data.Next
			if cursor == "" {
				break
			}
		}
		require.Len(t, seen, 105)
	})
	t.Run("invalid parameters never fall back to unbounded legacy", func(t *testing.T) {
		for _, query := range []string{"section=bad", "section=attempts&page_size=101", "section=attempts&page_size=0", "section=attempts&cursor=bad", "view=bad", "view=summary&section=attempts", "cursor=bad", "page_size=20", "view=%zz", "section=attempts;page_size=200"} {
			require.Equal(t, 400, read("101", path+"?"+query).Code, query)
		}
	})
	t.Run("every section retains authentication and ownership", func(t *testing.T) {
		for _, section := range []string{"attempts", "billing", "wallet", "diagnostics"} {
			query := "?section=" + section
			require.Equal(t, 401, read("", path+query).Code, section)
			require.Equal(t, 403, read("101", fmt.Sprintf("/api/v1/admin/requests/%s%s", h.ID, query)).Code, section)
			if section != "diagnostics" {
				require.Equal(t, 404, read("102", path+query).Code, section)
			} else {
				require.Equal(t, 403, read("101", path+query).Code)
			}
		}
	})
}

func readDetailSection[T any](t *testing.T, f *batchImageFixture, user, id, section, cursor string, size int) requestledger.DetailPage[T] {
	t.Helper()
	prefix := "/api/v1/requests/"
	if user == "999" {
		prefix = "/api/v1/admin/requests/"
	}
	path := fmt.Sprintf("%s%s?section=%s&page_size=%d&cursor=%s", prefix, id, section, size, url.QueryEscape(cursor))
	status, body := batchImageHTTPRead(t, f.server, user, path)
	require.Equal(t, 200, status, body)
	var envelope struct {
		Data requestledger.DetailPage[T] `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &envelope))
	require.LessOrEqual(t, len(envelope.Data.Items), size)
	require.Equal(t, size, envelope.Data.PageSize)
	require.Equal(t, "live", envelope.Data.Consistency)
	require.Equal(t, "new_records_late_evidence_and_state_changes", envelope.Data.RefreshHint)
	require.Equal(t, envelope.Data.NextCursor != "", envelope.Data.HasMore)
	return envelope.Data
}

func TestRequestLedgerHTTPBillingWalletSourcePaging(t *testing.T) {
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	_, err := f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_billing_links(request_id,billing_request_id,api_key_id,request_fingerprint,subscription_id,created_at)
 SELECT $1,'fixture_link_'||lpad(n::text,4,'0'),201,'fixture_unverified',n,'2026-01-01' FROM generate_series(1,45) n;
`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_tasks(request_id,task_kind,task_ref,user_id,api_key_id,created_at)
 SELECT $1,'image_batch','000_empty_'||lpad(n::text,4,'0'),101,201,'2026-01-01' FROM generate_series(1,41) n`, s.RequestID)
	require.NoError(t, err)
	before := f.balance(t)
	for _, user := range []string{"101", "999"} {
		seen := map[string]bool{}
		cursor := ""
		emptyIntermediate := false
		for n := 0; n < 30; n++ {
			p := readDetailSection[requestledger.BillingReference](t, f, user, s.RequestID, "billing", cursor, 7)
			if len(p.Items) == 0 && p.HasMore {
				emptyIntermediate = true
			}
			for _, ref := range p.Items {
				key := "batch"
				if ref.SubscriptionID != nil {
					key = fmt.Sprint(*ref.SubscriptionID)
					require.Nil(t, ref.BilledCost)
				} else {
					require.Equal(t, &s.ID, ref.BatchID)
					require.False(t, ref.Applied)
					require.Equal(t, 6.0, *ref.BilledCost)
				}
				require.False(t, seen[key], "duplicate billing reference %s", key)
				seen[key] = true
			}
			cursor = p.NextCursor
			if cursor == "" {
				break
			}
		}
		require.Empty(t, cursor, "bounded fixture traversal must terminate")
		require.True(t, emptyIntermediate, "invalid candidates must advance an empty intermediate page")
		require.Len(t, seen, 46)
		walletSeen := map[int64]bool{}
		for n := 0; n < 30; n++ {
			p := readDetailSection[requestledger.WalletReference](t, f, user, s.RequestID, "wallet", cursor, 7)
			for _, ref := range p.Items {
				require.False(t, walletSeen[ref.ID])
				walletSeen[ref.ID] = true
			}
			cursor = p.NextCursor
			if cursor == "" {
				break
			}
		}
		require.Empty(t, cursor)
		require.Len(t, walletSeen, 2)
	}
	status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
	require.Equal(t, 409, status, body)
	require.Contains(t, body, "PAGINATION_REQUIRED")
	require.Equal(t, before, f.balance(t), "pagination is read-only")
	first := readDetailSection[requestledger.BillingReference](t, f, "101", s.RequestID, "billing", "", 7)
	for _, suffix := range []string{"?section=wallet&cursor=", "?section=diagnostics&cursor="} {
		status, _ = batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID+suffix+first.NextCursor)
		if suffix == "?section=diagnostics&cursor=" {
			require.Equal(t, 403, status)
		} else {
			require.Equal(t, 400, status)
		}
	}
	status, _ = batchImageHTTPRead(t, f.server, "102", "/api/v1/requests/"+s.RequestID+"?section=billing&cursor="+first.NextCursor)
	require.Equal(t, 404, status)
	status, _ = batchImageHTTPRead(t, f.server, "999", "/api/v1/admin/requests/"+s.RequestID+"?section=billing&cursor="+first.NextCursor)
	require.Equal(t, 400, status, "cursor is bound to viewer and role")
}

func TestRequestLedgerHTTPRefreshFindsLateProofBehindCursor(t *testing.T) {
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	_, err := f.db.Exec(`INSERT INTO gateway_request_tasks(request_id,task_kind,task_ref,user_id,api_key_id) VALUES($1,'image_batch','zzzz_empty',101,201)`, s.RequestID)
	require.NoError(t, err)
	billing := readDetailSection[requestledger.BillingReference](t, f, "101", s.RequestID, "billing", "", 1)
	require.Empty(t, billing.Items)
	require.True(t, billing.HasMore)
	wallet := readDetailSection[requestledger.WalletReference](t, f, "101", s.RequestID, "wallet", "", 2)
	require.Len(t, wallet.Items, 1)
	require.Equal(t, "image_balance_hold", wallet.Items[0].Operation)
	require.True(t, wallet.HasMore)
	// Actual capture and usage commit after their already-scanned task/operation.
	// No generated output or upstream call is involved in this fixture.
	_, err = f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	before := f.balance(t)
	next := readDetailSection[requestledger.BillingReference](t, f, "101", s.RequestID, "billing", billing.NextCursor, 1)
	require.Empty(t, next.Items)
	require.False(t, next.HasMore)
	nextWallet := readDetailSection[requestledger.WalletReference](t, f, "101", s.RequestID, "wallet", wallet.NextCursor, 2)
	require.Empty(t, nextWallet.Items, "late capture is behind the live cursor")
	refreshed := readDetailSection[requestledger.BillingReference](t, f, "101", s.RequestID, "billing", "", 1)
	require.Len(t, refreshed.Items, 1)
	require.Equal(t, s.ID, *refreshed.Items[0].BatchID)
	require.Equal(t, 6.0, *refreshed.Items[0].BilledCost)
	refreshedWallet := readDetailSection[requestledger.WalletReference](t, f, "101", s.RequestID, "wallet", "", 2)
	require.Len(t, refreshedWallet.Items, 2)
	require.Equal(t, "image_balance_capture", refreshedWallet.Items[1].Operation)
	require.Equal(t, before, f.balance(t), "refresh must not replay capture")
	var count int
	require.NoError(t, f.db.QueryRow(`SELECT count(*) FROM balance_transactions WHERE user_id=101 AND source_id=$1 AND source_type='image_balance_capture'`, s.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestRequestLedgerHTTPLateCommitAndCursorIsolation(t *testing.T) {
	f := newBatchImageFixture(t)
	h, err := f.ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	require.NoError(t, requestledger.BindIdentity(requestledger.WithHandle(context.Background(), h), 101, 201))
	_, err = f.db.Exec(`INSERT INTO gateway_request_billing_links(request_id,billing_request_id,api_key_id,request_fingerprint,subscription_id,created_at)
 VALUES($1,'100-visible',201,'fixture',100,'2026-01-01'),($1,'200-visible',201,'fixture',200,'2026-01-01')`, h.ID)
	require.NoError(t, err)
	tx, err := f.db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO gateway_request_billing_links(request_id,billing_request_id,api_key_id,request_fingerprint,subscription_id,created_at)
 VALUES($1,'000-late',201,'fixture',1,'2026-01-01')`, h.ID)
	require.NoError(t, err)
	first := readDetailSection[requestledger.BillingReference](t, f, "101", h.ID, "billing", "", 1)
	require.EqualValues(t, 100, *first.Items[0].SubscriptionID)
	require.NoError(t, tx.Commit())
	next := readDetailSection[requestledger.BillingReference](t, f, "101", h.ID, "billing", first.NextCursor, 1)
	require.EqualValues(t, 200, *next.Items[0].SubscriptionID)
	require.False(t, next.HasMore)
	refresh := readDetailSection[requestledger.BillingReference](t, f, "101", h.ID, "billing", "", 1)
	require.EqualValues(t, 1, *refresh.Items[0].SubscriptionID)
	other, err := f.ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	require.NoError(t, requestledger.BindIdentity(requestledger.WithHandle(context.Background(), other), 101, 999))
	status, _ := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+other.ID+"?section=billing&cursor="+first.NextCursor)
	require.Equal(t, 400, status, "same-owner request under another key cannot reuse a cursor")
}

func TestRequestLedgerHTTPPagedEvidenceIsolationAndFailure(t *testing.T) {
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	_, err := f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_attempts(request_id,attempt_no,account_id,credential_account_id)
 SELECT $1,n,234,22 FROM generate_series(1,25) n`, s.RequestID)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO gateway_request_attempt_evidence(request_id,attempt_no,attempt_id,upstream_request_id,upstream_response_id)
 SELECT request_id,attempt_no,gen_random_uuid(),'page_req_'||attempt_no,'page_resp_'||attempt_no FROM gateway_request_attempts WHERE request_id=$1`, s.RequestID)
	require.NoError(t, err)
	first := readDetailSection[requestledger.AttemptRecord](t, f, "999", s.RequestID, "attempts", "", 20)
	second := readDetailSection[requestledger.AttemptRecord](t, f, "999", s.RequestID, "attempts", first.NextCursor, 20)
	require.Len(t, second.Items, 5)
	for i, a := range second.Items {
		require.Equal(t, 21+i, a.Number)
		require.Equal(t, fmt.Sprintf("page_req_%d", 21+i), *a.UpstreamRequestID)
		require.EqualValues(t, 22, *a.CredentialAccountID)
	}
	user := readDetailSection[requestledger.AttemptRecord](t, f, "101", s.RequestID, "attempts", "", 20)
	for _, a := range user.Items {
		require.NotNil(t, a.AttemptID)
		require.Nil(t, a.UpstreamRequestID)
		require.Nil(t, a.UpstreamResponseID)
		require.Nil(t, a.AccountID)
		require.Nil(t, a.CredentialAccountID)
	}
	_, err = f.db.Exec(`INSERT INTO gateway_request_ops_links(request_id,attempt_no,ops_error_log_id)
 SELECT $1,n%3,n FROM generate_series(1,106) n`, s.RequestID)
	require.NoError(t, err)
	cursor := ""
	seen := map[string]bool{}
	for n := 0; n < 20; n++ {
		page := readDetailSection[requestledger.DiagnosticReference](t, f, "999", s.RequestID, "diagnostics", cursor, 7)
		for _, d := range page.Items {
			key := fmt.Sprintf("%d:%d", d.AttemptNo, d.OpsErrorLogID)
			require.False(t, seen[key])
			seen[key] = true
			require.False(t, d.Available)
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	require.Empty(t, cursor)
	require.Len(t, seen, 106)
	status, body := batchImageHTTPRead(t, f.server, "999", "/api/v1/admin/requests/"+s.RequestID)
	require.Equal(t, 409, status, body)
	require.Contains(t, body, "PAGINATION_REQUIRED")
	// One optional metadata read fails; core page, summary and original settled
	// billing remain independently readable, with no inferred zero price.
	_, err = f.db.Exec(`ALTER TABLE gateway_request_attempt_evidence RENAME TO fixture_paged_evidence_offline`)
	require.NoError(t, err)
	defer func() {
		_, restoreErr := f.db.Exec(`ALTER TABLE fixture_paged_evidence_offline RENAME TO gateway_request_attempt_evidence`)
		require.NoError(t, restoreErr)
	}()
	before := f.balance(t)
	unavailable := readDetailSection[requestledger.AttemptRecord](t, f, "101", s.RequestID, "attempts", "", 20)
	require.True(t, unavailable.EvidenceUnavailable)
	require.Len(t, unavailable.Items, 20)
	billing := readDetailSection[requestledger.BillingReference](t, f, "101", s.RequestID, "billing", "", 20)
	require.Len(t, billing.Items, 1)
	require.Equal(t, 6.0, *billing.Items[0].BilledCost)
	status, body = batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID+"?view=summary")
	require.Equal(t, 200, status, body)
	require.Equal(t, before, f.balance(t))
}

func TestRequestLedgerHTTPAttemptPageConcurrentAdmission(t *testing.T) {
	f := newBatchImageFixture(t)
	h, err := f.ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	for range 3 {
		_, err = requestledger.BeginAttempt(ctx, 234, 22)
		require.NoError(t, err)
	}
	first := readDetailSection[requestledger.AttemptRecord](t, f, "101", h.ID, "attempts", "", 2)
	seen := map[int]bool{}
	for _, a := range first.Items {
		seen[a.Number] = true
	}
	errors := make(chan error, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := requestledger.BeginAttempt(ctx, 234, 22); errors <- err }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	cursor := first.NextCursor
	for n := 0; n < 20; n++ {
		page := readDetailSection[requestledger.AttemptRecord](t, f, "101", h.ID, "attempts", cursor, 2)
		for _, a := range page.Items {
			require.False(t, seen[a.Number])
			seen[a.Number] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	require.Empty(t, cursor)
	require.Len(t, seen, 19)
}
