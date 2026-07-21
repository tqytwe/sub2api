package handler

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type playQuestTodayDTO struct {
	Enabled           bool               `json:"enabled"`
	Energy            int                `json:"energy"`
	Level             int                `json:"level"`
	EnergyToNextLevel int                `json:"energy_to_next_level"`
	Tasks             []playQuestTaskDTO `json:"tasks"`
	ServerDate        string             `json:"server_date"`
}

type playQuestTaskDTO struct {
	Key       string `json:"key"`
	Label     string `json:"label,omitempty"`
	Completed bool   `json:"completed"`
	Energy    int    `json:"energy"`
	CTARoute  string `json:"cta_route,omitempty"`
}

type playHubImageStudioDTO struct {
	Enabled         bool `json:"enabled"`
	ImagesToday     int  `json:"images_today"`
	HasCompletedJob bool `json:"has_completed_job"`
}

func (h *PlayHandler) QuestsToday(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	quests, err := h.playService.GetQuestsToday(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayQuestTodayDTO(quests))
}

func (h *PlayHandler) ArenaDailyCurrent(c *gin.Context) {
	var userID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	current, err := h.playService.GetDailyArenaCurrent(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayArenaCurrentDTO(current))
}

func (h *PlayHandler) ArenaDailyLeaderboard(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, period, err := h.playService.ListDailyArenaLeaderboard(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playArenaLeaderboardDTO{Enabled: period != nil, Rows: make([]playArenaScoreDTO, 0, len(rows))}
	if period != nil {
		out.Period = toPlayArenaPeriodDTO(period)
	}
	for _, row := range rows {
		out.Rows = append(out.Rows, playArenaScoreDTO{
			Rank:        row.Rank,
			UserID:      row.UserID,
			DisplayName: row.DisplayName,
			AvatarURL:   row.AvatarURL,
			TokenSum:    row.TokenSum,
		})
	}
	response.Success(c, out)
}

func (h *PlayHandler) ArenaDailyRewardSummary(c *gin.Context) {
	summary, err := h.playService.GetDailyArenaRewardSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayArenaDailyRewardSummaryDTO(summary))
}

func toPlayArenaCurrentDTO(current *service.PlayArenaCurrent) playArenaCurrentDTO {
	if current == nil {
		return playArenaCurrentDTO{}
	}
	out := playArenaCurrentDTO{
		Enabled:              current.Enabled,
		TokenSum:             current.TokenSum,
		DisplayTokenSum:      current.DisplayTokenSum,
		Rank:                 current.Rank,
		TokensToPrevRank:     current.TokensToPrevRank,
		EstimatedReward:      current.EstimatedReward,
		RechargeBoostActive:  current.RechargeBoostActive,
		ArenaScoreMultiplier: current.ArenaScoreMultiplier,
		CampaignActive:       current.CampaignActive,
	}
	if current.Period != nil {
		out.Period = toPlayArenaPeriodDTO(current.Period)
	}
	return out
}

func toPlayArenaDailyRewardSummaryDTO(summary *service.PlayArenaDailyRewardSummary) playArenaDailyRewardSummaryDTO {
	if summary == nil {
		return playArenaDailyRewardSummaryDTO{}
	}
	out := playArenaDailyRewardSummaryDTO{Enabled: summary.Enabled}
	if summary.Recent != nil {
		out.Recent = &playArenaDailyRecentRewardSummaryDTO{
			Period:       toPlayArenaPeriodDTO(summary.Recent.Period),
			SettledAt:    formatOptionalPlayTime(summary.Recent.SettledAt),
			PaidToday:    summary.Recent.PaidToday,
			WinnersCount: summary.Recent.WinnersCount,
			TotalAmount:  summary.Recent.TotalAmount,
			Winners:      make([]playArenaDailyRewardWinnerDTO, 0, len(summary.Recent.Winners)),
		}
		for _, row := range summary.Recent.Winners {
			out.Recent.Winners = append(out.Recent.Winners, playArenaDailyRewardWinnerDTO{
				Rank:        row.Rank,
				UserID:      row.UserID,
				DisplayName: row.DisplayName,
				AvatarURL:   row.AvatarURL,
				TokenSum:    row.TokenSum,
				Amount:      row.Amount,
			})
		}
	}
	if summary.Current != nil {
		out.Current = &playArenaDailyCurrentRewardEstimateDTO{
			Period: toPlayArenaPeriodDTO(summary.Current.Period),
			Rows:   make([]playArenaDailyRewardEstimateDTO, 0, len(summary.Current.Rows)),
		}
		for _, row := range summary.Current.Rows {
			out.Current.Rows = append(out.Current.Rows, playArenaDailyRewardEstimateDTO{
				Rank:            row.Rank,
				UserID:          row.UserID,
				DisplayName:     row.DisplayName,
				AvatarURL:       row.AvatarURL,
				TokenSum:        row.TokenSum,
				EstimatedReward: row.EstimatedReward,
			})
		}
	}
	return out
}

func formatOptionalPlayTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format("2006-01-02T15:04:05Z07:00")
	return &formatted
}

func toPlayQuestTodayDTO(q *service.PlayQuestToday) playQuestTodayDTO {
	if q == nil {
		return playQuestTodayDTO{}
	}
	out := playQuestTodayDTO{
		Enabled:           q.Enabled,
		Energy:            q.Energy,
		Level:             q.Level,
		EnergyToNextLevel: q.EnergyToNextLevel,
		ServerDate:        q.ServerDate,
		Tasks:             make([]playQuestTaskDTO, 0, len(q.Tasks)),
	}
	for _, task := range q.Tasks {
		out.Tasks = append(out.Tasks, playQuestTaskDTO{
			Key:       task.Key,
			Label:     task.Label,
			Completed: task.Completed,
			Energy:    task.Energy,
			CTARoute:  task.CTARoute,
		})
	}
	return out
}

func toPlayHubImageStudioDTO(s *service.PlayHubImageStudio) *playHubImageStudioDTO {
	if s == nil {
		return nil
	}
	return &playHubImageStudioDTO{
		Enabled:         s.Enabled,
		ImagesToday:     s.ImagesToday,
		HasCompletedJob: s.HasCompletedJob,
	}
}
