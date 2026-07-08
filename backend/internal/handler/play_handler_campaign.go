package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CampaignsActive lists currently active play campaigns.
// GET /api/v1/play/campaigns/active
func (h *PlayHandler) CampaignsActive(c *gin.Context) {
	_, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	campaigns, err := h.playService.ListActiveCampaigns(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]playCampaignSummaryDTO, 0, len(campaigns))
	for _, item := range campaigns {
		out = append(out, toPlayCampaignSummaryDTO(item))
	}
	response.Success(c, out)
}

func toPlayCampaignSummaryDTO(item service.PlayCampaignSummary) playCampaignSummaryDTO {
	return playCampaignSummaryDTO{
		ID:      item.ID,
		Name:    item.Name,
		StartAt: item.StartAt.Format("2006-01-02T15:04:05Z07:00"),
		EndAt:   item.EndAt.Format("2006-01-02T15:04:05Z07:00"),
		Rules: playCampaignRulesDTO{
			RechargeBonusPct:     item.Rules.RechargeBonusPct,
			BlindboxExtraOpens:   item.Rules.BlindboxExtraOpens,
			ArenaScoreMultiplier: item.Rules.ArenaScoreMultiplier,
		},
	}
}

func toPlayCampaignSummaryDTOs(items []service.PlayCampaignSummary) []playCampaignSummaryDTO {
	if len(items) == 0 {
		return nil
	}
	out := make([]playCampaignSummaryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toPlayCampaignSummaryDTO(item))
	}
	return out
}
