package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"path"
)

func (h *AffiliateHandler) OperationsRead(c *gin.Context) {
	q, err := service.AffiliateOperationsQueryFromValues(c.Request.URL.Query(), 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.affiliateService.ReadAffiliateOperations(c.Request.Context(), path.Base(c.FullPath()), q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
