package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"path"
)

func (h *UserHandler) AffiliateOperationsRead(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	q, err := service.AffiliateOperationsQueryFromValues(c.Request.URL.Query(), subject.UserID, false)
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
