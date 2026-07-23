package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// IPRiskHandler exposes read-only Shadow Mode runtime health during CP1.
type IPRiskHandler struct {
	riskService *service.IPRiskService
}

func NewIPRiskHandler(riskService *service.IPRiskService) *IPRiskHandler {
	return &IPRiskHandler{riskService: riskService}
}

// GetRuntime GET /api/v1/admin/ip-risk/runtime
func (h *IPRiskHandler) GetRuntime(c *gin.Context) {
	var riskService *service.IPRiskService
	if h != nil {
		riskService = h.riskService
	}
	response.Success(c, riskService.Runtime(c.Request.Context()))
}
