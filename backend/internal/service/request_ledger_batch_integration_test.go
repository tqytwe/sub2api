//go:build integration

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerBatchDirectHTTPGatesEverySend(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	ctx := context.Background()
	h, err := ledger.Begin(ctx, "/v1/images/batches", "POST", "http")
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	ctx = requestledger.WithAccount(ctx, 234, 22)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var n int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1 AND account_id=234 AND credential_account_id=22`, h.ID).Scan(&n))
		require.Equal(t, calls, n)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	for range 2 {
		req, err := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
		require.NoError(t, err)
		resp, err := batchImageLedgerHTTP(server.Client(), req)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	_, err = db.Exec(`ALTER TABLE gateway_request_attempts RENAME TO ledger_fixture_attempts_offline`)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
	require.NoError(t, err)
	_, err = batchImageLedgerHTTP(server.Client(), req)
	require.ErrorIs(t, err, requestledger.ErrUnavailable)
	require.Equal(t, 2, calls)
}
