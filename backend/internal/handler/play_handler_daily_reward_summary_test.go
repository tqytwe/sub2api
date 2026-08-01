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
	"github.com/stretchr/testify/require"
)

type dailyRewardSummaryHandlerRepo struct {
	service.PlayRepository
	period *service.PlayArenaPeriod
}

type publicTeamRewardShowcaseHandlerRepo struct {
	service.PlayRepository
	limit int
}

func (r *publicTeamRewardShowcaseHandlerRepo) ListPublicTeamRewardWinners(_ context.Context, limit int) ([]service.PlayTeamRewardPublicWinner, error) {
	r.limit = limit
	paidAt := time.Date(2026, time.August, 1, 0, 10, 0, 0, time.UTC)
	return []service.PlayTeamRewardPublicWinner{{
		SettlementID: 71,
		PeriodStart:  time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		TeamName:     "星火小队",
		DisplayName:  "wi***@example.com",
		Amount:       14.16,
		PaidAt:       &paidAt,
	}}, nil
}

func (r *dailyRewardSummaryHandlerRepo) GetLatestSettledDailyArenaPeriod(context.Context) (*service.PlayArenaPeriod, error) {
	return r.period, nil
}

func (r *dailyRewardSummaryHandlerRepo) ListArenaDailyRewardLedger(context.Context, int64) ([]service.PlayArenaDailyRewardLedgerRow, error) {
	return []service.PlayArenaDailyRewardLedgerRow{{
		UserID:      42,
		DisplayName: "wi***@example.com",
		Amount:      0.5,
		Rank:        1,
		TokenSum:    12000,
		CreatedAt:   time.Date(2026, time.July, 21, 0, 8, 0, 0, time.UTC),
	}}, nil
}

func (r *dailyRewardSummaryHandlerRepo) EnsureDailyArenaPeriod(context.Context, time.Time) (*service.PlayArenaPeriod, error) {
	return nil, nil
}

func TestArenaDailyRewardSummaryRouteIsPublicAndPrivacyMasked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settledAt := time.Date(2026, time.July, 21, 0, 8, 0, 0, time.UTC)
	playService := service.NewPlayService(&dailyRewardSummaryHandlerRepo{period: &service.PlayArenaPeriod{
		ID:         44,
		Name:       "2026-07-20",
		StartAt:    time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC),
		EndAt:      time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC),
		Status:     "settled",
		PeriodType: "daily",
		SettledAt:  &settledAt,
	}}, nil, nil, service.NewSettingService(&blindboxHandlerSettingRepo{values: map[string]string{
		service.SettingKeyPlayArenaEnabled:      "true",
		service.SettingKeyPlayDailyArenaEnabled: "true",
	}}, nil), nil, nil)
	playHandler := handler.NewPlayHandler(playService, nil)

	authCalls := 0
	jwtAuth := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})

	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{Play: playHandler}, jwtAuth)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/play/arena/daily/reward-summary", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Zero(t, authCalls)
	body := recorder.Body.String()
	require.Contains(t, body, `"recent"`)
	require.Contains(t, body, `wi***@example.com`)
	require.NotContains(t, body, `"user_id"`)
	require.NotContains(t, strings.ToLower(body), `"email"`)
	require.NotContains(t, body, `winner@example.com`)
}

func TestTeamRewardShowcaseRouteIsPublicAndPrivacyMasked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &publicTeamRewardShowcaseHandlerRepo{}
	playHandler := handler.NewPlayHandler(service.NewPlayService(repo, nil, nil, nil, nil, nil), nil)

	authCalls := 0
	jwtAuth := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.Next()
	})
	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{Play: playHandler}, jwtAuth)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/play/teams/reward-showcase", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Zero(t, authCalls)
	require.Equal(t, 10, repo.limit)
	body := recorder.Body.String()
	require.Contains(t, body, `"team_name":"星火小队"`)
	require.Contains(t, body, `wi***@example.com`)
	require.Contains(t, body, `"amount":14.16`)
	require.NotContains(t, body, `"user_id"`)
	require.NotContains(t, strings.ToLower(body), `"email"`)
}
