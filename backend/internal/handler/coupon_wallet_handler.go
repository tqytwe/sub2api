package handler

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CouponWalletHandler exposes only the authenticated user's coupons. Coupon
// templates and cross-user issuance remain under the admin promo-code area.
type CouponWalletHandler struct {
	couponService *service.CouponService
}

func NewCouponWalletHandler(couponService *service.CouponService) *CouponWalletHandler {
	return &CouponWalletHandler{couponService: couponService}
}

// ListMine GET /coupons/me
func (h *CouponWalletHandler) ListMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.couponService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("COUPON_SERVICE_UNAVAILABLE", "coupon service is unavailable"))
		return
	}
	page, pageSize := response.ParsePagination(c)
	rows, result, err := h.couponService.ListUserCoupons(c.Request.Context(), service.UserCouponListFilter{
		UserID:   subject.UserID,
		Status:   service.UserCouponStatus(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}
