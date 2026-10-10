//go:build integration

package service

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerOpsRetrySnapshots(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil).WithContext(ctx)
	first, err := requestledger.BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{LedgerReceipt: first.Receipt(), AccountID: 234, UpstreamStatusCode: 503, Message: "synthetic failure"})
	second, err := requestledger.BeginAttempt(ctx, 235, 22)
	require.NoError(t, err)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{LedgerReceipt: second.Receipt(), AccountID: 235, UpstreamStatusCode: 429, Message: "synthetic retry"})
	entry := &OpsInsertErrorLogInput{UpstreamErrors: c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	require.Nil(t, entry.UpstreamErrors)
	require.Contains(t, *entry.UpstreamErrorsJSON, first.ID)
	require.Contains(t, *entry.UpstreamErrorsJSON, second.ID)
}

func TestRequestLedgerEmptyImageFailureKeepsOriginalAttemptAfterRetry(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	h, err := l.Begin(context.Background(), "/v1/images/generations", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil).WithContext(ctx)
	var n int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Request-ID", fmt.Sprintf("req_image_%d", n))
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_image_%d\",\"status\":\"completed\",\"output\":[]}}\n\n", n)
	}))
	defer server.Close()
	client := &http.Client{Transport: requestledger.Transport(http.DefaultTransport, 234)}
	send := func() *http.Response {
		req, e := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
		require.NoError(t, e)
		resp, e := client.Do(req)
		require.NoError(t, e)
		return resp
	}
	first := send()
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	_, count, _, err := svc.handleOpenAIImagesOAuthNonStreamingResponse(first, c, nil, "gpt-image-1")
	require.Error(t, err)
	require.Zero(t, count)
	require.NoError(t, first.Body.Close())
	second := send()
	_, err = io.Copy(io.Discard, second.Body)
	require.NoError(t, err)
	require.NoError(t, second.Body.Close())
	// An outer response restored after a retry must keep its own identity.
	compact, _ := openAICompactFallbackErrorResponse(first, &openAICompactFallbackSignal{})
	require.Equal(t, requestledger.ResponseReceipt(first).Correlation(), requestledger.ResponseReceipt(compact).Correlation())
	entry := &OpsInsertErrorLogInput{UpstreamErrors: c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	require.Contains(t, *entry.UpstreamErrorsJSON, requestledger.ResponseReceipt(first).Correlation().AttemptID)
	require.NotContains(t, *entry.UpstreamErrorsJSON, requestledger.ResponseReceipt(second).Correlation().AttemptID)
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=1`, h.ID).Scan(&state))
	require.Equal(t, "failed", state)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: 234, UpstreamStatusCode: 502})
	events := c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)
	require.Zero(t, events[len(events)-1].LedgerCorrelation.AttemptNo, "same account is not exact proof")
}

func TestRequestLedgerDelayedWSFailureCannotCopyNextTurn(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	session, err := l.Begin(context.Background(), "/v1/responses", "GET", "ws_session")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), session)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	first, err := requestledger.AcceptTurn(ctx)
	require.NoError(t, err)
	firstCtx := requestledger.WithHandle(ctx, first)
	a, err := requestledger.BeginAttempt(firstCtx, 234, 22)
	require.NoError(t, err)
	second, err := requestledger.AcceptTurn(ctx)
	require.NoError(t, err)
	secondCtx := requestledger.WithHandle(ctx, second)
	_, err = requestledger.BeginAttempt(secondCtx, 235, 22)
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil).WithContext(secondCtx)
	SetOpenAIClientTransport(c, OpenAIClientTransportWS)
	BeginOpsStreamTurn(c, 2)
	c.Set(OpsSkipPassthroughKey, true)
	c.Set(OpsUpstreamModelKey, "next-turn-model")
	markOpenAIWSClientVisibleFailure(c, "response.failed", []byte(`{"type":"response.failed","response":{"id":"resp_old","error":{"message":"synthetic failure"}}}`), a.Receipt())
	events := GetOpsStreamErrors(c)
	require.Len(t, events, 1)
	require.Equal(t, 1, events[0].Turn)
	require.Equal(t, int64(234), events[0].AccountID)
	require.False(t, events[0].SkipMonitoring)
	require.Empty(t, events[0].UpstreamModel)
	require.Equal(t, first.ID, events[0].LedgerReceipt.Correlation().RequestID)
}

func TestRequestLedgerHTTPHeaderTimeoutHasExactErrorReceipt(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil).WithContext(ctx)
	client := &http.Client{Transport: requestledger.Transport(http.DefaultTransport, 234), Timeout: 100 * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
	require.NoError(t, err)
	resp, err := requestledger.Do(req, client.Do)
	require.Nil(t, resp)
	require.Error(t, err)
	receipt := requestledger.ErrorReceipt(err)
	require.NotNil(t, receipt.Correlation())
	svc := &OpenAIGatewayService{}
	_ = svc.newOpenAIFirstOutputTimeoutError(receipt, ctx, c, &Account{ID: 234, Platform: PlatformOpenAI}, nil, "direct/no_proxy", time.Now(), "fixture", "", 100*time.Millisecond, "response_headers", nil)
	events := c.MustGet(OpsUpstreamErrorsKey).([]*OpsUpstreamErrorEvent)
	require.Len(t, events, 1)
	require.Equal(t, receipt.Correlation(), events[0].LedgerCorrelation)
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&state))
	// Client.Timeout may cancel an unknown RoundTripper with a generic error;
	// the final caller still receives a timeout with the same exact attempt.
	require.Contains(t, []string{"failed", "cancelled", "timeout"}, state)
}

func TestRequestLedgerWSResultBusinessIDIsNotResponseEvidence(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "GET", "ws_session")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	turn, err := requestledger.AcceptTurn(ctx)
	require.NoError(t, err)
	ctx = requestledger.WithHandle(ctx, turn)
	a, err := requestledger.BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	result := &OpenAIForwardResult{RequestID: "event_not_a_response", UpstreamTerminalEvent: "response.failed"}
	finishLedgerWSResult(ctx, a, result, nil)
	var id *string
	require.NoError(t, db.QueryRow(`SELECT upstream_response_id FROM gateway_request_attempt_evidence WHERE request_id=$1`, turn.ID).Scan(&id))
	require.Nil(t, id)
	require.Equal(t, "event_not_a_response", result.RequestID, "existing business/billing result is unchanged")
}
