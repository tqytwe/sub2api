package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationsCampaignFilterRepository struct {
	service.AffiliateRepository
	service.ReferralCampaignRepository
	qualification, risk string
	calls               int
}

func (r *operationsCampaignFilterRepository) ListReferralCampaignInvitesFiltered(_ context.Context, _ int64, _, _ int, _, qualification, risk string) (*service.ReferralCampaignInvitePage, error) {
	r.calls++
	r.qualification = qualification
	r.risk = risk
	return &service.ReferralCampaignInvitePage{Items: []service.ReferralCampaignInviteDetail{}}, nil
}

func TestAffiliateOperationsCampaignHTTPKeepsRiskAndQualificationSeparate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &operationsCampaignFilterRepository{}
	h := NewAffiliateHandler(service.NewAffiliateService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.GET("/campaigns/:campaign_id/invites", h.ReferralCampaignInvites)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/campaigns/12/invites?status=qualified&risk_status=rejected", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "qualified", repo.qualification)
	require.Equal(t, "rejected", repo.risk)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/campaigns/12/invites?status=approved&risk_status=qualified", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, 1, repo.calls)
}
