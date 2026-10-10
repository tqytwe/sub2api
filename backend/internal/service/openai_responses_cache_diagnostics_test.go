package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesCacheFailureDiagnosticsPreservePartialFailure(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			for _, metered := range []bool{false, true} {
				t.Run(fmt.Sprintf("passthrough=%v/stream=%v/metered=%v", passthrough, stream, metered), func(t *testing.T) {
					usage := ""
					if metered {
						usage = `,"usage":{"input_tokens":9,"output_tokens":2}`
					}
					payload := `{"type":"response.failed","response":{"status":"failed","output":[{"type":"message","content":[{"type":"output_text","text":"private-output"}]}],"error":{"code":"invalid_parameter","message":"prompt_cache_breakpoint is not supported on this model"}` + usage + `}}`
					response := newOpenAIRejectedFieldTestResponse(http.StatusOK, "data: "+payload+"\n\n")
					response.Header.Set("Content-Type", "text/event-stream; charset=utf-8")
					upstream := &httpUpstreamRecorder{responses: []*http.Response{response}}
					account := newOpenAIRejectedFieldTestAccount()
					account.Extra["openai_passthrough"] = passthrough
					body := []byte(fmt.Sprintf(`{"model":"gpt-5.5","stream":%v,"prompt_cache_key":"private-key","input":[{"role":"user","content":[{"type":"input_text","text":"private-input","prompt_cache_breakpoint":{"secret":"private-hint"}}]}]}`, stream))
					c := newOpenAIRejectedFieldTestContext(body)
					result, err := newOpenAIRejectedFieldTestService(upstream).Forward(context.Background(), c, account, body)
					require.Error(t, err)
					var failover *UpstreamFailoverError
					require.NotErrorAs(t, err, &failover, "cache validation failure must not ask the handler to replay")
					require.Len(t, upstream.bodies, 1, "partial output must not cause a replay, even without usage")
					if metered {
						require.NotNil(t, result)
						require.Equal(t, 9, result.Usage.InputTokens)
						require.Equal(t, 2, result.Usage.OutputTokens)
					} else if result != nil {
						require.False(t, result.HasObservedUsage(), "no usage may be invented from partial text")
					}
					events, exists := c.Get(OpsUpstreamErrorsKey)
					require.True(t, exists, "failed requests must remain auditable without a usage record")
					raw, err := json.Marshal(events)
					require.NoError(t, err)
					diagnostic := gjson.GetBytes(raw, "0.responses_cache_diagnostic")
					require.True(t, diagnostic.Exists(), string(raw))
					require.Equal(t, int64(200), diagnostic.Get("transport_http_status").Int())
					require.Equal(t, int64(502), diagnostic.Get("semantic_status").Int())
					require.Equal(t, "text/event-stream", diagnostic.Get("content_type").String())
					require.Equal(t, "response.failed", diagnostic.Get("event_type").String())
					require.True(t, diagnostic.Get("output_observed").Bool())
					require.Equal(t, metered, diagnostic.Get("usage_present").Bool())
					require.Equal(t, int64(1), diagnostic.Get("http_attempts").Int())
					require.Zero(t, diagnostic.Get("field_transforms_used").Int())
					require.Equal(t, int64(6), diagnostic.Get("field_transform_limit").Int())
					require.Equal(t, "input[0].content[0].prompt_cache_breakpoint", diagnostic.Get("breakpoint_paths.0").String())
					for _, private := range []string{"private-input", "private-output", "private-key", "private-hint"} {
						require.NotContains(t, diagnostic.Raw, private)
					}
				})
			}
		}
	}
}

func TestResponsesCacheHTTPDiagnosticsExposeBoundedRetry(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","stream":false,"input":[{"role":"user","content":[{"type":"input_text","text":"keep","prompt_cache_breakpoint":true}]}]}`)
	payload := string(cachePathRejection(t, "input[0].content[0].prompt_cache_breakpoint", "input[0].content[0].prompt_cache_breakpoint is not supported on this model"))
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newOpenAIRejectedFieldTestResponse(http.StatusBadRequest, payload),
		newOpenAIRejectedFieldTestResponse(http.StatusBadRequest, payload),
	}}
	c := newOpenAIRejectedFieldTestContext(body)
	_, err := newOpenAIRejectedFieldTestService(upstream).Forward(context.Background(), c, newOpenAIRejectedFieldTestAccount(), body)
	require.Error(t, err)
	require.Len(t, upstream.bodies, 2)
	events, exists := c.Get(OpsUpstreamErrorsKey)
	require.True(t, exists)
	raw, err := json.Marshal(events)
	require.NoError(t, err)
	diagnostic := gjson.GetBytes(raw, "0.responses_cache_diagnostic")
	require.Equal(t, int64(400), diagnostic.Get("transport_http_status").Int())
	require.Equal(t, int64(2), diagnostic.Get("http_attempts").Int())
	require.Equal(t, int64(1), diagnostic.Get("field_transforms_used").Int())
	require.Empty(t, diagnostic.Get("breakpoint_paths").Array())
}

func TestResponsesCacheDiagnosticPathsAreBoundedAndSnapshotsImmutable(t *testing.T) {
	items := make([]any, 40)
	for i := range items {
		items[i] = map[string]any{"prompt_cache_breakpoint": "private-value"}
	}
	body, err := json.Marshal(map[string]any{"input": items})
	require.NoError(t, err)
	c := newOpenAIRejectedFieldTestContext(body)
	response := newOpenAIRejectedFieldTestResponse(http.StatusBadRequest, "")
	beginResponsesCacheHTTPAttempt(c, body, response)
	observeResponsesCachePayload(c, cachePathRejection(t, "prompt_cache_breakpoint", "prompt_cache_breakpoint is not supported on this model"), "")
	event := OpsUpstreamErrorEvent{Message: "prompt_cache_breakpoint is not supported on this model"}
	appendOpsUpstreamError(c, event)
	events, _ := c.Get(OpsUpstreamErrorsKey)
	typedEvents, ok := events.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, typedEvents, 1)
	snapshot := typedEvents[0].ResponsesCacheDiagnostic
	require.Len(t, snapshot.BreakpointPaths, 32)
	require.True(t, snapshot.PathsTruncated)
	beginResponsesCacheHTTPAttempt(c, []byte(`{"input":[]}`), response)
	require.Len(t, snapshot.BreakpointPaths, 32)
	require.Equal(t, 1, snapshot.HTTPAttempts)
}

func TestResponsesCacheDiagnosticUsesSSEEventHeader(t *testing.T) {
	for _, eventType := range []string{"error", "response.failed"} {
		t.Run(eventType, func(t *testing.T) {
			body := []byte(`{"input":"hello"}`)
			c := newOpenAIRejectedFieldTestContext(body)
			beginResponsesCacheHTTPAttempt(c, body, newOpenAIRejectedFieldTestResponse(http.StatusOK, ""))
			payload := string(cachePathRejection(t, "", "prompt_cache_breakpoint is not supported on this model"))
			if eventType == "response.failed" {
				payload = `{"response":` + payload + `}`
			}
			observeResponsesCacheSSEBody(c, []byte("event: "+eventType+"\ndata: "+payload+"\n\n"))
			event := OpsUpstreamErrorEvent{Message: "prompt_cache_breakpoint is not supported on this model"}
			attachResponsesCacheDiagnostic(c, &event)
			require.NotNil(t, event.ResponsesCacheDiagnostic)
			require.Equal(t, eventType, event.ResponsesCacheDiagnostic.EventType)
			require.Equal(t, 502, event.ResponsesCacheDiagnostic.SemanticStatus)
		})
	}
}

func TestResponsesCacheDiagnosticPreservesErrorUsagePresenceThroughQueue(t *testing.T) {
	c := newOpenAIRejectedFieldTestContext([]byte(`{}`))
	beginResponsesCacheHTTPAttempt(c, []byte(`{}`), newOpenAIRejectedFieldTestResponse(http.StatusBadRequest, ""))
	observeResponsesCachePayload(c, []byte(`{"error":{"code":"invalid_parameter","message":"prompt_cache_breakpoint is not supported on this model","usage":{}}}`), "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{Message: "prompt_cache_breakpoint is not supported on this model"})
	events, _ := c.Get(OpsUpstreamErrorsKey)
	typedEvents, ok := events.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	entry := &OpsInsertErrorLogInput{UpstreamErrors: typedEvents}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	require.NotNil(t, entry.UpstreamErrorsJSON)
	stored, err := ParseOpsUpstreamErrors(*entry.UpstreamErrorsJSON)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.NotNil(t, stored[0].ResponsesCacheDiagnostic)
	require.True(t, stored[0].ResponsesCacheDiagnostic.UsagePresent, "presence must not be inferred from positive aggregate tokens")
}

func TestResponsesCacheErrorThenCompletedDoesNotRecordFalseFailure(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(fmt.Sprintf("passthrough=%v", passthrough), func(t *testing.T) {
			payload := "event: error\ndata: " + string(cachePathRejection(t, "prompt_cache_breakpoint", "prompt_cache_breakpoint is not supported on this model")) + "\n\n" +
				"event: response.completed\ndata: " + `{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"
			response := newOpenAIRejectedFieldTestResponse(http.StatusOK, payload)
			response.Header.Set("Content-Type", "text/event-stream")
			upstream := &httpUpstreamRecorder{responses: []*http.Response{response}}
			account := newOpenAIRejectedFieldTestAccount()
			account.Extra["openai_passthrough"] = passthrough
			body := []byte(`{"model":"gpt-5.5","stream":false,"input":"hello"}`)
			c := newOpenAIRejectedFieldTestContext(body)
			_, err := newOpenAIRejectedFieldTestService(upstream).Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			_, exists := c.Get(OpsUpstreamErrorsKey)
			require.False(t, exists, "successful final terminal must not acquire a synthetic failure from diagnostics")
		})
	}
}

func TestResponsesCacheFailedImageRetainsExpenseWithoutReplay(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("passthrough=%v/stream=%v", passthrough, stream), func(t *testing.T) {
				payload := `data: {"type":"response.failed","response":{"status":"failed","output":[{"id":"synthetic_image","type":"image_generation_call","status":"completed","result":"synthetic-final-image","size":"1024x1024"}],"error":{"code":"invalid_parameter","message":"prompt_cache_breakpoint is not supported on this model"}}}` + "\n\n"
				response := newOpenAIRejectedFieldTestResponse(http.StatusOK, payload)
				response.Header.Set("Content-Type", "text/event-stream")
				upstream := &httpUpstreamRecorder{responses: []*http.Response{response}}
				account := newOpenAIRejectedFieldTestAccount()
				account.Extra["openai_passthrough"] = passthrough
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.5","stream":%v,"input":"hello"}`, stream))
				c := newOpenAIRejectedFieldTestContext(body)
				result, err := newOpenAIRejectedFieldTestService(upstream).Forward(context.Background(), c, account, body)
				require.Error(t, err)
				var failover *UpstreamFailoverError
				require.NotErrorAs(t, err, &failover)
				require.NotNil(t, result)
				require.Equal(t, 1, result.ImageCount)
				require.Zero(t, result.Usage.InputTokens)
				require.Len(t, upstream.bodies, 1)
			})
		}
	}
}

func TestResponsesCacheDiagnosticRecognizesWrappedUsage(t *testing.T) {
	for _, evidence := range []string{`"data":{"usage":{}}`, `"data":{"usage":{"input_tokens":1}}`, `"data":{"response":{"usage":{}}}`, `"data":{"response":{"usage":{"input_tokens":1}}}`} {
		c := newOpenAIRejectedFieldTestContext([]byte(`{}`))
		beginResponsesCacheHTTPAttempt(c, []byte(`{}`), newOpenAIRejectedFieldTestResponse(http.StatusBadRequest, ""))
		observeResponsesCachePayload(c, []byte(`{"error":{"code":"invalid_parameter","message":"prompt_cache_breakpoint is not supported on this model"},`+evidence+`}`), "")
		event := OpsUpstreamErrorEvent{Message: "prompt_cache_breakpoint is not supported on this model"}
		attachResponsesCacheDiagnostic(c, &event)
		require.NotNil(t, event.ResponsesCacheDiagnostic)
		require.True(t, event.ResponsesCacheDiagnostic.UsagePresent, evidence)
	}
}
