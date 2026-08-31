package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userSubscriptionProgressRouteRepo struct {
	service.UserSubscriptionRepository
	subscription *service.UserSubscription
}

func (r userSubscriptionProgressRouteRepo) GetByID(_ context.Context, id int64) (*service.UserSubscription, error) {
	if r.subscription == nil || r.subscription.ID != id {
		return nil, service.ErrSubscriptionNotFound
	}
	copy := *r.subscription
	return &copy, nil
}

func newUserSubscriptionProgressRouteRouter(userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: userID})
		c.Next()
	})

	subscriptionService := service.NewSubscriptionService(
		nil,
		userSubscriptionProgressRouteRepo{subscription: &service.UserSubscription{
			ID:        42,
			UserID:    1001,
			GroupID:   9,
			Group:     &service.Group{ID: 9, Name: "Pro"},
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}},
		nil,
		nil,
		nil,
	)

	api := router.Group("/api/v1")
	registerUserSubscriptionRoutes(api.Group("/subscriptions"), &handler.Handlers{
		Subscription: handler.NewSubscriptionHandler(subscriptionService),
	})
	return router
}

func TestUserSubscriptionProgressRouteOnlyReturnsTheCurrentUsersSubscription(t *testing.T) {
	t.Run("owner can read progress", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/42/progress", nil)

		newUserSubscriptionProgressRouteRouter(1001).ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		var payload struct {
			Code int `json:"code"`
			Data struct {
				ID        int64  `json:"id"`
				GroupName string `json:"group_name"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.Equal(t, 0, payload.Code)
		require.Equal(t, int64(42), payload.Data.ID)
		require.Equal(t, "Pro", payload.Data.GroupName)
	})

	t.Run("other users receive the same not-found result as a missing subscription", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/42/progress", nil)

		newUserSubscriptionProgressRouteRouter(2002).ServeHTTP(recorder, request)

		require.Equal(t, http.StatusNotFound, recorder.Code)
		require.Contains(t, recorder.Body.String(), `"reason":"SUBSCRIPTION_NOT_FOUND"`)
	})

	t.Run("invalid identifiers are rejected before a repository lookup", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/not-a-number/progress", nil)

		newUserSubscriptionProgressRouteRouter(1001).ServeHTTP(recorder, request)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}
