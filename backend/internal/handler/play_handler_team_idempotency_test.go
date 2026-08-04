package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTeamWriteOptionalKeyRemainsUsableDuringStrictEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	cfg := service.DefaultIdempotencyConfig()
	cfg.ObserveOnly = false
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(newMobileReplayIdempotencyRepo(), cfg))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })

	called := 0
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})
		c.Next()
	})
	router.POST("/team-write", func(c *gin.Context) {
		executeUserIdempotentJSONOptionalKey(c, mobileUserIdempotencyScope(c, mobileOperationTeamInviteRotate), map[string]bool{"rotate": true}, time.Minute, func(context.Context) (any, error) {
			called++
			return gin.H{"ok": true}, nil
		})
	})

	request := httptest.NewRequest(http.MethodPost, "/team-write", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")
	require.Equal(t, 1, called)
}

func TestTeamWriteIdempotencyKeyIsIsolatedByAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	cfg := service.DefaultIdempotencyConfig()
	cfg.ObserveOnly = false
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(newMobileReplayIdempotencyRepo(), cfg))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })

	called := 0
	router := gin.New()
	router.Use(func(c *gin.Context) {
		userID, err := strconv.ParseInt(c.GetHeader("X-Test-User"), 10, 64)
		require.NoError(t, err)
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.POST("/team-write", func(c *gin.Context) {
		subject, _ := middleware2.GetAuthSubjectFromContext(c)
		executeUserIdempotentJSONOptionalKey(c, mobileUserIdempotencyScope(c, mobileOperationTeamRecruitingUpdate), map[string]bool{"recruiting": true}, time.Minute, func(context.Context) (any, error) {
			called++
			return gin.H{"user_id": subject.UserID}, nil
		})
	})

	perform := func(userID int64) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/team-write", nil)
		request.Header.Set("X-Test-User", strconv.FormatInt(userID, 10))
		request.Header.Set("Idempotency-Key", "shared-team-write-key")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	first := perform(31)
	secondAccount := perform(32)
	replay := perform(31)
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, secondAccount.Code)
	require.Empty(t, secondAccount.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, http.StatusOK, replay.Code)
	require.Equal(t, "true", replay.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 2, called)
}
