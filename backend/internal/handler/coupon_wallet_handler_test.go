package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type couponWalletRepositoryStub struct {
	service.CouponRepository
	filter service.UserCouponListFilter
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
