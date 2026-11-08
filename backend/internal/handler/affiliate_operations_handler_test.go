package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operationsOwnershipRepository struct {
	service.AffiliateRepository
	service.AffiliateOperationsRepository
	query service.AffiliateOperationsQuery
	calls int
}

func (r *operationsOwnershipRepository) ListAffiliateOperationsEarnings(_ context.Context, q service.AffiliateOperationsQuery) (*service.AffiliateOperationsPage[service.AffiliateOperationsEarning], error) {
	r.calls++
	r.query = q
	return &service.AffiliateOperationsPage[service.AffiliateOperationsEarning]{Items: []service.AffiliateOperationsEarning{}, Page: q.Page, PageSize: q.PageSize}, nil
}

func TestAffiliateOperationsHTTPUsesSubjectAndRejectsInvalidFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &operationsOwnershipRepository{}
	h := &UserHandler{affiliateService: service.NewAffiliateService(repo, nil, nil, nil)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-Subject") == "yes" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		c.Next()
	})
	router.GET("/user/aff/operations/earnings", h.AffiliateOperationsRead)
	request := httptest.NewRequest(http.MethodGet, "/user/aff/operations/earnings?user_id=999&invitee_id=888", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, repo.calls)
	request = httptest.NewRequest(http.MethodGet, "/user/aff/operations/earnings?user_id=999&invitee_id=888", nil)
	request.Header.Set("X-Test-Subject", "yes")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.EqualValues(t, 42, repo.query.UserID)
	require.Zero(t, repo.query.InviteeID)
	require.False(t, repo.query.Admin)
	request = httptest.NewRequest(http.MethodGet, "/user/aff/operations/earnings?page=broken", nil)
	request.Header.Set("X-Test-Subject", "yes")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, 1, repo.calls)
}
