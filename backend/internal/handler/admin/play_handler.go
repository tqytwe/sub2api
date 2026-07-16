package admin

import (
	"strconv"

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

func (h *AdminPlayHandler) GetTeamRewardSettings(c *gin.Context) {
	response.Success(c, h.playService.GetTeamRewardSettings(c.Request.Context()))
}

func (h *AdminPlayHandler) UpdateTeamRewardSettings(c *gin.Context) {
	var settings service.PlayTeamRewardSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team reward settings request"))
		return
	}
	updated, err := h.playService.UpdateTeamRewardSettings(c.Request.Context(), settings)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *AdminPlayHandler) ListTeamRewardSettlements(c *gin.Context) {
	records, err := h.playService.ListAdminTeamRewardSettlements(c.Request.Context(), 100)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, records)
}

func (h *AdminPlayHandler) RetryTeamRewardSettlement(c *gin.Context) {
	settlementID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || settlementID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid settlement id"))
		return
	}
	settlement, err := h.playService.PayoutTeamRewardSettlement(c.Request.Context(), settlementID)
	if err != nil && settlement == nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settlement)
}
