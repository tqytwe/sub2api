package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) acquireStarframeVideoAccountSlot(
	c *gin.Context,
	requestCtx context.Context,
	groupID *int64,
	sessionHash string,
	selection *service.AccountSelectionResult,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), openAISlotAcquireResult) {
	c.Request = c.Request.WithContext(requestCtx)
	return h.acquireResponsesAccountSlot(c, groupID, sessionHash, selection, false, streamStarted, reqLog)
}
