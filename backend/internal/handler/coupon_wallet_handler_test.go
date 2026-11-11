package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type couponWalletRepositoryStub struct {
	service.CouponRepository
	service.CouponCampaignRepository
	filter      service.UserCouponListFilter
	claimedUser int64
}

func (r *couponWalletRepositoryStub) ClaimCouponCode(_ context.Context, _ string, id int64, _ time.Time) (*service.UserCoupon, error) {
	r.claimedUser = id
	return &service.UserCoupon{ID: 41, UserID: id}, nil
}
func (r *couponWalletRepositoryStub) GetUserCoupon(context.Context, int64) (*service.UserCoupon, error) {
	return &service.UserCoupon{ID: 41, UserID: 77}, nil
}

func TestCouponWalletClaimIgnoresClientRecipientAndGetMineHidesOthers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &couponWalletRepositoryStub{}
	h := NewCouponWalletHandler(service.NewCouponService(repo))
	for _, test := range []struct {
		user   int64
		status int
	}{{77, http.StatusOK}, {88, http.StatusNotFound}} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/coupons/me/41", nil)
		c.Params = gin.Params{{Key: "id", Value: "41"}}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: test.user})
		h.GetMine(c)
		require.Equal(t, test.status, rec.Code)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/coupons/claim", strings.NewReader(`{"code":"SAVE-TEST","user_id":88}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})
	h.Claim(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(77), repo.claimedUser)
}

func (r *couponWalletRepositoryStub) ExpireAvailableUserCoupons(context.Context, int64, time.Time) error {
	return nil
}

func (r *couponWalletRepositoryStub) ListUserCoupons(_ context.Context, filter service.UserCouponListFilter) ([]service.UserCoupon, int64, error) {
	r.filter = filter
	return []service.UserCoupon{{ID: 41, UserID: filter.UserID, Status: service.UserCouponStatusAvailable}}, 1, nil
}

func TestCouponWalletHandlerListsOnlyAuthenticatedUsersCoupons(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &couponWalletRepositoryStub{}
	h := NewCouponWalletHandler(service.NewCouponService(repo))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/coupons/me?status=available&page=2&page_size=5", nil)
	ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})

	h.ListMine(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(77), repo.filter.UserID)
	require.Equal(t, service.UserCouponStatusAvailable, repo.filter.Status)
	require.Equal(t, 2, repo.filter.Page)
	require.Equal(t, 5, repo.filter.PageSize)
}

func TestCouponWalletHandlerRejectsUnauthenticatedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/coupons/me", nil)

	NewCouponWalletHandler(nil).ListMine(ctx)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
