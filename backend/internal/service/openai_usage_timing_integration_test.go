package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageTimingFlushWriter struct {
	gin.ResponseWriter
	failureMode string
	recorder    *httptest.ResponseRecorder
}

func (w *usageTimingFlushWriter) FlushError() error {
	if w.failureMode == "all" || (w.failureMode == "terminal" && strings.Contains(w.recorder.Body.String(), "response.completed")) {
		return errors.New("synthetic transport flush failure")
	}
	w.Flush()
	return nil
}

func TestOpenAIUsageTimingStreamsCollectContentAndTerminal(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			snapshot, usage := runUsageTimingStream(t, passthrough, "",
				"event: response.created\ndata: {\"response\":{\"id\":\"resp_timing\"}}\n\n"+
					"event: response.reasoning_summary_text.delta\ndata: {\"delta\":\"reasoning must not be persisted\"}\n\n"+
					"event: response.output_text.delta\ndata: {\"delta\":\"visible text must not be persisted\"}\n\n"+
					"event: response.completed\ndata: {\"response\":{\"id\":\"resp_timing\",\"usage\":{\"input_tokens\":3,\"output_tokens\":7}}}\n\n")
			require.Equal(t, 3, usage.InputTokens)
			require.Equal(t, 7, usage.OutputTokens)
			require.Contains(t, snapshot.Events, "first_semantic")
			require.Contains(t, snapshot.Events, "first_visible")
			require.Contains(t, snapshot.Events, "first_output_flush")
			require.Contains(t, snapshot.Events, "first_sse_event_flush")
			require.Equal(t, "completed", snapshot.Terminal)
			require.Equal(t, "response.completed", snapshot.TerminalFlushed)
			require.Len(t, snapshot.Attempts, 2)
			physical := snapshot.Attempts[1]
			require.Contains(t, physical.Events, "first_reasoning")
			require.Contains(t, physical.Events, "first_visible_text")
			require.Contains(t, physical.Events, "first_semantic")
			require.Equal(t, "completed", physical.Terminal)
			content := requesttiming.DeriveContentObservation(snapshot)
			require.Equal(t, "observed", content.Status)
			require.NotNil(t, content.ReasoningFirstMS)
			require.NotNil(t, content.VisibleTextFirstMS)
			encoded, err := json.Marshal(snapshot)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "must not be persisted")
		})
	}
}

func TestOpenAIUsageTimingFlushFailureDoesNotClaimDeliveryAndStillDrainsUsage(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			snapshot, usage := runUsageTimingStream(t, passthrough, "all",
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"output\"}\n\n"+
					"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":7}}}\n\n")
			require.Equal(t, 7, usage.OutputTokens)
			require.Equal(t, "completed", snapshot.Terminal)
			require.Empty(t, snapshot.TerminalFlushed)
			require.NotContains(t, snapshot.Events, "first_output_flush")
			require.NotContains(t, snapshot.Events, "first_sse_event_flush")
			require.True(t, snapshot.ClientDisconnect)
			require.Equal(t, "client_disconnected", snapshot.Outcome)
		})
	}
}

func TestOpenAIUsageTimingTerminalFlushFailureKeepsUpstreamAndDeliverySeparate(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			snapshot, usage := runUsageTimingStream(t, passthrough, "terminal",
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"output\"}\n\n"+
					"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":7}}}\n\n")
			require.Equal(t, 7, usage.OutputTokens)
			require.Equal(t, "completed", snapshot.Terminal)
			require.Empty(t, snapshot.TerminalFlushed)
			require.Contains(t, snapshot.Events, "first_output_flush")
			require.Contains(t, snapshot.Events, "first_sse_event_flush")
			require.True(t, snapshot.ClientDisconnect)
			require.Equal(t, "client_disconnected", snapshot.Outcome)
		})
	}
}

func TestOpenAIUsageTimingMetadataIsNotSemanticOutput(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			snapshot, _ := runUsageTimingStream(t, passthrough, "",
				": heartbeat\n\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_timing\"}}\n\n"+
					"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
			require.NotContains(t, snapshot.Events, "first_semantic")
			require.NotContains(t, snapshot.Events, "first_visible")
			require.NotContains(t, snapshot.Events, "first_output_flush")
		})
	}
}

func TestOpenAIUsageTimingResidualFlushFailureUpdatesReturnedDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Writer = &usageTimingFlushWriter{ResponseWriter: c.Writer, failureMode: "all", recorder: recorder}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: &passthroughFlushTestErrorBody{
		payload: []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n"),
		err:     io.ErrUnexpectedEOF,
	}}
	result, err := svc.handleStreamingResponsePassthrough(context.Background(), response, c,
		&Account{ID: 41, Platform: PlatformOpenAI}, time.Now(), "test-model", "test-model")
	require.Error(t, err)
	require.NotNil(t, result)
	require.True(t, result.clientDisconnect, "a deferred residual flush must update the result used by the forwarding boundary")
}

func runUsageTimingStream(t *testing.T, passthrough bool, failureMode, payload string) (requesttiming.Snapshot, *OpenAIUsage) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	started := time.Now()
	collector := requesttiming.New(started, 0)
	ctx := requesttiming.With(context.Background(), collector)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	request, egress := requesttiming.StartAttempt(request, 41, 0)
	request, transport := requesttiming.StartTransport(request)
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload)), Request: request}
	transport.Response(response, nil)
	egress.Response(response, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Writer = &usageTimingFlushWriter{ResponseWriter: c.Writer, failureMode: failureMode, recorder: recorder}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	account := &Account{ID: 41, Platform: PlatformOpenAI, Name: "timing-test"}
	var usage *OpenAIUsage
	if passthrough {
		result, err := svc.handleStreamingResponsePassthrough(ctx, response, c, account, started, "test-model", "test-model")
		require.NoError(t, err)
		require.NotNil(t, result)
		usage = result.usage
	} else {
		result, err := svc.handleStreamingResponse(ctx, response, c, account, started, "test-model", "test-model")
		require.NoError(t, err)
		require.NotNil(t, result)
		usage = result.usage
	}
	require.NoError(t, response.Body.Close())
	var snapshot requesttiming.Snapshot
	collector.WhenFinished(func(s requesttiming.Snapshot) { snapshot = s })
	collector.Finish(http.StatusOK, false)
	return snapshot, usage
}
