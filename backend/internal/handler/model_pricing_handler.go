package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPricingHandler serves authenticated model pricing endpoints.
type ModelPricingHandler struct {
	catalogService *service.ModelCatalogService
	playService    *service.PlayService
	billingService *service.BillingService
}

func NewModelPricingHandler(
	catalogService *service.ModelCatalogService,
	playService *service.PlayService,
	billingService *service.BillingService,
) *ModelPricingHandler {
	return &ModelPricingHandler{
		catalogService: catalogService,
		playService:    playService,
		billingService: billingService,
	}
}

// MyPricing GET /api/v1/models/my-pricing
func (h *ModelPricingHandler) MyPricing(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.catalogService == nil {
		response.Success(c, service.MyModelPricingResponse{Enabled: false, Models: []service.MyModelPricingRow{}})
		return
	}
	resp, err := h.catalogService.ListMyPricing(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, resp)
}

// PublicModelPricing GET /api/v1/public/model-pricing — guest catalog pricing.
func (h *ModelPricingHandler) PublicModelPricing(c *gin.Context) {
	if h.catalogService == nil {
		if h.playService != nil && h.billingService != nil {
			rows := h.playService.ListPublicModelPricing(c.Request.Context(), h.billingService)
			response.Success(c, rows)
			return
		}
		response.Success(c, []service.PublicModelPricingRow{})
		return
	}
	rows := h.catalogService.ListPublicPricing(c.Request.Context())
	response.Success(c, rows)
}
