package handler

import (
	"strconv"

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

// Claim uses only the authenticated subject; clients cannot select a recipient.
func (h *CouponWalletHandler) Claim(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var input struct {
		Code string `json:"code" binding:"required,max=64"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid coupon code")
		return
	}
	coupon, err := h.couponService.ClaimCode(c.Request.Context(), input.Code, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, coupon)
}

func (h *CouponWalletHandler) GetMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid coupon id")
		return
	}
	coupon, err := h.couponService.GetUserCouponForUser(c.Request.Context(), id, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, coupon)
}
