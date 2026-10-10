//go:build integration

package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerCompressedSSEUsesDecodedEvidence(t *testing.T) {
	_ = testEntClient(t)
	ledger := requestledger.New(integrationDB)
	for _, tc := range []struct {
		name   string
		encode func(*testing.T, []byte) []byte
	}{
		{"gzip", compressGzip}, {"br", compressBrotli}, {"deflate", compressDeflate}, {"zstd", compressZstd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
			encoded := tc.encode(t, payload)
			h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var attempts int
				require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&attempts))
				require.Equal(t, 1, attempts, "send must have precommitted evidence")
				w.Header().Set("Content-Encoding", tc.name)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write(encoded)
			}))
			defer upstream.Close()
			req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
			require.NoError(t, err)
			req.Header.Set("Accept-Encoding", tc.name)
			resp, err := doUpstreamRequest(httpClientWithRequestLedger(upstream.Client(), 234), req)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, payload, body)
			require.NoError(t, resp.Body.Close())
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			var state string
			require.NoError(t, integrationDB.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&state))
			require.Equal(t, "succeeded", state)
		})
	}
}
