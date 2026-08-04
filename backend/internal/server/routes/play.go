package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterPlayRoutes registers play/engagement and public model routes.
func RegisterPlayRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	teamAdmissionRateLimiters ...*middleware.PanelRateLimiter,
) {
	// These writes are commonly retried by mobile clients after transient
	// transport failures. Keep the correlation ID and idempotency policy on the
	// canonical routes rather than creating mobile-only mirrors.
	teamWriteCorrelationID := middleware.ClientRequestID()
	teamAdmissionRateLimit := gin.HandlerFunc(func(c *gin.Context) { c.Next() })
	if len(teamAdmissionRateLimiters) > 0 && teamAdmissionRateLimiters[0] != nil {
		teamAdmissionRateLimit = teamAdmissionRateLimiters[0].TeamAdmission()
	}
	publicTeamCompetitionRateLimit := gin.HandlerFunc(func(c *gin.Context) { c.Next() })
	if len(teamAdmissionRateLimiters) > 0 && teamAdmissionRateLimiters[0] != nil {
		publicTeamCompetitionRateLimit = teamAdmissionRateLimiters[0].PublicIP()
	}
	v1.GET("/public/models", h.Play.PublicModels)
	v1.GET("/public/model-pricing", h.ModelPricing.PublicModelPricing)

	play := v1.Group("/play")
	{
		play.GET("/arena/current", middleware.OptionalJWTAuth(jwtAuth), h.Play.ArenaCurrent)
		play.GET("/arena/leaderboard", h.Play.ArenaLeaderboard)
		play.GET("/arena/overview", middleware.OptionalJWTAuth(jwtAuth), h.Play.ArenaSeasonOverview)
		play.GET("/arena/daily/current", middleware.OptionalJWTAuth(jwtAuth), h.Play.ArenaDailyCurrent)
		play.GET("/arena/daily/leaderboard", h.Play.ArenaDailyLeaderboard)
		play.GET("/arena/daily/reward-summary", h.Play.ArenaDailyRewardSummary)
		play.GET("/arena/reward-summary", h.Play.ArenaRewardSummary)
		play.GET("/teams/reward-showcase", h.Play.TeamRewardShowcase)
		play.GET("/teams/directory", publicTeamCompetitionRateLimit, h.Play.TeamDirectory)
		play.GET("/teams/leaderboard/public", publicTeamCompetitionRateLimit, h.Play.TeamPublicLeaderboard)
		play.GET("/teams/seasons", publicTeamCompetitionRateLimit, h.Play.TeamSeasons)
		play.GET("/teams/seasons/:month", publicTeamCompetitionRateLimit, h.Play.TeamSeason)
		play.GET("/blindbox/pool", h.Play.BlindboxPool)
		play.GET("/blindbox/recent", h.Play.BlindboxRecent)
	}

	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		checkin := authenticated.Group("/play/checkin")
		{
			checkin.GET("/status", h.Play.CheckinStatus)
			checkin.POST("", h.Play.Checkin)
			checkin.POST("/makeup", h.Play.CheckinMakeup)
		}

		blindbox := authenticated.Group("/play/blindbox")
		{
			blindbox.GET("/status", h.Play.BlindboxStatus)
			blindbox.POST("/open", h.Play.BlindboxOpen)
		}

		quiz := authenticated.Group("/play/quiz")
		{
			quiz.GET("/today", h.Play.QuizToday)
			quiz.POST("/submit", h.Play.QuizSubmit)
		}

		teams := authenticated.Group("/play/teams")
		{
			teams.GET("/me", h.Play.TeamMe)
			teams.GET("/admission", h.Play.TeamAdmissionEligibility)
			teams.POST("", teamAdmissionRateLimit, h.Play.TeamCreate)
			teams.POST("/join", teamAdmissionRateLimit, h.Play.TeamJoin)
			teams.POST("/applications", teamWriteCorrelationID, teamAdmissionRateLimit, h.Play.TeamApply)
			teams.GET("/applications/me", h.Play.TeamMyApplications)
			teams.GET("/applications", h.Play.TeamCaptainApplications)
			teams.POST("/applications/:id/withdraw", teamAdmissionRateLimit, h.Play.TeamApplicationWithdraw)
			teams.POST("/applications/:id/decision", teamWriteCorrelationID, teamAdmissionRateLimit, h.Play.TeamApplicationDecision)
			teams.POST("/invite/rotate", teamWriteCorrelationID, teamAdmissionRateLimit, h.Play.TeamInviteRotate)
			teams.PUT("/recruiting", teamWriteCorrelationID, teamAdmissionRateLimit, h.Play.TeamRecruiting)
			teams.POST("/leave", h.Play.TeamLeave)
			teams.POST("/transfer", h.Play.TeamTransfer)
			teams.POST("/remove", h.Play.TeamRemove)
			teams.GET("/settlements", h.Play.TeamSettlements)
			teams.GET("/leaderboard", h.Play.TeamLeaderboard)
		}

		authenticated.GET("/play/hub", h.Play.Hub)
		authenticated.GET("/play/quests/today", h.Play.QuestsToday)
		authenticated.GET("/play/campaigns/active", h.Play.CampaignsActive)
		// The legacy fallback may carry the same correlation/idempotency key as
		// the canonical support request. Preserve its request ID for diagnosis
		// while the handler safely replays a duplicate keyed submission.
		authenticated.POST("/play/mobile-feedback", middleware.ClientRequestID(), h.Play.SubmitMobileFeedback)
	}
}
