package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type publicTeamCompetitionHandlerRepo struct {
	service.PlayRepository
}

func (r *publicTeamCompetitionHandlerRepo) ListPublicTeamDirectory(context.Context, time.Time, time.Time, int) ([]service.PlayTeamDirectoryBase, error) {
	return []service.PlayTeamDirectoryBase{{
		TeamID:      11,
		TeamName:    "星火战队",
		MemberCount: 8,
		Recruiting:  true,
		Spend:       decimal.RequireFromString("120.50000000"),
	}}, nil
}

func (r *publicTeamCompetitionHandlerRepo) ListPublicTeamLeaderboard(context.Context, time.Time, time.Time, int) ([]service.PlayTeamPublicLeaderboardBase, int, error) {
	return []service.PlayTeamPublicLeaderboardBase{{
		Rank:        1,
		TeamID:      11,
		TeamName:    "星火战队",
		MemberCount: 8,
		Spend:       decimal.RequireFromString("120.50000000"),
	}}, 1, nil
}

func (r *publicTeamCompetitionHandlerRepo) ListPublicTeamSeasons(context.Context, int) ([]service.PlayTeamSeason, error) {
	return []service.PlayTeamSeason{{Month: "2026-07", Status: "settled"}}, nil
}

func (r *publicTeamCompetitionHandlerRepo) GetPublicTeamSeason(context.Context, time.Time, int) (*service.PlayTeamSeason, []service.PlayTeamSeasonRanking, int, error) {
	return &service.PlayTeamSeason{Month: "2026-07", Status: "settled"}, []service.PlayTeamSeasonRanking{{
		Rank:       1,
		TeamID:     11,
		TeamName:   "星火战队",
		TeamSpend:  decimal.RequireFromString("120.50000000"),
		PoolAmount: decimal.RequireFromString("6.02500000"),
	}}, 1, nil
}

func TestPublicTeamCompetitionRoutesAreUnauthenticatedAndDoNotExposeSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playHandler := handler.NewPlayHandler(service.NewPlayService(&publicTeamCompetitionHandlerRepo{}, nil, nil, nil, nil, nil), nil)
	authCalls := 0
	jwtAuth := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{Play: playHandler}, jwtAuth)

	for _, path := range []string{
		"/api/v1/play/teams/directory",
		"/api/v1/play/teams/leaderboard/public",
		"/api/v1/play/teams/seasons/2026-07",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, path)
		require.Contains(t, recorder.Body.String(), "星火战队")
		require.NotContains(t, strings.ToLower(recorder.Body.String()), "invite_code")
		require.NotContains(t, recorder.Body.String(), "user_id")
	}

	// The historical index intentionally exposes only immutable season metadata.
	// Rankings stay behind the selected season endpoint above so a first paint
	// cannot accidentally include members or any other team-level detail.
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/play/teams/seasons", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "2026-07")
	require.NotContains(t, recorder.Body.String(), "星火战队")
	require.NotContains(t, strings.ToLower(recorder.Body.String()), "invite_code")
	require.NotContains(t, recorder.Body.String(), "user_id")
	require.Zero(t, authCalls)
}
