package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelCatalogHandler admin APIs for site model catalog.
type ModelCatalogHandler struct {
	catalogService *service.ModelCatalogService
}

func NewModelCatalogHandler(catalogService *service.ModelCatalogService) *ModelCatalogHandler {
	return &ModelCatalogHandler{catalogService: catalogService}
}

type catalogEntryRequest struct {
	ID              int64    `json:"id"`
	ModelName       string   `json:"model_name" binding:"required,max=128"`
	Platform        string   `json:"platform" binding:"max=50"`
	DisplayName     *string  `json:"display_name"`
	UseCase         *string  `json:"use_case"`
	SortOrder       int      `json:"sort_order"`
	VisiblePublic   bool     `json:"visible_public"`
	VisibleAuth     bool     `json:"visible_auth"`
	Featured        bool     `json:"featured"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
	PriceMultiplier *float64 `json:"price_multiplier"`
	BillingMode     string   `json:"billing_mode"`
	Source          string   `json:"source"`
}

type batchVisibilityRequest struct {
	IDs           []int64 `json:"ids" binding:"required,min=1"`
	VisiblePublic *bool   `json:"visible_public"`
	VisibleAuth   *bool   `json:"visible_auth"`
}

type batchPricesRequest struct {
	IDs         []int64  `json:"ids" binding:"required,min=1"`
	Multiplier  *float64 `json:"multiplier"`
	InputPrice  *float64 `json:"input_price"`
	OutputPrice *float64 `json:"output_price"`
}

type importDiscoveriesRequest struct {
	IDs            []int64  `json:"ids"`
	ToCatalog      bool     `json:"to_catalog"`
	SiteMultiplier *float64 `json:"site_multiplier"`
}

// List GET /admin/model-catalog
func (h *ModelCatalogHandler) List(c *gin.Context) {
	filter := service.CatalogListFilter{
		Platform: c.Query("platform"),
		Search:   c.Query("search"),
	}
	if v := c.Query("visible_public"); v != "" {
		b := v == "true"
		filter.VisiblePublic = &b
	}
	if v := c.Query("visible_auth"); v != "" {
		b := v == "true"
		filter.VisibleAuth = &b
	}
	rows, err := h.catalogService.ListAdminCatalog(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

// Upsert PUT /admin/model-catalog
func (h *ModelCatalogHandler) Upsert(c *gin.Context) {
	var req catalogEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	entry := &service.SiteModelCatalogEntry{
		ID:              req.ID,
		ModelName:       req.ModelName,
		Platform:        req.Platform,
		DisplayName:     req.DisplayName,
		UseCase:         req.UseCase,
		SortOrder:       req.SortOrder,
		VisiblePublic:   req.VisiblePublic,
		VisibleAuth:     req.VisibleAuth,
		Featured:        req.Featured,
		InputPrice:      req.InputPrice,
		OutputPrice:     req.OutputPrice,
		CacheReadPrice:  req.CacheReadPrice,
		CacheWritePrice: req.CacheWritePrice,
		PriceMultiplier: req.PriceMultiplier,
		BillingMode:     req.BillingMode,
		Source:          req.Source,
	}
	if err := h.catalogService.SaveCatalogEntry(c.Request.Context(), entry); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entry)
}

// Delete DELETE /admin/model-catalog/:id
func (h *ModelCatalogHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.catalogService.DeleteCatalogEntry(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// BatchVisibility POST /admin/model-catalog/batch-visibility
func (h *ModelCatalogHandler) BatchVisibility(c *gin.Context) {
	var req batchVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	n, err := h.catalogService.BatchVisibility(c.Request.Context(), req.IDs, req.VisiblePublic, req.VisibleAuth)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": n})
}

// BatchPrices POST /admin/model-catalog/batch-prices
func (h *ModelCatalogHandler) BatchPrices(c *gin.Context) {
	var req batchPricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	n, err := h.catalogService.BatchPrices(c.Request.Context(), req.IDs, req.Multiplier, req.InputPrice, req.OutputPrice)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": n})
}

// CreateSyncJob POST /admin/model-catalog/sync-jobs
func (h *ModelCatalogHandler) CreateSyncJob(c *gin.Context) {
	job, err := h.catalogService.StartSyncJob(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

// GetSyncJob GET /admin/model-catalog/sync-jobs/:id
func (h *ModelCatalogHandler) GetSyncJob(c *gin.Context) {
	job, err := h.catalogService.GetSyncJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if job == nil {
		response.NotFound(c, "job not found")
		return
	}
	response.Success(c, job)
}

// ListDiscoveries GET /admin/model-catalog/discoveries
func (h *ModelCatalogHandler) ListDiscoveries(c *gin.Context) {
	filter := service.DiscoveryListFilter{
		Status: c.DefaultQuery("status", "new"),
		Search: c.Query("search"),
	}
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil {
		filter.Limit = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil {
		filter.Offset = v
	}
	result, err := h.catalogService.ListDiscoveries(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ImportDiscoveries POST /admin/model-catalog/discoveries/import
func (h *ModelCatalogHandler) ImportDiscoveries(c *gin.Context) {
	var req importDiscoveriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		response.BadRequest(c, "ids required: select discoveries to import")
		return
	}
	n, err := h.catalogService.ImportDiscoveries(c.Request.Context(), req.IDs, req.ToCatalog, req.SiteMultiplier)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"imported": n})
}
