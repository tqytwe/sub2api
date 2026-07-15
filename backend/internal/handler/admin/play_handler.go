package admin

import (
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
