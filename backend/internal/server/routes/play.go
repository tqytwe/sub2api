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

	play := v1.Group("/play")
	{
		play.GET("/arena/current", h.Play.ArenaCurrent)
		play.GET("/arena/leaderboard", h.Play.ArenaLeaderboard)
		play.GET("/arena/daily/current", h.Play.ArenaDailyCurrent)
		play.GET("/arena/daily/leaderboard", h.Play.ArenaDailyLeaderboard)
		play.GET("/quiz/today", h.Play.QuizToday)
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
			quiz.POST("/submit", h.Play.QuizSubmit)
		}

		teams := authenticated.Group("/play/teams")
		{
			teams.GET("/me", h.Play.TeamMe)
			teams.POST("", h.Play.TeamCreate)
			teams.POST("/join", h.Play.TeamJoin)
		}

		authenticated.GET("/play/hub", h.Play.Hub)
		authenticated.GET("/play/quests/today", h.Play.QuestsToday)
		authenticated.GET("/play/campaigns/active", h.Play.CampaignsActive)
	}
}
