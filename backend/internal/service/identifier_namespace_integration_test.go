//go:build integration

package service

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

// This proves a reachable image adapter namespace error, not the cause of any
// historical ordinary /responses request. Headers and response IDs may differ.
func TestRequestLedgerImageIdentifierNamespaces(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	for _, kind := range []string{"response.incomplete", "response.failed"} {
		t.Run(kind, func(t *testing.T) {
			body := fmt.Sprintf("data: {\"type\":%q,\"response\":{\"id\":\"resp_image\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"error\":{\"type\":\"server_error\",\"message\":\"synthetic failure\"}}}\n\n", kind)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Request-ID", "req_image")
				_, _ = io.WriteString(w, body)
			}))
			defer upstream.Close()
			ctx, h, c := ledgerFollowupContext(t, ledger, "/v1/images/generations", "http")
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, nil)
			require.NoError(t, err)
			resp, err := (&http.Client{Transport: requestledger.Transport(http.DefaultTransport, 234)}).Do(req)
			require.NoError(t, err)
			payload, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			receipt := requestledger.ResponseReceipt(resp)
			upstreamErr := extractOpenAIImagesUpstreamError(payload)
			require.NotNil(t, upstreamErr)
			// Mark output as already written so this isolated path retains its
			// existing error without invoking account policy or scheduling.
			service := &OpenAIGatewayService{}
			got := service.handleOpenAIImagesOAuthResponseError(receipt, ctx, c,
				&Account{ID: 234, Platform: "openai"}, "gpt-image-2", "", resp, -2, upstreamErr)
			require.Same(t, upstreamErr, got)
			raw, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			events, ok := raw.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.NotEmpty(t, events)
			event := events[len(events)-1]
			require.Equal(t, receipt.Correlation(), event.LedgerCorrelation)
			require.Equal(t, "req_image", event.UpstreamRequestID, "response.id must not replace the transport request ID")
			require.Equal(t, "resp_image", event.UpstreamResponseID)
			require.Eventually(t, func() bool {
				var requestID, responseID string
				var conflict bool
				err := db.QueryRow(`SELECT upstream_request_id,upstream_response_id,identifier_conflict FROM gateway_request_attempt_evidence WHERE request_id=$1 AND attempt_id=$2`, h.ID, receipt.Correlation().AttemptID).Scan(&requestID, &responseID, &conflict)
				return err == nil && requestID == "req_image" && responseID == "resp_image" && !conflict
			}, 3*time.Second, 10*time.Millisecond)
		})
	}
}
