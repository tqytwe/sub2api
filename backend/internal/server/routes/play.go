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
) {
	v1.GET("/public/models", h.Play.PublicModels)
	v1.GET("/public/model-pricing", h.ModelPricing.PublicModelPricing)
	v1.GET("/public/home-stats", h.Play.PublicHomeStats)
	v1.GET("/public/activity", h.Play.PublicActivity)

	play := v1.Group("/play")
	{
		play.GET("/arena/current", h.Play.ArenaCurrent)
		play.GET("/arena/leaderboard", h.Play.ArenaLeaderboard)
		play.GET("/arena/daily/current", h.Play.ArenaDailyCurrent)
		play.GET("/arena/daily/leaderboard", h.Play.ArenaDailyLeaderboard)
		play.GET("/arena/summary", h.Play.ArenaSummary)
		play.GET("/arena/daily/summary", h.Play.ArenaDailySummary)
		play.GET("/blindbox/recent", h.Play.BlindboxRecent)
		play.GET("/blindbox/pool", h.Play.BlindboxPool)
		play.GET("/teams/discover", h.Play.TeamDiscover)
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
			teams.GET("/join-requests", h.Play.TeamJoinRequests)
			teams.GET("/activity", h.Play.TeamActivity)
			teams.POST("", h.Play.TeamCreate)
			teams.POST("/join", h.Play.TeamJoin)
			teams.POST("/request", h.Play.TeamRequestJoin)
			teams.POST("/review", h.Play.TeamReviewRequest)
			teams.POST("/leave", h.Play.TeamLeave)
			teams.POST("/transfer", h.Play.TeamTransfer)
			teams.POST("/remove", h.Play.TeamRemoveMember)
		}

		authenticated.GET("/play/hub", h.Play.Hub)
		authenticated.GET("/play/quests/today", h.Play.QuestsToday)
		authenticated.GET("/play/campaigns/active", h.Play.CampaignsActive)
	}
}
