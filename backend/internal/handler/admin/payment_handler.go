package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles admin payment management.
type PaymentHandler struct {
	paymentService *service.PaymentService
	configService  *service.PaymentConfigService
}

// NewPaymentHandler creates a new admin PaymentHandler.
func NewPaymentHandler(paymentService *service.PaymentService, configService *service.PaymentConfigService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		configService:  configService,
	}
}

// --- Dashboard ---

// GetDashboard returns payment dashboard statistics.
// GET /api/v1/admin/payment/dashboard
func (h *PaymentHandler) GetDashboard(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	stats, err := h.paymentService.GetDashboardStats(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// GetPlayBillingConfig returns the Google Play Billing product mapping.
// GET /api/v1/admin/payment/play-billing/config
func (h *PaymentHandler) GetPlayBillingConfig(c *gin.Context) {
	if h == nil || h.configService == nil {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "Play Billing 配置服务未初始化", "PLAY_BILLING_NOT_CONFIGURED", nil)
		return
	}
	cfg, err := h.configService.GetMobilePlayBillingAdminConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdatePlayBillingConfig updates the backend-owned mapping between Google Play
// product IDs and platform balance/subscription fulfillment rules.
// PUT /api/v1/admin/payment/play-billing/config
func (h *PaymentHandler) UpdatePlayBillingConfig(c *gin.Context) {
	if h == nil || h.configService == nil {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "Play Billing 配置服务未初始化", "PLAY_BILLING_NOT_CONFIGURED", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
	var req service.UpdateMobilePlayBillingProductsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Play Billing 商品映射格式不正确")
		return
	}
	cfg, err := h.configService.UpdateMobilePlayBillingProducts(c.Request.Context(), req)
	if err != nil {
		playBillingConfigError(c, err)
		return
	}
	response.Success(c, cfg)
}

func playBillingConfigError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrMobilePlayBillingNotConfigured) {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "Play Billing 配置服务未初始化", "PLAY_BILLING_NOT_CONFIGURED", nil)
		return
	}
	response.ErrorFrom(c, err)
}

// --- Orders ---

// ListOrders returns a paginated list of all payment orders.
// GET /api/v1/admin/payment/orders
func (h *PaymentHandler) ListOrders(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if uid := c.Query("user_id"); uid != "" {
		if v, err := strconv.ParseInt(uid, 10, 64); err == nil {
			userID = v
		}
	}
	orders, total, err := h.paymentService.AdminListOrders(c.Request.Context(), userID, service.OrderListParams{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		OrderType:   c.Query("order_type"),
		PaymentType: c.Query("payment_type"),
		Keyword:     c.Query("keyword"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, sanitizeAdminPaymentOrdersForResponse(orders), int64(total), page, pageSize)
}

// GetOrderDetail returns detailed information about a single order.
// GET /api/v1/admin/payment/orders/:id
func (h *PaymentHandler) GetOrderDetail(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	order, err := h.paymentService.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	auditLogs, _ := h.paymentService.GetOrderAuditLogs(c.Request.Context(), orderID)
	response.Success(c, gin.H{"order": sanitizeAdminPaymentOrderForResponse(order), "auditLogs": auditLogs})
}

// CancelOrder cancels a pending order (admin).
// POST /api/v1/admin/payment/orders/:id/cancel
func (h *PaymentHandler) CancelOrder(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	msg, err := h.paymentService.AdminCancelOrder(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": msg})
}

// RetryFulfillment retries fulfillment for a paid order.
// POST /api/v1/admin/payment/orders/:id/retry
func (h *PaymentHandler) RetryFulfillment(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.paymentService.RetryFulfillment(c.Request.Context(), orderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "fulfillment retried"})
}

type AdminPaymentOrderResult struct {
	ID                       int64          `json:"id"`
	UserID                   int64          `json:"user_id"`
	UserEmail                string         `json:"user_email,omitempty"`
	UserName                 string         `json:"user_name,omitempty"`
	UserNotes                *string        `json:"user_notes,omitempty"`
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
	RechargeCode             string         `json:"recharge_code,omitempty"`
	OutTradeNo               string         `json:"out_trade_no"`
	PaymentType              string         `json:"payment_type"`
	PaymentTradeNo           string         `json:"payment_trade_no,omitempty"`
	PayURL                   *string        `json:"pay_url,omitempty"`
	QRCode                   *string        `json:"qr_code,omitempty"`
	QRCodeImg                *string        `json:"qr_code_img,omitempty"`
	OrderType                string         `json:"order_type"`
	PlanID                   *int64         `json:"plan_id,omitempty"`
	SubscriptionGroupID      *int64         `json:"subscription_group_id,omitempty"`
	SubscriptionDays         *int           `json:"subscription_days,omitempty"`
	ProviderInstanceID       *string        `json:"provider_instance_id,omitempty"`
	ProviderKey              *string        `json:"provider_key,omitempty"`
	Status                   string         `json:"status"`
	RefundAmount             float64        `json:"refund_amount"`
	RefundReason             *string        `json:"refund_reason,omitempty"`
	RefundAt                 *time.Time     `json:"refund_at,omitempty"`
	ForceRefund              bool           `json:"force_refund,omitempty"`
	RefundRequestedAt        *time.Time     `json:"refund_requested_at,omitempty"`
	RefundRequestReason      *string        `json:"refund_request_reason,omitempty"`
	RefundRequestedBy        *string        `json:"refund_requested_by,omitempty"`
	ExpiresAt                time.Time      `json:"expires_at"`
	PaidAt                   *time.Time     `json:"paid_at,omitempty"`
	CompletedAt              *time.Time     `json:"completed_at,omitempty"`
	FailedAt                 *time.Time     `json:"failed_at,omitempty"`
	FailedReason             *string        `json:"failed_reason,omitempty"`
	ClientIP                 string         `json:"client_ip,omitempty"`
	SrcHost                  string         `json:"src_host,omitempty"`
	SrcURL                   *string        `json:"src_url,omitempty"`
	RechargeSnapshot         map[string]any `json:"recharge_snapshot,omitempty"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
}

func sanitizeAdminPaymentOrdersForResponse(orders []*dbent.PaymentOrder) []*AdminPaymentOrderResult {
	out := make([]*AdminPaymentOrderResult, 0, len(orders))
	for _, order := range orders {
		if item := sanitizeAdminPaymentOrderForResponse(order); item != nil {
			out = append(out, item)
		}
	}
	return out
}

func sanitizeAdminPaymentOrderForResponse(order *dbent.PaymentOrder) *AdminPaymentOrderResult {
	if order == nil {
		return nil
	}
	return &AdminPaymentOrderResult{
		ID:                       order.ID,
		UserID:                   order.UserID,
		UserEmail:                order.UserEmail,
		UserName:                 order.UserName,
		UserNotes:                order.UserNotes,
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
		RechargeCode:             order.RechargeCode,
		OutTradeNo:               order.OutTradeNo,
		PaymentType:              order.PaymentType,
		PaymentTradeNo:           order.PaymentTradeNo,
		PayURL:                   order.PayURL,
		QRCode:                   order.QrCode,
		QRCodeImg:                order.QrCodeImg,
		OrderType:                order.OrderType,
		PlanID:                   order.PlanID,
		SubscriptionGroupID:      order.SubscriptionGroupID,
		SubscriptionDays:         order.SubscriptionDays,
		ProviderInstanceID:       order.ProviderInstanceID,
		ProviderKey:              order.ProviderKey,
		Status:                   order.Status,
		RefundAmount:             order.RefundAmount,
		RefundReason:             order.RefundReason,
		RefundAt:                 order.RefundAt,
		ForceRefund:              order.ForceRefund,
		RefundRequestedAt:        order.RefundRequestedAt,
		RefundRequestReason:      order.RefundRequestReason,
		RefundRequestedBy:        order.RefundRequestedBy,
		ExpiresAt:                order.ExpiresAt,
		PaidAt:                   order.PaidAt,
		CompletedAt:              order.CompletedAt,
		FailedAt:                 order.FailedAt,
		FailedReason:             order.FailedReason,
		ClientIP:                 order.ClientIP,
		SrcHost:                  order.SrcHost,
		SrcURL:                   order.SrcURL,
		RechargeSnapshot:         adminPaymentRechargeSnapshotForResponse(order),
		CreatedAt:                order.CreatedAt,
		UpdatedAt:                order.UpdatedAt,
	}
}

func adminPaymentRechargeSnapshotForResponse(order *dbent.PaymentOrder) map[string]any {
	if order == nil || len(order.RechargeSnapshot) == 0 {
		return nil
	}
	return order.RechargeSnapshot
}

// AdminProcessRefundRequest is the request body for admin refund processing.
type AdminProcessRefundRequest struct {
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
	Force         bool    `json:"force"`
	DeductBalance bool    `json:"deduct_balance"`
}

// ProcessRefund processes a refund for an order (admin).
// POST /api/v1/admin/payment/orders/:id/refund
func (h *PaymentHandler) ProcessRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req AdminProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plan, earlyResult, err := h.paymentService.PrepareRefund(c.Request.Context(), orderID, req.Amount, req.Reason, req.Force, req.DeductBalance)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if earlyResult != nil {
		response.Success(c, earlyResult)
		return
	}

	result, err := h.paymentService.ExecuteRefund(c.Request.Context(), plan)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// QueryAndFinalizeRefund queries the provider refund status and finalizes a pending refund.
// POST /api/v1/admin/payment/orders/:id/refund/query
func (h *PaymentHandler) QueryAndFinalizeRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	result, err := h.paymentService.QueryAndFinalizeRefund(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// --- Subscription Plans ---

// ListPlans returns all subscription plans.
// GET /api/v1/admin/payment/plans
func (h *PaymentHandler) ListPlans(c *gin.Context) {
	plans, err := h.configService.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	groupInfo := h.configService.GetGroupInfoMap(c.Request.Context(), plans)
	response.Success(c, adminSubscriptionPlansForResponse(plans, groupInfo))
}

type AdminSubscriptionPlanResult struct {
	ID              int64     `json:"id"`
	GroupID         int64     `json:"group_id"`
	GroupPlatform   string    `json:"group_platform,omitempty"`
	GroupName       string    `json:"group_name,omitempty"`
	RateMultiplier  float64   `json:"rate_multiplier,omitempty"`
	DailyLimitUSD   *float64  `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD  *float64  `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD *float64  `json:"monthly_limit_usd,omitempty"`
	ModelScopes     []string  `json:"supported_model_scopes,omitempty"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Price           float64   `json:"price"`
	OriginalPrice   *float64  `json:"original_price,omitempty"`
	Currency        string    `json:"currency,omitempty"`
	ValidityDays    int       `json:"validity_days"`
	ValidityUnit    string    `json:"validity_unit"`
	RequestLimit    *int64    `json:"request_limit,omitempty"`
	AmountLimitUSD  *float64  `json:"amount_limit_usd,omitempty"`
	TokenLimit      *int64    `json:"token_limit,omitempty"`
	Features        string    `json:"features"`
	ProductName     string    `json:"product_name"`
	ForSale         bool      `json:"for_sale"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

func adminSubscriptionPlansForResponse(plans []*dbent.SubscriptionPlan, groupInfo map[int64]service.PlanGroupInfo) []AdminSubscriptionPlanResult {
	result := make([]AdminSubscriptionPlanResult, 0, len(plans))
	for _, p := range plans {
		if p == nil {
			continue
		}
		gi := groupInfo[p.GroupID]
		result = append(result, AdminSubscriptionPlanResult{
			ID:              int64(p.ID),
			GroupID:         p.GroupID,
			GroupPlatform:   gi.Platform,
			GroupName:       gi.Name,
			RateMultiplier:  gi.RateMultiplier,
			DailyLimitUSD:   gi.DailyLimitUSD,
			WeeklyLimitUSD:  gi.WeeklyLimitUSD,
			MonthlyLimitUSD: gi.MonthlyLimitUSD,
			ModelScopes:     gi.ModelScopes,
			Name:            p.Name,
			Description:     p.Description,
			Price:           p.Price,
			OriginalPrice:   p.OriginalPrice,
			Currency:        p.Currency,
			ValidityDays:    p.ValidityDays,
			ValidityUnit:    p.ValidityUnit,
			RequestLimit:    p.RequestLimit,
			AmountLimitUSD:  p.AmountLimitUsd,
			TokenLimit:      p.TokenLimit,
			Features:        p.Features,
			ProductName:     p.ProductName,
			ForSale:         p.ForSale,
			SortOrder:       p.SortOrder,
			CreatedAt:       p.CreatedAt,
			UpdatedAt:       p.UpdatedAt,
		})
	}
	return result
}

// CreatePlan creates a new subscription plan.
// POST /api/v1/admin/payment/plans
func (h *PaymentHandler) CreatePlan(c *gin.Context) {
	var req service.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.configService.CreatePlan(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, plan)
}

// UpdatePlan updates an existing subscription plan.
// PUT /api/v1/admin/payment/plans/:id
func (h *PaymentHandler) UpdatePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.configService.UpdatePlan(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

// DeletePlan deletes a subscription plan.
// DELETE /api/v1/admin/payment/plans/:id
func (h *PaymentHandler) DeletePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeletePlan(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

// GetStorefrontConfig returns the configurable user-facing plan shelves and tags.
// GET /api/v1/admin/payment/storefront
func (h *PaymentHandler) GetStorefrontConfig(c *gin.Context) {
	plans, err := h.configService.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	cfg, err := h.configService.GetPaymentStorefrontConfig(c.Request.Context(), plans)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateStorefrontConfig updates the configurable user-facing plan shelves and tags.
// PUT /api/v1/admin/payment/storefront
func (h *PaymentHandler) UpdateStorefrontConfig(c *gin.Context) {
	var req service.PaymentStorefrontConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.configService.UpdatePaymentStorefrontConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// GetAPIOnboardingConfig returns the configurable API Key onboarding cards.
// GET /api/v1/admin/payment/api-onboarding
func (h *PaymentHandler) GetAPIOnboardingConfig(c *gin.Context) {
	cfg, err := h.configService.GetAPIOnboardingConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateAPIOnboardingConfig updates API Key onboarding recommendation cards.
// PUT /api/v1/admin/payment/api-onboarding
func (h *PaymentHandler) UpdateAPIOnboardingConfig(c *gin.Context) {
	var req service.APIOnboardingConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.configService.UpdateAPIOnboardingConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// --- Provider Instances ---

// ListProviders returns all payment provider instances.
// GET /api/v1/admin/payment/providers
func (h *PaymentHandler) ListProviders(c *gin.Context) {
	providers, err := h.configService.ListProviderInstancesWithConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, providers)
}

// CreateProvider creates a new payment provider instance.
// POST /api/v1/admin/payment/providers
func (h *PaymentHandler) CreateProvider(c *gin.Context) {
	var req service.CreateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.CreateProviderInstance(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Created(c, inst)
}

// UpdateProvider updates an existing payment provider instance.
// PUT /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) UpdateProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.UpdateProviderInstance(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, inst)
}

// DeleteProvider deletes a payment provider instance.
// DELETE /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) DeleteProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeleteProviderInstance(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, gin.H{"message": "deleted"})
}

// parseIDParam parses an int64 path parameter.
// Returns the parsed ID and true on success; on failure it writes a BadRequest response and returns false.
func parseIDParam(c *gin.Context, paramName string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid "+paramName)
		return 0, false
	}
	return id, true
}

// --- Config ---

// GetConfig returns the payment configuration (admin view).
// GET /api/v1/admin/payment/config
func (h *PaymentHandler) GetConfig(c *gin.Context) {
	cfg, err := h.configService.GetPaymentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig updates the payment configuration.
// PUT /api/v1/admin/payment/config
func (h *PaymentHandler) UpdateConfig(c *gin.Context) {
	var req service.UpdatePaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.configService.UpdatePaymentConfig(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}
