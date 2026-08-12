package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var (
	ErrMobilePlayBillingNotConfigured       = service.ErrMobilePlayBillingNotConfigured
	ErrMobilePlayBillingVerificationFailed  = service.ErrMobilePlayBillingVerificationFailed
	ErrMobilePlayBillingDuplicatePurchase   = service.ErrMobilePlayBillingDuplicatePurchase
	ErrMobilePlayBillingProductNotMapped    = service.ErrMobilePlayBillingProductNotMapped
	ErrMobilePlayBillingPurchaseNotApproved = service.ErrMobilePlayBillingPurchaseNotApproved
)

type MobilePlayBillingPurchaseInput = service.MobilePlayBillingPurchaseInput
type MobilePlayBillingPurchaseResult = service.MobilePlayBillingPurchaseResult

type MobilePlayBillingVerifier interface {
	VerifyMobilePlayBillingPurchase(ctx context.Context, userID int64, input MobilePlayBillingPurchaseInput) (*MobilePlayBillingPurchaseResult, error)
}

type MobilePlayBillingHandler struct {
	verifier MobilePlayBillingVerifier
}

func NewMobilePlayBillingHandler(verifier ...MobilePlayBillingVerifier) *MobilePlayBillingHandler {
	var selected MobilePlayBillingVerifier
	if len(verifier) > 0 {
		selected = verifier[0]
	}
	return &MobilePlayBillingHandler{verifier: selected}
}

func (h *MobilePlayBillingHandler) SubmitPurchase(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req MobilePlayBillingPurchaseInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Play Billing 请求格式不正确")
		return
	}
	if req.ProductID == "" || req.PurchaseToken == "" || req.ClientRequestID == "" {
		response.BadRequest(c, "product_id、purchase_token 和 client_request_id 不能为空")
		return
	}
	if h == nil || h.verifier == nil {
		mobilePlayBillingError(c, ErrMobilePlayBillingNotConfigured)
		return
	}
	result, err := h.verifier.VerifyMobilePlayBillingPurchase(c.Request.Context(), subject.UserID, req)
	if err != nil {
		mobilePlayBillingError(c, err)
		return
	}
	if result == nil {
		mobilePlayBillingError(c, ErrMobilePlayBillingVerificationFailed)
		return
	}
	response.Success(c, result)
}

func mobilePlayBillingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMobilePlayBillingNotConfigured):
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "Play Billing 服务端验单未配置", "PLAY_BILLING_NOT_CONFIGURED", nil)
	case errors.Is(err, ErrMobilePlayBillingVerificationFailed):
		response.ErrorWithDetails(c, http.StatusBadGateway, "Google Play 购买验单失败", "PLAY_BILLING_VERIFICATION_FAILED", nil)
	case errors.Is(err, ErrMobilePlayBillingDuplicatePurchase):
		response.ErrorWithDetails(c, http.StatusConflict, "Google Play 购买已处理", "PLAY_BILLING_DUPLICATE_PURCHASE", nil)
	case errors.Is(err, ErrMobilePlayBillingProductNotMapped):
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Google Play 商品未映射到账户权益", "PLAY_BILLING_PRODUCT_NOT_MAPPED", nil)
	case errors.Is(err, ErrMobilePlayBillingPurchaseNotApproved):
		response.ErrorWithDetails(c, http.StatusAccepted, "Google Play 购买尚未完成确认", "PLAY_BILLING_PURCHASE_NOT_APPROVED", nil)
	default:
		response.ErrorWithDetails(c, http.StatusInternalServerError, "Play Billing 处理失败", "PLAY_BILLING_INTERNAL_ERROR", nil)
	}
}
