package handler

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// A local rejection ends this attempt without upstream-health reporting or an
// automatic replay, including when a streaming response has already started.
func (h *OpenAIGatewayHandler) handleOpenAITurnAdmissionError(c *gin.Context, err error, streamStarted bool) bool {
	var contextErr *service.APIKeyLongContextError
	if errors.As(err, &contextErr) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		h.handleStreamingAwareErrorWithCode(c, http.StatusBadRequest, "invalid_request_error", service.APIKeyContextLengthExceededCode, service.APIKeyContextLengthExceededMessage, streamStarted, false)
		return true
	}
	if !service.IsOpenAITurnAdmissionError(err) {
		return false
	}
	h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "admission_unavailable", "Account eligibility changed; please retry with complete context", streamStarted)
	return true
}
