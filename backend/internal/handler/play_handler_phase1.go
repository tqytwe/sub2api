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

type playArenaSeasonHistoryWinnerDTO struct {
	Rank         int     `json:"rank"`
	DisplayName  string  `json:"display_name,omitempty"`
	Anonymous    bool    `json:"anonymous,omitempty"`
	AvatarURL    string  `json:"avatar_url,omitempty"`
	TokenSum     int64   `json:"token_sum"`
	RewardAmount float64 `json:"reward_amount"`
	PayoutStatus string  `json:"payout_status"`
	PaidAt       *string `json:"paid_at,omitempty"`
}

type playArenaSeasonHistoryDTO struct {
	Period       *playArenaPeriodDTO               `json:"period,omitempty"`
	WinnersCount int                               `json:"winners_count"`
	TotalAmount  float64                           `json:"total_amount"`
	Winners      []playArenaSeasonHistoryWinnerDTO `json:"winners"`
}

type playArenaSeasonOverviewDTO struct {
	Enabled     bool                              `json:"enabled"`
	Period      *playArenaPeriodDTO               `json:"period,omitempty"`
	Current     playArenaCurrentDTO               `json:"current"`
	Rows        []playArenaScoreDTO               `json:"rows"`
	RewardTiers []service.PlayArenaSettlementTier `json:"reward_tiers"`
	History     []playArenaSeasonHistoryDTO       `json:"history"`
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

// ArenaSeasonOverview serves one selected daily or monthly tab. It is public
// for competition proof and optionally augments only the viewer's own state.
func (h *PlayHandler) ArenaSeasonOverview(c *gin.Context) {
	periodType := c.DefaultQuery("period", "monthly")
	// The current board is the page's first-screen payload. Historical proof is
	// immutable but comparatively expensive, so clients can request it only
	// after the user expands the history section.
	includeHistory := c.DefaultQuery("include_history", "1") != "0"
	var userID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	overview, err := h.playService.GetArenaSeasonOverview(c.Request.Context(), userID, periodType, includeHistory)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playArenaSeasonOverviewDTO{
		Enabled:     overview.Enabled,
		Current:     toPlayArenaCurrentDTO(overview.Current),
		Rows:        make([]playArenaScoreDTO, 0, len(overview.Rows)),
		RewardTiers: append([]service.PlayArenaSettlementTier(nil), overview.RewardTiers...),
		History:     make([]playArenaSeasonHistoryDTO, 0, len(overview.History)),
	}
	if overview.Period != nil {
		out.Period = toPlayArenaPeriodDTO(overview.Period)
	}
	for _, row := range overview.Rows {
		out.Rows = append(out.Rows, toPlayArenaScoreDTO(row))
	}
	for _, history := range overview.History {
		item := playArenaSeasonHistoryDTO{
			Period:       toPlayArenaPeriodDTO(&history.Period),
			WinnersCount: history.WinnersCount,
			TotalAmount:  history.TotalAmount,
			Winners:      make([]playArenaSeasonHistoryWinnerDTO, 0, len(history.Winners)),
		}
		for _, winner := range history.Winners {
			item.Winners = append(item.Winners, playArenaSeasonHistoryWinnerDTO{
				Rank:         winner.Rank,
				DisplayName:  winner.DisplayName,
				Anonymous:    winner.Anonymous,
				AvatarURL:    winner.AvatarURL,
				TokenSum:     winner.TokenSum,
				RewardAmount: winner.RewardAmount,
				PayoutStatus: winner.PayoutStatus,
				PaidAt:       formatOptionalPlayTime(winner.PaidAt),
			})
		}
		out.History = append(out.History, item)
	}
	response.Success(c, out)
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
		out.Rows = append(out.Rows, toPlayArenaScoreDTO(row))
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

func (h *PlayHandler) ArenaRewardSummary(c *gin.Context) {
	summary, err := h.playService.GetMonthlyArenaRewardSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playArenaMonthlyRewardSummaryDTO{Enabled: summary.Enabled, WinnersCount: summary.WinnersCount, TotalAmount: summary.TotalAmount, Winners: make([]playArenaMonthlyRewardWinnerDTO, 0, len(summary.Winners))}
	if summary.Period != nil {
		out.Period = toPlayArenaPeriodDTO(summary.Period)
	}
	out.SettledAt = formatOptionalPlayTime(summary.SettledAt)
	for _, row := range summary.Winners {
		out.Winners = append(out.Winners, playArenaMonthlyRewardWinnerDTO{Rank: row.Rank, DisplayName: row.DisplayName, Anonymous: row.Anonymous, AvatarURL: row.AvatarURL, Amount: row.Amount, PaidAt: formatOptionalPlayTime(row.PaidAt)})
	}
	response.Success(c, out)
}

func (h *PlayHandler) TeamRewardShowcase(c *gin.Context) {
	winners, err := h.playService.ListPublicTeamRewardWinners(c.Request.Context(), 50)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playTeamRewardShowcaseDTO{Winners: make([]playTeamRewardShowcaseWinnerDTO, 0, len(winners))}
	for _, row := range winners {
		out.Winners = append(out.Winners, playTeamRewardShowcaseWinnerDTO{SettlementMonth: row.PeriodStart.Format("2006-01"), TeamName: row.TeamName, DisplayName: row.DisplayName, AvatarURL: row.AvatarURL, Amount: row.Amount, PaidAt: formatOptionalPlayTime(row.PaidAt)})
	}
	response.Success(c, out)
}

func (h *PlayHandler) TeamDirectory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	payload, etag, err := h.publicCompetitionCache.load("directory:"+strconv.Itoa(limit), func() (any, error) {
		return h.playService.PublicTeamDirectory(c.Request.Context(), limit)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondPublicTeamCompetition(c, payload, etag)
}

func (h *PlayHandler) TeamPublicLeaderboard(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	payload, etag, err := h.publicCompetitionCache.load("leaderboard:"+strconv.Itoa(limit), func() (any, error) {
		return h.playService.PublicTeamLeaderboard(c.Request.Context(), limit)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondPublicTeamCompetition(c, payload, etag)
}

func (h *PlayHandler) TeamSeasons(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))
	payload, etag, err := h.publicCompetitionCache.load("seasons:"+strconv.Itoa(limit), func() (any, error) {
		return h.playService.ListPublicTeamSeasons(c.Request.Context(), limit)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondPublicTeamCompetition(c, payload, etag)
}

func (h *PlayHandler) TeamSeason(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	month := c.Param("month")
	payload, etag, err := h.publicCompetitionCache.load("season:"+month+":"+strconv.Itoa(limit), func() (any, error) {
		return h.playService.GetPublicTeamSeason(c.Request.Context(), month, limit)
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondPublicTeamCompetition(c, payload, etag)
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
				DisplayName: row.DisplayName,
				Anonymous:   row.Anonymous,
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
				DisplayName:     row.DisplayName,
				Anonymous:       row.Anonymous,
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
