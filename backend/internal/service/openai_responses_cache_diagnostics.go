package service

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const responsesCacheDiagnosticKey = "openai_responses_cache_diagnostic"
const maxResponsesCacheDiagnosticPaths = 32

// ResponsesCacheDiagnostic contains only allowlisted metadata, never hint
// values, cache keys, prompt/output text, image data, or upstream error params.
// UsagePresent is presence evidence, not a claim that absent usage means free.
type ResponsesCacheDiagnostic struct {
	TransportHTTPStatus int      `json:"transport_http_status"`
	SemanticStatus      int      `json:"semantic_status"`
	ContentType         string   `json:"content_type"`
	EventType           string   `json:"event_type,omitempty"`
	BreakpointPaths     []string `json:"breakpoint_paths"`
	PathsTruncated      bool     `json:"paths_truncated"`
	HTTPAttempts        int      `json:"http_attempts"`
	FieldTransformsUsed int      `json:"field_transforms_used"`
	FieldTransformLimit int      `json:"field_transform_limit"`
	DownstreamWritten   bool     `json:"downstream_written"`
	OutputObserved      bool     `json:"output_observed"`
	UsagePresent        bool     `json:"usage_present"`
}

type responsesCacheDiagnosticState struct {
	mu sync.Mutex
	ResponsesCacheDiagnostic
	cacheFailure bool
	recorded     bool
	requestID    string
}

func responsesCacheState(c *gin.Context) *responsesCacheDiagnosticState {
	if c == nil {
		return nil
	}
	v, _ := c.Get(responsesCacheDiagnosticKey)
	state, _ := v.(*responsesCacheDiagnosticState)
	return state
}

func beginResponsesCacheHTTPAttempt(c *gin.Context, body []byte, resp *http.Response) {
	if c == nil {
		return
	}
	attempts := 1
	if previous := responsesCacheState(c); previous != nil {
		previous.mu.Lock()
		attempts += previous.HTTPAttempts
		previous.mu.Unlock()
	}
	paths, truncated := responsesCacheBreakpointPaths(body)
	state := &responsesCacheDiagnosticState{ResponsesCacheDiagnostic: ResponsesCacheDiagnostic{
		HTTPAttempts: attempts, BreakpointPaths: paths, PathsTruncated: truncated,
		FieldTransformLimit: maxOpenAIResponsesRejectedFieldRetries,
	}}
	if resp != nil {
		state.TransportHTTPStatus = resp.StatusCode
		state.SemanticStatus = resp.StatusCode
		state.requestID = resp.Header.Get("x-request-id")
		contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		switch contentType {
		case "text/event-stream", "application/json":
			state.ContentType = contentType
		default:
			state.ContentType = "other_or_missing"
		}
	}
	c.Set(responsesCacheDiagnosticKey, state)
}

func responsesCacheBreakpointPaths(body []byte) ([]string, bool) {
	paths := make([]string, 0)
	if gjson.GetBytes(body, "prompt_cache_breakpoint").Exists() {
		paths = append(paths, "prompt_cache_breakpoint")
	}
	input := gjson.GetBytes(body, "input")
	if input.IsArray() {
		input.ForEach(func(i, item gjson.Result) bool {
			if item.IsObject() && item.Get("prompt_cache_breakpoint").Exists() {
				paths = append(paths, fmt.Sprintf("input[%d].prompt_cache_breakpoint", i.Int()))
			}
			if content := item.Get("content"); content.IsArray() && len(paths) <= maxResponsesCacheDiagnosticPaths {
				content.ForEach(func(j, part gjson.Result) bool {
					if part.IsObject() && part.Get("prompt_cache_breakpoint").Exists() {
						paths = append(paths, fmt.Sprintf("input[%d].content[%d].prompt_cache_breakpoint", i.Int(), j.Int()))
					}
					return len(paths) <= maxResponsesCacheDiagnosticPaths
				})
			}
			return len(paths) <= maxResponsesCacheDiagnosticPaths
		})
	}
	if len(paths) > maxResponsesCacheDiagnosticPaths {
		return paths[:maxResponsesCacheDiagnosticPaths], true
	}
	return paths, false
}

func isResponsesCacheModelFailure(payload []byte, message string) bool {
	message = strings.ToLower(message)
	return openAIStreamFailedEventErrorCode(payload) == "invalid_parameter" &&
		strings.Contains(message, "prompt_cache_breakpoint") && strings.Contains(message, "is not supported on this model")
}

func observeResponsesCachePayload(c *gin.Context, payload []byte, eventType string) {
	state := responsesCacheState(c)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	for _, path := range []string{"usage", "response.usage", "error.usage", "data.usage", "data.response.usage"} {
		state.UsagePresent = state.UsagePresent || gjson.GetBytes(payload, path).Exists()
	}
	for _, path := range []string{"output", "response.output", "output_text", "response.output_text", "delta", "item"} {
		value := gjson.GetBytes(payload, path)
		if (value.IsArray() && len(value.Array()) > 0) || value.IsObject() || (value.Type == gjson.String && value.String() != "") {
			state.OutputObserved = true
		}
	}
	eventType = effectiveOpenAISSEEventType(payload, eventType)
	message := extractOpenAISSEErrorMessage(payload)
	if isResponsesCacheModelFailure(payload, message) {
		state.cacheFailure = true
		if eventType == "error" || eventType == "response.failed" {
			state.EventType = eventType
			state.SemanticStatus = openAIStreamFailedEventSemanticStatus(payload, message)
		}
	}
}

func observeResponsesCacheSSEBody(c *gin.Context, body []byte) {
	forEachOpenAISSEFrame(string(body), func(eventType string, payload []byte) {
		observeResponsesCachePayload(c, payload, eventType)
	})
}

func attachResponsesCacheDiagnostic(c *gin.Context, event *OpsUpstreamErrorEvent) {
	state := responsesCacheState(c)
	if state == nil || !strings.Contains(strings.ToLower(event.Message), "prompt_cache_breakpoint") {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.cacheFailure {
		return
	}
	snapshot := state.ResponsesCacheDiagnostic
	snapshot.BreakpointPaths = append([]string{}, snapshot.BreakpointPaths...)
	snapshot.DownstreamWritten = c.Writer != nil && c.Writer.Written()
	if v, ok := c.Get(openAIResponsesRejectedFieldRetryBudgetContextKey); ok {
		if budget, ok := v.(*openAIResponsesRejectedFieldRetryBudget); ok {
			budget.mu.Lock()
			snapshot.FieldTransformsUsed = budget.attempts
			budget.mu.Unlock()
		}
	}
	event.ResponsesCacheDiagnostic = &snapshot
	state.recorded = true
}

// Some buffered/protocol-error paths only set the final error status. Preserve
// an attempt record for cache failures even when no billable usage was supplied.
func (s *OpenAIGatewayService) recordUnloggedResponsesCacheFailure(c *gin.Context, account *Account, passthrough bool) {
	state := responsesCacheState(c)
	if state == nil {
		return
	}
	state.mu.Lock()
	record := state.cacheFailure && !state.recorded
	requestID := state.requestID
	state.mu.Unlock()
	if record {
		s.recordOpenAIStreamUpstreamError(c, account, passthrough, requestID, "stream_failed", nil,
			"prompt_cache_breakpoint is not supported on this model")
	}
}
