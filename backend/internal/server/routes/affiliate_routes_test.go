package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminReferralCampaignFinancialActionsRequireStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Affiliate: adminhandler.NewAffiliateHandler(nil, nil),
		},
	}
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) {
		servermiddleware.AbortWithError(c, http.StatusPreconditionRequired, "STEP_UP_REQUIRED", "step-up required")
	})
	registerAffiliateRoutes(router.Group("/api/v1/admin"), handlers, stepUp)

	for _, path := range []string{
		"/api/v1/admin/affiliates/campaigns/12/status",
		"/api/v1/admin/affiliates/campaigns/12/early-close",
		"/api/v1/admin/affiliates/campaigns/12/reviews",
		"/api/v1/admin/affiliates/campaigns/12/rewards/99/resolve",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusPreconditionRequired, recorder.Code, path)
	}
}
