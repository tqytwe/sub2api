package handler

import (
	"net/http"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// TryOpenAIVideoGeneration selects the relay only for models routed to an
// opted-in account. Existing provider-specific models keep their handler.
func (h *OpenAIGatewayHandler) TryOpenAIVideoGeneration(c *gin.Context) bool {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil || h == nil || h.gatewayService == nil || !service.GroupAllowsImageGeneration(key.Group) {
		return false
	}
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
		} else {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		}
		return true
	}
	if c.Request.Body != nil {
		_ = c.Request.Body.Close()
	}
	c.Request.Body = pkghttputil.NewPrereadBody(body)
	model := service.ExtractAgnesVideoRequestModel(body)
	if model == "" {
		model = service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body).Model
	}
	enabled, err := h.gatewayService.OpenAIVideoModelUsesRelay(c.Request.Context(), key.GroupID, model)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Could not check video availability")
		return true
	}
	if !enabled {
		return false
	}
	setActualUpstreamEndpoint(c, "/v1/videos")
	h.handleGrokMedia(c, service.OpenAIVideoEndpointCreate, "")
	return true
}

func (h *OpenAIGatewayHandler) OpenAIVideoLookup(c *gin.Context) {
	endpoint := service.OpenAIVideoEndpointStatus
	if strings.HasSuffix(c.FullPath(), "/content") {
		endpoint = service.OpenAIVideoEndpointContent
	}
	// Composite status/content calls have no model for the creation router to
	// resolve. The task namespace identifies the quota platform.
	c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformOpenAI))
	setActualUpstreamEndpoint(c, "/v1/videos")
	h.handleGrokMedia(c, endpoint, c.Param("request_id"))
}
