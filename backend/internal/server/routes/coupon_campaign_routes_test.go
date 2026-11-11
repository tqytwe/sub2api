package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCouponCampaignRoutesRejectGuestAndOrdinaryUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "coupon-auth-fixture", ExpireHour: 1}}
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive}
	// Reuse the existing administrator-authentication fixture repository.
	repo := &modelBillingAuthUserRepo{user: user}
	token, err := auth.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	router := gin.New()
	admin := router.Group("/api/v1/admin", gin.HandlerFunc(middleware.NewAdminAuthMiddleware(auth, service.NewUserService(repo, nil, nil, nil), nil, nil)))
	registerPromoCodeRoutes(admin, &handler.Handlers{Admin: &handler.AdminHandlers{Coupon: adminhandler.NewCouponHandler(nil)}})
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/templates/1/claim-policy"},
		{http.MethodPut, "/templates/1/claim-policy"},
		{http.MethodPost, "/campaigns/preview"},
		{http.MethodPost, "/campaigns/execute"},
		{http.MethodGet, "/summary"},
		{http.MethodGet, "/claim-policies?ids=1"},
		{http.MethodGet, "/user-coupons/1/events"},
	} {
		for _, loggedIn := range []bool{false, true} {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(route.method, "/api/v1/admin/promo-codes/coupons"+route.path, nil)
			want := http.StatusUnauthorized
			if loggedIn {
				request.Header.Set("Authorization", "Bearer "+token)
				want = http.StatusForbidden
			}
			router.ServeHTTP(recorder, request)
			require.Equal(t, want, recorder.Code, route.path)
		}
	}
}
