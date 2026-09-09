//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReferralCampaignAdminErrorsUseStableBusinessResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name       string
		err        error
		wantStatus int
		wantReason string
	}{
		{"invalid state", service.ErrReferralCampaignInvalidState, http.StatusConflict, "REFERRAL_CAMPAIGN_INVALID_STATE"},
		{"version conflict", service.ErrReferralCampaignVersionConflict, http.StatusConflict, "REFERRAL_CAMPAIGN_VERSION_CONFLICT"},
		{"missing close reason", service.ErrReferralCampaignEarlyCloseReason, http.StatusBadRequest, "REFERRAL_CAMPAIGN_EARLY_CLOSE_REASON_REQUIRED"},
		{"persistence failure", infraerrors.InternalServer("REFERRAL_CAMPAIGN_EARLY_CLOSE_FAILED", "unable to close the referral campaign; refresh and try again"), http.StatusInternalServerError, "REFERRAL_CAMPAIGN_EARLY_CLOSE_FAILED"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)

			respondReferralCampaignAdminError(ctx, testCase.err)

			require.Equal(t, testCase.wantStatus, recorder.Code)
			var payload response.Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, testCase.wantReason, payload.Reason)
			require.NotEqual(t, "internal error", payload.Message)
		})
	}
}
