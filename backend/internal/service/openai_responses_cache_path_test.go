package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func cachePathRejection(t *testing.T, param, message string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"error": map[string]any{
		"code": "invalid_parameter", "param": param, "message": message,
	}})
	require.NoError(t, err)
	return payload
}

func TestResponsesCacheContentPathRemovesOnlyRejectedHint(t *testing.T) {
	body := []byte(`{"prompt_cache_breakpoint":"top","prompt_cache_key":"keep-key","prompt_cache_options":{"mode":"explicit"},"tools":[{"name":"keep"}],"input":[{"prompt_cache_breakpoint":"message","content":[{"type":"input_text","text":"keep","prompt_cache_breakpoint":"sibling"},{"type":"input_text","text":"target text","prompt_cache_breakpoint":null}]}]}`)
	want := `{"prompt_cache_breakpoint":"top","prompt_cache_key":"keep-key","prompt_cache_options":{"mode":"explicit"},"tools":[{"name":"keep"}],"input":[{"prompt_cache_breakpoint":"message","content":[{"type":"input_text","text":"keep","prompt_cache_breakpoint":"sibling"},{"type":"input_text","text":"target text"}]}]}`
	path := "input[0].content[1].prompt_cache_breakpoint"
	for _, param := range []string{"", path} {
		t.Run("param="+param, func(t *testing.T) {
			retry, _, changed, err := normalizeOpenAIResponsesHTTPRejectedFieldRetryBody(http.StatusBadRequest, body,
				cachePathRejection(t, param, "Unsupported: '"+path+"' is not supported on this model (request id: synthetic)"))
			require.NoError(t, err)
			require.True(t, changed)
			require.JSONEq(t, want, string(retry))
		})
	}
}

func TestResponsesCachePathNeverFallsBackToSuffix(t *testing.T) {
	body := []byte(`{"prompt_cache_breakpoint":"top","input":[{"prompt_cache_breakpoint":"message","content":[{"prompt_cache_breakpoint":"content"}]}]}`)
	for _, path := range []string{
		"metadata.prompt_cache_breakpoint", "prefix_prompt_cache_breakpoint", "other.input[0].prompt_cache_breakpoint",
		"input[0].content[0].metadata.prompt_cache_breakpoint", "input[0].content[999].prompt_cache_breakpoint",
		"input[-1].prompt_cache_breakpoint", "input[999999999999999999999].prompt_cache_breakpoint",
		"input[0].content[999999999999999999999].prompt_cache_breakpoint",
		"`input[0].content[0].prompt_cache_breakpoint`",
	} {
		for _, param := range []string{"", "prompt_cache_breakpoint"} {
			t.Run(path+"/param="+param, func(t *testing.T) {
				retry, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, body,
					cachePathRejection(t, param, path+" is not supported on this model"))
				require.NoError(t, err)
				require.False(t, changed)
				require.Nil(t, retry)
			})
		}
	}
	for _, message := range []string{
		"input[0].content[0].prompt_cache_breakpoint is not supported on this model",
		"prompt_cache_breakpoint is not supported on this model; input[0].prompt_cache_breakpoint is not supported on this model",
		"prompt_cache_breakpoint is not supported on this model;input[0].prompt_cache_breakpoint is not supported on this model",
		"prompt_cache_breakpoint is not supported on this model; `input[0].prompt_cache_breakpoint` is not supported on this model",
	} {
		retry, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, body, cachePathRejection(t, "prompt_cache_breakpoint", message))
		require.NoError(t, err)
		require.False(t, changed)
		require.Nil(t, retry)
	}
}

func TestResponsesCacheContentPathRequiresArraysAndObjects(t *testing.T) {
	for _, body := range []string{
		`{"input":{"0":{"content":[{"prompt_cache_breakpoint":true}]}}}`,
		`{"input":[{"content":{"0":{"prompt_cache_breakpoint":true}}}]}`,
		`{"input":[{"content":[null]}]}`,
		`{"input":[{"content":["text"]}]}`,
		`{"input":[{"content":[]}]}`,
	} {
		retry, _, changed, err := normalizeOpenAIResponsesHTTPRejectedFieldRetryBody(http.StatusBadRequest, []byte(body),
			cachePathRejection(t, "input[0].content[0].prompt_cache_breakpoint", "Optional cache hint rejected"))
		require.NoError(t, err)
		require.False(t, changed)
		require.Nil(t, retry)
	}
}

func TestResponsesCacheRejectionDoesNotReplayResponseEvidence(t *testing.T) {
	for _, evidence := range []string{
		`"usage":{"input_tokens":1}`, `"usage":{}`, `"output_text":"partial"`,
		`"output":[{"type":"image_generation_call","status":"completed","result":"synthetic"}]`,
		`"response":{"status":"failed","usage":{"input_tokens":1}}`,
		`"data":{"usage":{}}`, `"data":{"usage":{"input_tokens":1}}`,
		`"data":{"response":{"usage":{}}}`, `"data":{"response":{"usage":{"input_tokens":1}}}`,
	} {
		response := []byte(fmt.Sprintf(`{"error":{"code":"invalid_parameter","param":"prompt_cache_breakpoint"},%s}`, evidence))
		retry, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, []byte(`{"prompt_cache_breakpoint":true}`), response)
		require.NoError(t, err)
		require.False(t, changed)
		require.Nil(t, retry)
	}
	for _, status := range []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable} {
		retry, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(status, []byte(`{"prompt_cache_breakpoint":true}`),
			cachePathRejection(t, "prompt_cache_breakpoint", "prompt_cache_breakpoint is not supported on this model"))
		require.NoError(t, err)
		require.False(t, changed)
		require.Nil(t, retry)
	}
}

func TestResponsesCacheStreamRejectionNeverRequestsFailover(t *testing.T) {
	for _, kind := range []string{"error", "response.failed"} {
		payload := string(cachePathRejection(t, "", "prompt_cache_breakpoint is not supported on this model"))
		if kind == "response.failed" {
			payload = `{"type":"response.failed","response":` + payload + `}`
		}
		require.False(t, openAIStreamFailedEventShouldFailover([]byte(payload), "prompt_cache_breakpoint is not supported on this model"))
		require.False(t, openAIStreamErrorEventShouldFailover([]byte(payload), "prompt_cache_breakpoint is not supported on this model"))
	}
}

func TestResponsesCacheWebSocketDoesNotGainContentRetry(t *testing.T) {
	body := []byte(`{"type":"response.create","input":[{"content":[{"prompt_cache_breakpoint":true}]}]}`)
	response := []byte(`{"type":"error","status":400,"error":{"code":"invalid_parameter","param":"input[0].content[0].prompt_cache_breakpoint","message":"input[0].content[0].prompt_cache_breakpoint is not supported on this model"}}`)
	retry, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, body, response)
	require.NoError(t, err)
	require.False(t, changed, "content-path support is HTTP-only; WS safety is not established by a synthetic 400")
	require.Nil(t, retry)
}
