package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles user-facing payment requests.
type PaymentHandler struct {
	paymentService *service.PaymentService
	configService  *service.PaymentConfigService

	checkoutCacheMu sync.Mutex
	checkoutCache   paymentCheckoutPublicCache
	checkoutCacheSF singleflight.Group
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(paymentService *service.PaymentService, configService *service.PaymentConfigService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		configService:  configService,
	}
}

const paymentCheckoutPublicCacheTTL = 15 * time.Second

type paymentCheckoutPublicCache struct {
	expiresAt time.Time
	payload   *paymentCheckoutPublicPayload
}

type paymentCheckoutPublicPayload struct {
	methods                       map[string]service.MethodLimits
	globalMin                     float64
	globalMax                     float64
	plans                         []checkoutPlan
	balanceDisabled               bool
	balanceRechargeMultiplier     float64
	subscriptionUSDToCNYRate      float64
	rechargeFeeRate               float64
	storefrontConfig              *service.PaymentStorefrontConfig
	playBillingProducts           []service.MobilePlayBillingPublicProduct
	helpText                      string
	helpImageURL                  string
	stripePublishableKey          string
	alipayForceQRCode             bool
	alipayMobilePrecreateDeepLink bool
}

// GetPaymentConfig returns the payment system configuration.
// GET /api/v1/payment/config
func (h *PaymentHandler) GetPaymentConfig(c *gin.Context) {
	cfg, err := h.configService.GetPaymentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// GetPlans returns subscription plans available for sale.
// GET /api/v1/payment/plans
func (h *PaymentHandler) GetPlans(c *gin.Context) {
	plans, err := h.configService.ListPlansForSale(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildPaymentPlansForResponse(c.Request.Context(), h.configService, plans))
}

type paymentPlanResult struct {
	ID                 int64    `json:"id"`
	GroupID            int64    `json:"group_id"`
	GroupPlatform      string   `json:"group_platform"`
	GroupName          string   `json:"group_name"`
	RateMultiplier     float64  `json:"rate_multiplier"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakStart          string   `json:"peak_start"`
	PeakEnd            string   `json:"peak_end"`
	PeakRateMultiplier float64  `json:"peak_rate_multiplier"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Price              float64  `json:"price"`
	OriginalPrice      *float64 `json:"original_price,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	ValidityDays       int      `json:"validity_days"`
	ValidityUnit       string   `json:"validity_unit"`
	Features           string   `json:"features"`
	ProductName        string   `json:"product_name"`
	CoverImageURL      string   `json:"cover_image_url"`
	DetailDescription  string   `json:"detail_description"`
	StorefrontPlatform string   `json:"storefront_platform"`
	StorefrontCategory string   `json:"storefront_category"`
	StorefrontFeatured bool     `json:"storefront_featured"`
	StorefrontBadge    string   `json:"storefront_badge"`
	ForSale            bool     `json:"for_sale"`
	SortOrder          int      `json:"sort_order"`
}

func buildPaymentPlansForResponse(ctx context.Context, configService *service.PaymentConfigService, plans []*dbent.SubscriptionPlan) []paymentPlanResult {
	groupInfo := configService.GetGroupInfoMap(ctx, plans)
	result := make([]paymentPlanResult, 0, len(plans))
	for _, p := range plans {
		gi := groupInfo[p.GroupID]
		result = append(result, paymentPlanResult{
			ID: int64(p.ID), GroupID: p.GroupID,
			GroupPlatform: gi.Platform, GroupName: gi.Name,
			RateMultiplier: gi.RateMultiplier, PeakRateEnabled: gi.PeakRateEnabled,
			PeakStart: gi.PeakStart, PeakEnd: gi.PeakEnd, PeakRateMultiplier: gi.PeakRateMultiplier,
			Name: p.Name, Description: p.Description, Price: p.Price, OriginalPrice: p.OriginalPrice,
			Currency:     p.Currency,
			ValidityDays: p.ValidityDays, ValidityUnit: p.ValidityUnit, Features: p.Features,
			ProductName: p.ProductName, CoverImageURL: p.CoverImageURL, DetailDescription: p.DetailDescription,
			StorefrontPlatform: p.StorefrontPlatform, StorefrontCategory: p.StorefrontCategory,
			StorefrontFeatured: p.StorefrontFeatured, StorefrontBadge: p.StorefrontBadge,
			ForSale: p.ForSale, SortOrder: p.SortOrder,
		})
	}
	return result
}

// GetCheckoutInfo returns all data the payment page needs in a single call:
// payment methods with limits, subscription plans, and configuration.
// GET /api/v1/payment/checkout-info
func (h *PaymentHandler) GetCheckoutInfo(c *gin.Context) {
	ctx := c.Request.Context()
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	publicPayload, err := h.getPaymentCheckoutPublicPayload(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	rechargeQuote, err := h.paymentService.BuildRechargeQuoteWithMultiplier(ctx, subject.UserID, 0, publicPayload.balanceRechargeMultiplier)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, publicPayload.withRechargeQuote(rechargeQuote))
}

func (h *PaymentHandler) getPaymentCheckoutPublicPayload(ctx context.Context) (*paymentCheckoutPublicPayload, error) {
	now := time.Now()
	h.checkoutCacheMu.Lock()
	if h.checkoutCache.payload != nil && now.Before(h.checkoutCache.expiresAt) {
		payload := h.checkoutCache.payload.clone()
		h.checkoutCacheMu.Unlock()
		return payload, nil
	}
	h.checkoutCacheMu.Unlock()

	value, err, _ := h.checkoutCacheSF.Do("payment-checkout-public", func() (any, error) {
		// A request may have populated the cache while this caller was waiting
		// for the singleflight slot, so always check it again inside the flight.
		now := time.Now()
		h.checkoutCacheMu.Lock()
		if h.checkoutCache.payload != nil && now.Before(h.checkoutCache.expiresAt) {
			payload := h.checkoutCache.payload.clone()
			h.checkoutCacheMu.Unlock()
			return payload, nil
		}
		h.checkoutCacheMu.Unlock()

		payload, buildErr := h.buildPaymentCheckoutPublicPayload(ctx)
		if buildErr != nil {
			return nil, buildErr
		}

		h.checkoutCacheMu.Lock()
		h.checkoutCache = paymentCheckoutPublicCache{
			expiresAt: now.Add(paymentCheckoutPublicCacheTTL),
			payload:   payload.clone(),
		}
		h.checkoutCacheMu.Unlock()
		return payload, nil
	})
	if err != nil {
		return nil, err
	}
	payload, ok := value.(*paymentCheckoutPublicPayload)
	if !ok {
		return nil, fmt.Errorf("payment checkout cache returned %T", value)
	}
	return payload.clone(), nil
}

func (h *PaymentHandler) buildPaymentCheckoutPublicPayload(ctx context.Context) (*paymentCheckoutPublicPayload, error) {
	limitsResp, err := h.configService.GetAvailableMethodLimits(ctx)
	if err != nil {
		return nil, err
	}

	cfg, err := h.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}

	alipayMobilePrecreateDeepLink := false
	if cfg.AlipayMobilePrecreateDeepLink {
		alipayMobilePrecreateDeepLink, err = h.configService.UsesOfficialAlipayVisibleMethod(ctx)
		if err != nil {
			return nil, err
		}
	}

	plans, err := h.configService.ListPlansForSale(ctx)
	if err != nil {
		return nil, err
	}
	storefrontConfig, err := h.configService.GetPublicPaymentStorefrontConfig(ctx, plans)
	if err != nil {
		return nil, err
	}
	groupInfo := h.configService.GetGroupInfoMap(ctx, plans)
	planList := make([]checkoutPlan, 0, len(plans))
	for _, p := range plans {
		gi := groupInfo[p.GroupID]
		planList = append(planList, checkoutPlan{
			ID: int64(p.ID), GroupID: p.GroupID,
			GroupPlatform: gi.Platform, GroupName: gi.Name,
			RateMultiplier:  gi.RateMultiplier,
			PeakRateEnabled: gi.PeakRateEnabled, PeakStart: gi.PeakStart,
			PeakEnd: gi.PeakEnd, PeakRateMultiplier: gi.PeakRateMultiplier,
			DailyLimitUSD:  gi.DailyLimitUSD,
			WeeklyLimitUSD: gi.WeeklyLimitUSD, MonthlyLimitUSD: gi.MonthlyLimitUSD,
			ModelScopes: gi.ModelScopes,
			Name:        p.Name, Description: p.Description, Price: p.Price, OriginalPrice: p.OriginalPrice,
			Currency:     p.Currency,
			ValidityDays: p.ValidityDays, ValidityUnit: p.ValidityUnit, Features: parseFeatures(p.Features),
			ProductName: p.ProductName, CoverImageURL: p.CoverImageURL, DetailDescription: p.DetailDescription,
			StorefrontPlatform: p.StorefrontPlatform, StorefrontCategory: p.StorefrontCategory,
			StorefrontFeatured: p.StorefrontFeatured, StorefrontBadge: p.StorefrontBadge,
		})
	}

	return &paymentCheckoutPublicPayload{
		methods:                       cloneMethodLimitsMap(limitsResp.Methods),
		globalMin:                     limitsResp.GlobalMin,
		globalMax:                     limitsResp.GlobalMax,
		plans:                         cloneCheckoutPlans(planList),
		balanceDisabled:               cfg.BalanceDisabled,
		balanceRechargeMultiplier:     cfg.BalanceRechargeMultiplier,
		subscriptionUSDToCNYRate:      cfg.SubscriptionUSDToCNYRate,
		rechargeFeeRate:               cfg.RechargeFeeRate,
		storefrontConfig:              clonePaymentStorefrontConfig(storefrontConfig),
		playBillingProducts:           h.configService.PublicMobilePlayBillingProducts(ctx),
		helpText:                      cfg.HelpText,
		helpImageURL:                  cfg.HelpImageURL,
		stripePublishableKey:          cfg.StripePublishableKey,
		alipayForceQRCode:             cfg.AlipayForceQRCode,
		alipayMobilePrecreateDeepLink: alipayMobilePrecreateDeepLink,
	}, nil
}

func (p *paymentCheckoutPublicPayload) withRechargeQuote(rechargeQuote *service.PaymentRechargeQuote) checkoutInfoResponse {
	return checkoutInfoResponse{
		Methods:                       cloneMethodLimitsMap(p.methods),
		GlobalMin:                     p.globalMin,
		GlobalMax:                     p.globalMax,
		Plans:                         cloneCheckoutPlans(p.plans),
		BalanceDisabled:               p.balanceDisabled,
		BalanceRechargeMultiplier:     p.balanceRechargeMultiplier,
		SubscriptionUSDToCNYRate:      p.subscriptionUSDToCNYRate,
		RechargeFeeRate:               p.rechargeFeeRate,
		StorefrontConfig:              clonePaymentStorefrontConfig(p.storefrontConfig),
		PlayBillingProducts:           cloneMobilePlayBillingPublicProducts(p.playBillingProducts),
		HelpText:                      p.helpText,
		HelpImageURL:                  p.helpImageURL,
		StripePublishableKey:          p.stripePublishableKey,
		AlipayForceQRCode:             p.alipayForceQRCode,
		AlipayMobilePrecreateDeepLink: p.alipayMobilePrecreateDeepLink,
		RechargeQuote:                 rechargeQuote,
	}
}

func (p *paymentCheckoutPublicPayload) clone() *paymentCheckoutPublicPayload {
	if p == nil {
		return nil
	}
	return &paymentCheckoutPublicPayload{
		methods:                       cloneMethodLimitsMap(p.methods),
		globalMin:                     p.globalMin,
		globalMax:                     p.globalMax,
		plans:                         cloneCheckoutPlans(p.plans),
		balanceDisabled:               p.balanceDisabled,
		balanceRechargeMultiplier:     p.balanceRechargeMultiplier,
		subscriptionUSDToCNYRate:      p.subscriptionUSDToCNYRate,
		rechargeFeeRate:               p.rechargeFeeRate,
		storefrontConfig:              clonePaymentStorefrontConfig(p.storefrontConfig),
		playBillingProducts:           cloneMobilePlayBillingPublicProducts(p.playBillingProducts),
		helpText:                      p.helpText,
		helpImageURL:                  p.helpImageURL,
		stripePublishableKey:          p.stripePublishableKey,
		alipayForceQRCode:             p.alipayForceQRCode,
		alipayMobilePrecreateDeepLink: p.alipayMobilePrecreateDeepLink,
	}
}

func cloneMethodLimitsMap(in map[string]service.MethodLimits) map[string]service.MethodLimits {
	if in == nil {
		return nil
	}
	out := make(map[string]service.MethodLimits, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneCheckoutPlans(in []checkoutPlan) []checkoutPlan {
	if in == nil {
		return nil
	}
	out := make([]checkoutPlan, len(in))
	copy(out, in)
	for i := range out {
		out[i].Features = append([]string(nil), in[i].Features...)
		out[i].ModelScopes = append([]string(nil), in[i].ModelScopes...)
	}
	return out
}

func clonePaymentStorefrontConfig(in *service.PaymentStorefrontConfig) *service.PaymentStorefrontConfig {
	if in == nil {
		return nil
	}
	out := &service.PaymentStorefrontConfig{
		Shelves: make([]service.PaymentStorefrontShelf, len(in.Shelves)),
		Tags:    make([]service.PaymentStorefrontTag, len(in.Tags)),
	}
	copy(out.Shelves, in.Shelves)
	copy(out.Tags, in.Tags)
	for i := range out.Shelves {
		out.Shelves[i].PlanIDs = append([]int64(nil), in.Shelves[i].PlanIDs...)
	}
	for i := range out.Tags {
		out.Tags[i].PlanIDs = append([]int64(nil), in.Tags[i].PlanIDs...)
	}
	return out
}

func cloneMobilePlayBillingPublicProducts(in []service.MobilePlayBillingPublicProduct) []service.MobilePlayBillingPublicProduct {
	if in == nil {
		return nil
	}
	out := make([]service.MobilePlayBillingPublicProduct, len(in))
	copy(out, in)
	return out
}

type checkoutInfoResponse struct {
	Methods                       map[string]service.MethodLimits          `json:"methods"`
	GlobalMin                     float64                                  `json:"global_min"`
	GlobalMax                     float64                                  `json:"global_max"`
	Plans                         []checkoutPlan                           `json:"plans"`
	BalanceDisabled               bool                                     `json:"balance_disabled"`
	BalanceRechargeMultiplier     float64                                  `json:"balance_recharge_multiplier"`
	SubscriptionUSDToCNYRate      float64                                  `json:"subscription_usd_to_cny_rate"`
	RechargeFeeRate               float64                                  `json:"recharge_fee_rate"`
	StorefrontConfig              *service.PaymentStorefrontConfig         `json:"storefront_config"`
	PlayBillingProducts           []service.MobilePlayBillingPublicProduct `json:"play_billing_products,omitempty"`
	HelpText                      string                                   `json:"help_text"`
	HelpImageURL                  string                                   `json:"help_image_url"`
	StripePublishableKey          string                                   `json:"stripe_publishable_key"`
	AlipayForceQRCode             bool                                     `json:"alipay_force_qrcode"`
	AlipayMobilePrecreateDeepLink bool                                     `json:"alipay_mobile_precreate_deep_link"`
	RechargeQuote                 *service.PaymentRechargeQuote            `json:"recharge_quote,omitempty"`
}

type checkoutPlan struct {
	ID                 int64    `json:"id"`
	GroupID            int64    `json:"group_id"`
	GroupPlatform      string   `json:"group_platform"`
	GroupName          string   `json:"group_name"`
	RateMultiplier     float64  `json:"rate_multiplier"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakStart          string   `json:"peak_start"`
	PeakEnd            string   `json:"peak_end"`
	PeakRateMultiplier float64  `json:"peak_rate_multiplier"`
	DailyLimitUSD      *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD     *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD    *float64 `json:"monthly_limit_usd"`
	ModelScopes        []string `json:"supported_model_scopes"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Price              float64  `json:"price"`
	OriginalPrice      *float64 `json:"original_price,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	ValidityDays       int      `json:"validity_days"`
	ValidityUnit       string   `json:"validity_unit"`
	Features           []string `json:"features"`
	ProductName        string   `json:"product_name"`
	CoverImageURL      string   `json:"cover_image_url"`
	DetailDescription  string   `json:"detail_description"`
	StorefrontPlatform string   `json:"storefront_platform"`
	StorefrontCategory string   `json:"storefront_category"`
	StorefrontFeatured bool     `json:"storefront_featured"`
	StorefrontBadge    string   `json:"storefront_badge"`
}

// parseFeatures splits a newline-separated features string into a string slice.
func parseFeatures(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

// GetLimits returns per-payment-type limits derived from enabled provider instances.
// GET /api/v1/payment/limits
func (h *PaymentHandler) GetLimits(c *gin.Context) {
	resp, err := h.configService.GetAvailableMethodLimits(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, resp)
}

// CreateOrderRequest is the request body for creating a payment order.
type CreateOrderRequest struct {
	Amount            float64 `json:"amount"`
	CouponID          int64   `json:"coupon_id,omitempty"`
	PaymentType       string  `json:"payment_type" binding:"required"`
	OpenID            string  `json:"openid"`
	WechatResumeToken string  `json:"wechat_resume_token"`
	ReturnURL         string  `json:"return_url"`
	PaymentSource     string  `json:"payment_source"`
	OrderType         string  `json:"order_type"`
	PlanID            int64   `json:"plan_id"`
	// IsMobile lets the frontend declare its mobile status directly. When
	// nil we fall back to User-Agent heuristics (which miss iPadOS / some
	// embedded browsers that strip the "Mobile" keyword).
	IsMobile *bool `json:"is_mobile,omitempty"`
}

// QuoteCouponPaymentRequest previews a selected coupon before an order is
// created. The authenticated user is supplied by the handler, never by JSON.
type QuoteCouponPaymentRequest struct {
	CouponID    int64   `json:"coupon_id" binding:"required"`
	Amount      float64 `json:"amount"`
	PaymentType string  `json:"payment_type" binding:"required"`
	OrderType   string  `json:"order_type"`
	PlanID      int64   `json:"plan_id"`
}

// QuoteCouponPayment returns the same settlement calculation CreateOrder will
// use. It does not reserve the coupon; reservation happens atomically with the
// order so an abandoned quote never makes a coupon unavailable.
// POST /api/v1/payment/coupons/quote
func (h *PaymentHandler) QuoteCouponPayment(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var req QuoteCouponPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	quote, err := h.paymentService.QuoteCouponPayment(c.Request.Context(), service.CouponPaymentQuoteRequest{
		UserID:      subject.UserID,
		CouponID:    req.CouponID,
		Amount:      req.Amount,
		PaymentType: req.PaymentType,
		OrderType:   req.OrderType,
		PlanID:      req.PlanID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}

// CreateOrder creates a new payment order.
// POST /api/v1/payment/orders
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.WechatResumeToken) != "" {
		claims, err := h.paymentService.ParseWeChatPaymentResumeToken(req.WechatResumeToken)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if err := applyWeChatPaymentResumeClaims(&req, claims); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	mobile := isMobile(c)
	if req.IsMobile != nil {
		mobile = *req.IsMobile
	}
	result, err := h.paymentService.CreateOrder(c.Request.Context(), service.CreateOrderRequest{
		UserID:          subject.UserID,
		Amount:          req.Amount,
		CouponID:        req.CouponID,
		PaymentType:     req.PaymentType,
		OpenID:          req.OpenID,
		ClientIP:        c.ClientIP(),
		IsMobile:        mobile,
		IsWeChatBrowser: isWeChatBrowser(c),
		SrcHost:         c.Request.Host,
		SrcURL:          c.Request.Referer(),
		ReturnURL:       req.ReturnURL,
		PaymentSource:   req.PaymentSource,
		OrderType:       req.OrderType,
		PlanID:          req.PlanID,
		Locale:          c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type MobilePaymentResult struct {
	Order           any                    `json:"order"`
	Launch          *service.PaymentLaunch `json:"launch,omitempty"`
	Deeplink        string                 `json:"deeplink,omitempty"`
	SchemeURL       string                 `json:"scheme_url,omitempty"`
	MWebURL         string                 `json:"mweb_url,omitempty"`
	H5URL           string                 `json:"h5_url,omitempty"`
	PayURL          string                 `json:"pay_url,omitempty"`
	QRCode          string                 `json:"qr_code,omitempty"`
	ResultType      string                 `json:"result_type,omitempty"`
	ReturnURL       string                 `json:"return_url,omitempty"`
	ResumeToken     string                 `json:"resume_token,omitempty"`
	VerifyAfterMS   int                    `json:"verify_after_ms,omitempty"`
	Paid            bool                   `json:"paid"`
	Completed       bool                   `json:"completed"`
	CanRetryPayment bool                   `json:"can_retry_payment"`
}

func (h *PaymentHandler) MobileCreate(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "支付参数不正确："+err.Error())
		return
	}
	if strings.TrimSpace(req.WechatResumeToken) != "" {
		claims, err := h.paymentService.ParseWeChatPaymentResumeToken(req.WechatResumeToken)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if err := applyWeChatPaymentResumeClaims(&req, claims); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	result, err := h.paymentService.CreateOrder(c.Request.Context(), service.CreateOrderRequest{
		UserID: subject.UserID, Amount: req.Amount, PaymentType: req.PaymentType, OpenID: req.OpenID,
		ClientIP: c.ClientIP(), IsMobile: true, IsWeChatBrowser: isWeChatBrowser(c),
		SrcHost: c.Request.Host, SrcURL: c.Request.Referer(), ReturnURL: req.ReturnURL,
		PaymentSource: firstNonEmptyPaymentSource(req.PaymentSource, "android_app"),
		OrderType:     req.OrderType, PlanID: req.PlanID, Locale: c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildMobilePaymentCreateResult(result, req.OrderType))
}

func (h *PaymentHandler) MobileGet(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	orderID, ok := parseMobilePaymentOrderID(c)
	if !ok {
		return
	}
	order, err := h.paymentService.GetOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildMobilePaymentOrderResult(order))
}

func (h *PaymentHandler) MobileSync(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	orderID, ok := parseMobilePaymentOrderID(c)
	if !ok {
		return
	}
	order, err := h.paymentService.GetOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if strings.TrimSpace(order.OutTradeNo) != "" {
		order, err = h.paymentService.VerifyOrderByOutTradeNo(c.Request.Context(), order.OutTradeNo, subject.UserID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	response.Success(c, buildMobilePaymentOrderResult(order))
}

func parseMobilePaymentOrderID(c *gin.Context) (int64, bool) {
	orderID, err := strconv.ParseInt(strings.TrimSpace(c.Param("order_id")), 10, 64)
	if err != nil || orderID <= 0 {
		response.BadRequest(c, "订单号不正确")
		return 0, false
	}
	return orderID, true
}

func buildMobilePaymentCreateResult(result *service.CreateOrderResponse, orderType string) MobilePaymentResult {
	if result == nil {
		return MobilePaymentResult{}
	}
	payURL := strings.TrimSpace(result.PayURL)
	schemeURL := mobilePaymentSchemeURL(payURL)
	return MobilePaymentResult{
		Order: sanitizeMobilePaymentCreateOrder(result, orderType), Launch: result.Launch,
		Deeplink: firstNonEmptyPaymentSource(schemeURL, payURL), SchemeURL: schemeURL,
		MWebURL: mobilePaymentHTTPURL(payURL), H5URL: mobilePaymentHTTPURL(payURL), PayURL: payURL,
		QRCode: strings.TrimSpace(result.QRCode), ResultType: string(result.ResultType),
		ReturnURL: strings.TrimSpace(result.ReturnURL), ResumeToken: strings.TrimSpace(result.ResumeToken),
		VerifyAfterMS: result.VerifyAfterMS, Paid: mobilePaymentStatusPaid(result.Status),
		Completed: result.Status == service.OrderStatusCompleted, CanRetryPayment: result.Status == service.OrderStatusPending,
	}
}

func buildMobilePaymentOrderResult(order *dbent.PaymentOrder) MobilePaymentResult {
	item := sanitizePaymentOrderForResponse(order)
	status := ""
	if item != nil {
		status = item.Status
	}
	return MobilePaymentResult{
		Order: item, Paid: mobilePaymentStatusPaid(status),
		Completed:       status == service.OrderStatusCompleted,
		CanRetryPayment: status == service.OrderStatusPending,
	}
}

func sanitizeMobilePaymentCreateOrder(result *service.CreateOrderResponse, orderType string) gin.H {
	return gin.H{
		"id": result.OrderID, "order_id": result.OrderID, "amount": result.Amount,
		"pay_amount": result.PayAmount, "fee_rate": result.FeeRate, "status": result.Status,
		"payment_type": result.PaymentType, "out_trade_no": result.OutTradeNo,
		"currency": result.Currency, "country_code": result.CountryCode,
		"payment_env": result.PaymentEnv, "payment_mode": result.PaymentMode,
		"order_type": strings.TrimSpace(orderType), "expires_at": result.ExpiresAt,
		"recharge_snapshot": result.RechargeSnapshot,
	}
}

func mobilePaymentHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return value
	}
	return ""
}

func mobilePaymentSchemeURL(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value != "" && !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return value
	}
	return ""
}

func mobilePaymentStatusPaid(status string) bool {
	switch status {
	case service.OrderStatusPaid, service.OrderStatusRecharging, service.OrderStatusCompleted,
		service.OrderStatusRefundRequested, service.OrderStatusRefunding, service.OrderStatusRefundPending,
		service.OrderStatusPartiallyRefunded, service.OrderStatusRefunded, service.OrderStatusRefundFailed:
		return true
	default:
		return false
	}
}

func firstNonEmptyPaymentSource(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func applyWeChatPaymentResumeClaims(req *CreateOrderRequest, claims *service.WeChatPaymentResumeClaims) error {
	if req == nil || claims == nil {
		return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume context is missing")
	}
	openid := strings.TrimSpace(claims.OpenID)
	if openid == "" {
		return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume token missing openid")
	}

	paymentType := service.NormalizeVisibleMethod(claims.PaymentType)
	if paymentType == "" {
		paymentType = payment.TypeWxpay
	}
	if req.PaymentType != "" {
		requestPaymentType := service.NormalizeVisibleMethod(req.PaymentType)
		if requestPaymentType != "" && requestPaymentType != paymentType {
			return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume token payment type mismatch")
		}
	}
	req.PaymentType = paymentType
	req.OpenID = openid

	if strings.TrimSpace(claims.Amount) != "" {
		amount, err := strconv.ParseFloat(strings.TrimSpace(claims.Amount), 64)
		if err != nil || amount <= 0 {
			return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", fmt.Sprintf("invalid resume amount: %s", claims.Amount))
		}
		req.Amount = amount
	}
	if claims.OrderType != "" {
		req.OrderType = claims.OrderType
	}
	if claims.PlanID > 0 {
		req.PlanID = claims.PlanID
	}
	if claims.CouponID > 0 {
		if req.CouponID > 0 && req.CouponID != claims.CouponID {
			return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume token coupon mismatch")
		}
		req.CouponID = claims.CouponID
	}
	return nil
}

// GetMyOrders returns the authenticated user's orders.
// GET /api/v1/payment/orders/my
func (h *PaymentHandler) GetMyOrders(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	page, pageSize := response.ParsePagination(c)
	orders, total, err := h.paymentService.GetUserOrders(c.Request.Context(), subject.UserID, service.OrderListParams{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		OrderType:   c.Query("order_type"),
		PaymentType: c.Query("payment_type"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, sanitizePaymentOrdersForResponse(orders), int64(total), page, pageSize)
}

// GetOrder returns a single order for the authenticated user.
// GET /api/v1/payment/orders/:id
func (h *PaymentHandler) GetOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	order, err := h.paymentService.GetOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, sanitizePaymentOrderForResponse(order))
}

// CancelOrder cancels a pending order for the authenticated user.
// POST /api/v1/payment/orders/:id/cancel
func (h *PaymentHandler) CancelOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	msg, err := h.paymentService.CancelOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": msg})
}

// RefundRequestBody is the request body for requesting a refund.
type RefundRequestBody struct {
	Reason string `json:"reason"`
}

// RequestRefund submits a refund request for a completed order.
// POST /api/v1/payment/orders/:id/refund-request
func (h *PaymentHandler) RequestRefund(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	var req RefundRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.paymentService.RequestRefund(c.Request.Context(), orderID, subject.UserID, req.Reason); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "refund requested"})
}

// GetRefundEligibleProviders returns provider instance IDs that allow user refund.
func (h *PaymentHandler) GetRefundEligibleProviders(c *gin.Context) {
	ids, err := h.configService.GetUserRefundEligibleInstanceIDs(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"provider_instance_ids": ids})
}

// VerifyOrderRequest is the request body for verifying a payment order.
type VerifyOrderRequest struct {
	OutTradeNo string `json:"out_trade_no" binding:"required"`
}

type ResolveOrderByResumeTokenRequest struct {
	ResumeToken string `json:"resume_token" binding:"required"`
}

// VerifyOrder actively queries the upstream payment provider to check
// if payment was made, and processes it if so.
// POST /api/v1/payment/orders/verify
func (h *PaymentHandler) VerifyOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	var req VerifyOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, err := h.paymentService.VerifyOrderByOutTradeNo(c.Request.Context(), req.OutTradeNo, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, sanitizePaymentOrderForResponse(order))
}

// PublicOrderResult is returned after a signed resume-token lookup. The token
// proves possession of the checkout session, so the result keeps the legacy
// frontend contract needed by payment result pages.
type PublicOrderResult struct {
	ID                       int64          `json:"id"`
	OutTradeNo               string         `json:"out_trade_no"`
	Amount                   float64        `json:"amount"`
	ListAmount               float64        `json:"list_amount"`
	GatewayBaseAmount        float64        `json:"gateway_base_amount"`
	DiscountAmount           float64        `json:"discount_amount"`
	FeeAmount                float64        `json:"fee_amount"`
	QualifyingRechargeAmount float64        `json:"qualifying_recharge_amount"`
	PayAmount                float64        `json:"pay_amount"`
	FeeRate                  float64        `json:"fee_rate"`
	Currency                 string         `json:"currency"`
	PaymentCurrency          string         `json:"payment_currency"`
	CouponID                 *int64         `json:"coupon_id,omitempty"`
	CouponTemplateID         *int64         `json:"coupon_template_id,omitempty"`
	PaymentType              string         `json:"payment_type"`
	OrderType                string         `json:"order_type"`
	Status                   string         `json:"status"`
	CreatedAt                time.Time      `json:"created_at"`
	ExpiresAt                time.Time      `json:"expires_at"`
	PaidAt                   *time.Time     `json:"paid_at,omitempty"`
	CompletedAt              *time.Time     `json:"completed_at,omitempty"`
	FailedAt                 *time.Time     `json:"failed_at,omitempty"`
	FailedReason             *string        `json:"failed_reason,omitempty"`
	RefundAmount             float64        `json:"refund_amount"`
	RefundReason             *string        `json:"refund_reason,omitempty"`
	RefundRequestedAt        *time.Time     `json:"refund_requested_at,omitempty"`
	RefundRequestedBy        *string        `json:"refund_requested_by,omitempty"`
	RefundRequestReason      *string        `json:"refund_request_reason,omitempty"`
	PlanID                   *int64         `json:"plan_id,omitempty"`
	RechargeSnapshot         map[string]any `json:"recharge_snapshot,omitempty"`
}

// PublicOrderVerifyResult is returned by the legacy anonymous out_trade_no
// lookup. Keep this intentionally minimal because out_trade_no is not secret.
type PublicOrderVerifyResult struct {
	OutTradeNo  string     `json:"out_trade_no"`
	Status      string     `json:"status"`
	Paid        bool       `json:"paid"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func buildPublicOrderResult(order *dbent.PaymentOrder) PublicOrderResult {
	return PublicOrderResult{
		ID:                       order.ID,
		OutTradeNo:               order.OutTradeNo,
		Amount:                   order.Amount,
		ListAmount:               order.ListAmount,
		GatewayBaseAmount:        order.GatewayBaseAmount,
		DiscountAmount:           order.DiscountAmount,
		FeeAmount:                order.FeeAmount,
		QualifyingRechargeAmount: order.QualifyingRechargeAmount,
		PayAmount:                order.PayAmount,
		FeeRate:                  order.FeeRate,
		Currency:                 service.PaymentOrderCurrency(order),
		PaymentCurrency:          order.PaymentCurrency,
		CouponID:                 order.CouponID,
		CouponTemplateID:         order.CouponTemplateID,
		PaymentType:              order.PaymentType,
		OrderType:                order.OrderType,
		Status:                   order.Status,
		CreatedAt:                order.CreatedAt,
		ExpiresAt:                order.ExpiresAt,
		PaidAt:                   order.PaidAt,
		CompletedAt:              order.CompletedAt,
		FailedAt:                 order.FailedAt,
		FailedReason:             order.FailedReason,
		RefundAmount:             order.RefundAmount,
		RefundReason:             order.RefundReason,
		RefundRequestedAt:        order.RefundRequestedAt,
		RefundRequestedBy:        order.RefundRequestedBy,
		RefundRequestReason:      order.RefundRequestReason,
		PlanID:                   order.PlanID,
		RechargeSnapshot:         servicePaymentRechargeSnapshotForResponse(order),
	}
}

func buildPublicOrderVerifyResult(order *dbent.PaymentOrder) PublicOrderVerifyResult {
	return PublicOrderVerifyResult{
		OutTradeNo:  order.OutTradeNo,
		Status:      order.Status,
		Paid:        publicOrderStatusPaid(order.Status),
		CreatedAt:   order.CreatedAt,
		ExpiresAt:   order.ExpiresAt,
		PaidAt:      order.PaidAt,
		CompletedAt: order.CompletedAt,
	}
}

func publicOrderStatusPaid(status string) bool {
	switch status {
	case service.OrderStatusPaid,
		service.OrderStatusCompleted,
		service.OrderStatusRefundRequested,
		service.OrderStatusRefunding,
		service.OrderStatusRefundPending,
		service.OrderStatusPartiallyRefunded,
		service.OrderStatusRefunded,
		service.OrderStatusRefundFailed:
		return true
	default:
		return false
	}
}

// VerifyOrderPublic keeps the legacy anonymous out_trade_no lookup available as
// a compatibility path for older result pages and staggered deploys.
// POST /api/v1/payment/public/orders/verify
func (h *PaymentHandler) VerifyOrderPublic(c *gin.Context) {
	var req VerifyOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, err := h.paymentService.VerifyOrderPublic(c.Request.Context(), req.OutTradeNo)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildPublicOrderVerifyResult(order))
}

// ResolveOrderPublicByResumeToken resolves a payment order from a signed resume token.
// POST /api/v1/payment/public/orders/resolve
func (h *PaymentHandler) ResolveOrderPublicByResumeToken(c *gin.Context) {
	var req ResolveOrderByResumeTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, err := h.paymentService.GetPublicOrderByResumeToken(c.Request.Context(), req.ResumeToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildPublicOrderResult(order))
}

// requireAuth extracts the authenticated subject from the context.
// Returns the subject and true on success; on failure it writes an Unauthorized response and returns false.
func requireAuth(c *gin.Context) (middleware2.AuthSubject, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return middleware2.AuthSubject{}, false
	}
	return subject, true
}

// isMobile detects mobile user agents.
func isMobile(c *gin.Context) bool {
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	for _, kw := range []string{"mobile", "android", "iphone", "ipad", "ipod"} {
		if strings.Contains(ua, kw) {
			return true
		}
	}
	return false
}

type PaymentOrderResult struct {
	ID                       int64          `json:"id"`
	UserID                   int64          `json:"user_id"`
	Amount                   float64        `json:"amount"`
	ListAmount               float64        `json:"list_amount"`
	GatewayBaseAmount        float64        `json:"gateway_base_amount"`
	DiscountAmount           float64        `json:"discount_amount"`
	FeeAmount                float64        `json:"fee_amount"`
	QualifyingRechargeAmount float64        `json:"qualifying_recharge_amount"`
	PayAmount                float64        `json:"pay_amount"`
	FeeRate                  float64        `json:"fee_rate"`
	Currency                 string         `json:"currency"`
	PaymentCurrency          string         `json:"payment_currency"`
	CouponID                 *int64         `json:"coupon_id,omitempty"`
	CouponTemplateID         *int64         `json:"coupon_template_id,omitempty"`
	PaymentType              string         `json:"payment_type"`
	OutTradeNo               string         `json:"out_trade_no"`
	Status                   string         `json:"status"`
	OrderType                string         `json:"order_type"`
	CreatedAt                time.Time      `json:"created_at"`
	ExpiresAt                time.Time      `json:"expires_at"`
	PaidAt                   *time.Time     `json:"paid_at,omitempty"`
	CompletedAt              *time.Time     `json:"completed_at,omitempty"`
	FailedAt                 *time.Time     `json:"failed_at,omitempty"`
	FailedReason             *string        `json:"failed_reason,omitempty"`
	RefundAmount             float64        `json:"refund_amount"`
	RefundReason             *string        `json:"refund_reason,omitempty"`
	RefundRequestedAt        *time.Time     `json:"refund_requested_at,omitempty"`
	RefundRequestedBy        *string        `json:"refund_requested_by,omitempty"`
	RefundRequestReason      *string        `json:"refund_request_reason,omitempty"`
	PlanID                   *int64         `json:"plan_id,omitempty"`
	ProviderInstanceID       *string        `json:"provider_instance_id,omitempty"`
	RechargeSnapshot         map[string]any `json:"recharge_snapshot,omitempty"`
}

func sanitizePaymentOrdersForResponse(orders []*dbent.PaymentOrder) []PaymentOrderResult {
	out := make([]PaymentOrderResult, 0, len(orders))
	for _, order := range orders {
		if item := sanitizePaymentOrderForResponse(order); item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func sanitizePaymentOrderForResponse(order *dbent.PaymentOrder) *PaymentOrderResult {
	if order == nil {
		return nil
	}
	return &PaymentOrderResult{
		ID:                       order.ID,
		UserID:                   order.UserID,
		Amount:                   order.Amount,
		ListAmount:               order.ListAmount,
		GatewayBaseAmount:        order.GatewayBaseAmount,
		DiscountAmount:           order.DiscountAmount,
		FeeAmount:                order.FeeAmount,
		QualifyingRechargeAmount: order.QualifyingRechargeAmount,
		PayAmount:                order.PayAmount,
		FeeRate:                  order.FeeRate,
		Currency:                 service.PaymentOrderCurrency(order),
		PaymentCurrency:          order.PaymentCurrency,
		CouponID:                 order.CouponID,
		CouponTemplateID:         order.CouponTemplateID,
		PaymentType:              order.PaymentType,
		OutTradeNo:               order.OutTradeNo,
		Status:                   order.Status,
		OrderType:                order.OrderType,
		CreatedAt:                order.CreatedAt,
		ExpiresAt:                order.ExpiresAt,
		PaidAt:                   order.PaidAt,
		CompletedAt:              order.CompletedAt,
		FailedAt:                 order.FailedAt,
		FailedReason:             order.FailedReason,
		RefundAmount:             order.RefundAmount,
		RefundReason:             order.RefundReason,
		RefundRequestedAt:        order.RefundRequestedAt,
		RefundRequestedBy:        order.RefundRequestedBy,
		RefundRequestReason:      order.RefundRequestReason,
		PlanID:                   order.PlanID,
		ProviderInstanceID:       order.ProviderInstanceID,
		RechargeSnapshot:         servicePaymentRechargeSnapshotForResponse(order),
	}
}

func servicePaymentRechargeSnapshotForResponse(order *dbent.PaymentOrder) map[string]any {
	if order == nil || len(order.RechargeSnapshot) == 0 {
		return nil
	}
	return order.RechargeSnapshot
}

func isWeChatBrowser(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "micromessenger")
}
