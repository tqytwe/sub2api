package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func providerProfitLinkError(c *gin.Context, err error) {
	for _, known := range []error{
		service.ErrProviderProfitLinkValidation,
		service.ErrProviderProfitLinkNotFound,
		service.ErrProviderProfitLinkConflict,
		service.ErrProviderProfitLinkUnavailable,
	} {
		if errors.Is(err, known) {
			response.ErrorFrom(c, known)
			return
		}
	}
	response.InternalError(c, "Provider profit link operation failed")
}

func (h *ChannelHandler) ListProviderProfitLinks(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageErr != nil || sizeErr != nil || service.ValidateProviderProfitLinkPage(page, size) != nil {
		providerProfitLinkError(c, service.ErrProviderProfitLinkValidation)
		return
	}
	r, err := h.channelService.ProviderProfitLinkRepository()
	if err != nil {
		providerProfitLinkError(c, err)
		return
	}
	out, err := r.ListProviderProfitLinks(c.Request.Context(), page, size)
	if err != nil {
		providerProfitLinkError(c, err)
		return
	}
	response.Success(c, out)
}

func (h *ChannelHandler) SetProviderProfitLink(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		providerProfitLinkError(c, service.ErrProviderProfitLinkValidation)
		return
	}
	var input service.ProviderProfitLinkMutation
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		providerProfitLinkError(c, service.ErrProviderProfitLinkValidation)
		return
	}
	if err := input.Validate(id); err != nil {
		providerProfitLinkError(c, err)
		return
	}
	r, err := h.channelService.ProviderProfitLinkRepository()
	if err != nil {
		providerProfitLinkError(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.provider.profit_channel")
	middleware.SetAuditExtra(c, map[string]any{"provider_id": id, "target_channel_id": input.ChannelID, "revision": input.Revision})
	out, err := r.SetProviderProfitLink(c.Request.Context(), id, input)
	if err != nil {
		providerProfitLinkError(c, err)
		return
	}
	response.Success(c, out)
}
