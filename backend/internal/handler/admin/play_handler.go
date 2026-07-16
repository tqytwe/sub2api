package admin

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type adminArenaSettleRequest struct {
	PeriodID int64 `json:"period_id"`
}

type adminArenaSettleResultDTO struct {
	PeriodID     int64   `json:"period_id"`
	PeriodName   string  `json:"period_name"`
	WinnersCount int     `json:"winners_count"`
	TotalAwarded float64 `json:"total_awarded"`
}

// AdminPlayHandler serves admin play operations.
type AdminPlayHandler struct {
	playService *service.PlayService
}

func NewAdminPlayHandler(playService *service.PlayService) *AdminPlayHandler {
	return &AdminPlayHandler{playService: playService}
}

// GetBlindboxPool returns the effective editable blindbox pool.
// GET /api/v1/admin/play/blindbox/pool
func (h *AdminPlayHandler) GetBlindboxPool(c *gin.Context) {
	pool, err := h.playService.GetBlindboxPoolConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

// UpdateBlindboxPool validates and replaces the editable blindbox pool.
// PUT /api/v1/admin/play/blindbox/pool
func (h *AdminPlayHandler) UpdateBlindboxPool(c *gin.Context) {
	var pool service.PlayBlindboxPool
	if err := c.ShouldBindJSON(&pool); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid blindbox pool request"))
		return
	}
	updated, err := h.playService.UpdateBlindboxPoolConfig(c.Request.Context(), pool)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

// ArenaSettle settles an arena period and distributes rank rewards.
// POST /api/v1/admin/play/arena/settle
func (h *AdminPlayHandler) ArenaSettle(c *gin.Context) {
	var req adminArenaSettleRequest
	_ = c.ShouldBindJSON(&req)
	result, err := h.playService.SettleArenaPeriod(c.Request.Context(), req.PeriodID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminArenaSettleResultDTO{
		PeriodID:     result.PeriodID,
		PeriodName:   result.PeriodName,
		WinnersCount: result.WinnersCount,
		TotalAwarded: result.TotalAwarded,
	})
}
