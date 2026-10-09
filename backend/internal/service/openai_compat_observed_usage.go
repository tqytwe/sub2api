package service

import (
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
)

// A failed compatibility response still owns its explicitly reported usage.
// Output text and token-detail subsets alone never create an estimated charge.
func openAICompatObservedFailureResult(c *gin.Context, account *Account, resp *http.Response, terminal *apicompat.ResponsesResponse, usage OpenAIUsage, originalModel, billingModel, upstreamModel string, startTime time.Time) *OpenAIForwardResult {
	if account == nil || !account.IsOpenAI() || !hasObservedOpenAIUsage(&usage, 0) {
		return nil
	}
	result := &OpenAIForwardResult{
		RequestID: resp.Header.Get("x-request-id"), UpstreamHeaders: resp.Header,
		Usage: usage, Model: originalModel, BillingModel: billingModel, UpstreamModel: upstreamModel,
		UpstreamResponseModel:         observedUpstreamResponseModel(c),
		UpstreamResponseModelConflict: observedUpstreamResponseModelConflict(c),
		UpstreamResponseServiceTier:   observedUpstreamResponseServiceTier(c),
		Duration:                      time.Since(startTime),
	}
	if terminal != nil {
		result.ResponseID = terminal.ID
	}
	return result
}

// Preserve an observed aggregate when a failed OpenAI terminal provides only
// detail subsets. Other terminals and complete typed usage keep their existing
// precedence; details alone cannot replace or manufacture measured expense.
func openAICompatTerminalUsage(current OpenAIUsage, terminal *apicompat.ResponsesUsage, preserveFailureAggregate bool) OpenAIUsage {
	next := copyOpenAIUsageFromResponsesUsage(terminal)
	if preserveFailureAggregate && hasObservedOpenAIUsage(&current, 0) && !hasObservedOpenAIUsage(&next, 0) {
		return current
	}
	return next
}
