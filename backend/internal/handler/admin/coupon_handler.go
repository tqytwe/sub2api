package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CouponHandler manages payment coupon templates, issued coupons, batches,
// and reward-pool versions under the existing promo-code admin entry.
type CouponHandler struct {
	service *service.CouponService
}

func NewCouponHandler(couponService *service.CouponService) *CouponHandler {
	return &CouponHandler{service: couponService}
}

type couponTemplateRequest struct {
	Key                string                       `json:"key" binding:"required,max=64"`
	Name               string                       `json:"name" binding:"required,max=120"`
	Description        string                       `json:"description" binding:"max=4000"`
	Status             service.CouponTemplateStatus `json:"status" binding:"omitempty,oneof=draft active paused archived"`
	BenefitType        service.CouponBenefitType    `json:"benefit_type" binding:"required,oneof=fixed_amount percentage"`
	BenefitValue       float64                      `json:"benefit_value" binding:"required,gt=0"`
	MaxDiscountAmount  *float64                     `json:"max_discount_amount" binding:"omitempty,gt=0"`
	Currency           string                       `json:"currency" binding:"required,len=3"`
	ApplicableScopes   []service.CouponScope        `json:"applicable_scopes" binding:"required,min=1,max=2"`
	MinimumOrderAmount float64                      `json:"minimum_order_amount" binding:"gte=0"`
	EligiblePlanIDs    []int64                      `json:"eligible_plan_ids" binding:"max=100"`
	ValidityMode       service.CouponValidityMode   `json:"validity_mode" binding:"required,oneof=relative_days end_of_day end_of_month fixed"`
	ValidityDays       int                          `json:"validity_days" binding:"gte=0,lte=3650"`
	FixedExpiresAt     *time.Time                   `json:"fixed_expires_at"`
	ValidFrom          *time.Time                   `json:"valid_from"`
	TotalIssueLimit    *int64                       `json:"total_issue_limit" binding:"omitempty,gt=0"`
	Rules              map[string]any               `json:"rules"`
}

func (r couponTemplateRequest) input() service.CouponTemplateInput {
	return service.CouponTemplateInput{
		Key:                r.Key,
		Name:               r.Name,
		Description:        r.Description,
		Status:             r.Status,
		BenefitType:        r.BenefitType,
		BenefitValue:       r.BenefitValue,
		MaxDiscountAmount:  r.MaxDiscountAmount,
		Currency:           r.Currency,
		ApplicableScopes:   r.ApplicableScopes,
		MinimumOrderAmount: r.MinimumOrderAmount,
		EligiblePlanIDs:    r.EligiblePlanIDs,
		ValidityMode:       r.ValidityMode,
		ValidityDays:       r.ValidityDays,
		FixedExpiresAt:     r.FixedExpiresAt,
		ValidFrom:          r.ValidFrom,
		TotalIssueLimit:    r.TotalIssueLimit,
		Rules:              r.Rules,
	}
}

// ListTemplates GET /admin/promo-codes/coupons/templates
func (h *CouponHandler) ListTemplates(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	rows, result, err := h.service.ListTemplates(c.Request.Context(), service.CouponTemplateListFilter{
		Status:   service.CouponTemplateStatus(c.Query("status")),
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

// GetTemplate GET /admin/promo-codes/coupons/templates/:id
func (h *CouponHandler) GetTemplate(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	template, err := h.service.GetTemplate(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, template)
}

// CreateTemplate POST /admin/promo-codes/coupons/templates
func (h *CouponHandler) CreateTemplate(c *gin.Context) {
	var request couponTemplateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon template: "+err.Error())
		return
	}
	template, err := h.service.CreateTemplate(c.Request.Context(), request.input(), adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, template)
}

// UpdateTemplate PUT /admin/promo-codes/coupons/templates/:id
func (h *CouponHandler) UpdateTemplate(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	var request couponTemplateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon template: "+err.Error())
		return
	}
	template, err := h.service.UpdateTemplate(c.Request.Context(), id, request.input(), adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, template)
}

// DeleteTemplate DELETE /admin/promo-codes/coupons/templates/:id
func (h *CouponHandler) DeleteTemplate(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ListUserCoupons GET /admin/promo-codes/coupons/user-coupons
func (h *CouponHandler) ListUserCoupons(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	issuedFrom := couponOptionalTime(c.Query("issued_from"))
	issuedTo := couponOptionalTime(c.Query("issued_to"))
	rows, result, err := h.service.ListUserCoupons(c.Request.Context(), service.UserCouponListFilter{
		UserID:     couponOptionalInt64(c.Query("user_id")),
		UserQuery:  strings.TrimSpace(c.Query("user")),
		TemplateID: couponOptionalInt64(c.Query("template_id")),
		Source:     service.CouponIssueSource(c.Query("source")),
		Status:     service.UserCouponStatus(c.Query("status")),
		IssuedFrom: issuedFrom,
		IssuedTo:   issuedTo,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

type couponIssueRequest struct {
	TemplateID     int64                     `json:"template_id" binding:"required,gt=0"`
	UserID         int64                     `json:"user_id" binding:"required,gt=0"`
	Source         service.CouponIssueSource `json:"source" binding:"omitempty,oneof=blindbox quiz admin_batch manual compensation"`
	SourceRef      string                    `json:"source_ref" binding:"max=256"`
	IdempotencyKey string                    `json:"idempotency_key" binding:"required,max=200"`
}

// IssueUserCoupon POST /admin/promo-codes/coupons/user-coupons/issue
func (h *CouponHandler) IssueUserCoupon(c *gin.Context) {
	var request couponIssueRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon issue: "+err.Error())
		return
	}
	if request.Source == "" {
		request.Source = service.CouponIssueSourceManual
	}
	coupon, err := h.service.IssueCoupon(c.Request.Context(), service.CouponIssueInput{
		TemplateID:     request.TemplateID,
		UserID:         request.UserID,
		Source:         request.Source,
		SourceRef:      request.SourceRef,
		IdempotencyKey: request.IdempotencyKey,
		ActorID:        adminActorID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, coupon)
}

type couponBatchIssueRequest struct {
	TemplateID     int64                     `json:"template_id" binding:"required,gt=0"`
	UserIDs        []int64                   `json:"user_ids" binding:"required,min=1,max=10000"`
	Source         service.CouponIssueSource `json:"source" binding:"omitempty,oneof=blindbox quiz admin_batch manual compensation"`
	IdempotencyKey string                    `json:"idempotency_key" binding:"required,max=200"`
	Metadata       map[string]any            `json:"metadata"`
}

// IssueUserCouponBatch POST /admin/promo-codes/coupons/user-coupons/batches
func (h *CouponHandler) IssueUserCouponBatch(c *gin.Context) {
	var request couponBatchIssueRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon issue batch: "+err.Error())
		return
	}
	if request.Source == "" {
		request.Source = service.CouponIssueSourceAdminBatch
	}
	batch, err := h.service.IssueCouponBatch(c.Request.Context(), service.CouponBatchIssueInput{
		TemplateID:     request.TemplateID,
		UserIDs:        request.UserIDs,
		Source:         request.Source,
		IdempotencyKey: request.IdempotencyKey,
		ActorID:        adminActorID(c),
		Metadata:       request.Metadata,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, batch)
}

// ListIssueBatches GET /admin/promo-codes/coupons/batches
func (h *CouponHandler) ListIssueBatches(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	rows, result, err := h.service.ListCouponIssueBatches(c.Request.Context(), service.CouponIssueBatchListFilter{
		TemplateID: couponOptionalInt64(c.Query("template_id")),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

type couponVoidRequest struct {
	Reason string `json:"reason" binding:"required,max=500"`
}

// VoidUserCoupon POST /admin/promo-codes/coupons/user-coupons/:id/void
func (h *CouponHandler) VoidUserCoupon(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	var request couponVoidRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon void request: "+err.Error())
		return
	}
	coupon, err := h.service.VoidUserCoupon(c.Request.Context(), id, request.Reason, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, coupon)
}

type couponRewardPoolRequest struct {
	Activity           service.CouponRewardActivity    `json:"activity" binding:"required,oneof=blindbox quiz checkin"`
	Version            string                          `json:"version" binding:"required,max=80"`
	CouponWeightBP     int                             `json:"coupon_weight_bp" binding:"required,gte=0,lte=10000"`
	RedeemCodeWeightBP int                             `json:"redeem_code_weight_bp" binding:"gte=0,lte=10000"`
	BalanceWeightBP    int                             `json:"balance_weight_bp" binding:"required,gte=0,lte=10000"`
	RewardConfig       service.CouponRewardPoolConfig  `json:"reward_config"`
	FallbackTemplateID int64                           `json:"fallback_template_id" binding:"required,gt=0"`
	Entries            []service.CouponRewardPoolEntry `json:"entries" binding:"required,min=1,max=100"`
}

func (r couponRewardPoolRequest) pool(id int64) service.CouponRewardPoolVersion {
	return service.CouponRewardPoolVersion{
		ID:                 id,
		Activity:           r.Activity,
		Version:            r.Version,
		Status:             service.CouponRewardPoolStatusDraft,
		CouponWeightBP:     r.CouponWeightBP,
		RedeemCodeWeightBP: r.RedeemCodeWeightBP,
		BalanceWeightBP:    r.BalanceWeightBP,
		RewardConfig:       r.RewardConfig,
		FallbackTemplateID: r.FallbackTemplateID,
		Entries:            r.Entries,
	}
}

// ListRewardPools GET /admin/promo-codes/coupons/pools
func (h *CouponHandler) ListRewardPools(c *gin.Context) {
	pools, err := h.service.ListRewardPools(c.Request.Context(), service.CouponRewardActivity(c.Query("activity")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pools)
}

// GetRewardPool GET /admin/promo-codes/coupons/pools/:id
func (h *CouponHandler) GetRewardPool(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	pool, err := h.service.GetRewardPool(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

// GetPublishedRewardPool GET /admin/promo-codes/coupons/pools/published?activity=blindbox|quiz|checkin
func (h *CouponHandler) GetPublishedRewardPool(c *gin.Context) {
	pool, err := h.service.GetPublishedRewardPool(c.Request.Context(), service.CouponRewardActivity(c.Query("activity")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

// CreateRewardPool POST /admin/promo-codes/coupons/pools
func (h *CouponHandler) CreateRewardPool(c *gin.Context) {
	var request couponRewardPoolRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon reward pool: "+err.Error())
		return
	}
	pool, err := h.service.SaveRewardPool(c.Request.Context(), request.pool(0), adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, pool)
}

// UpdateRewardPool PUT /admin/promo-codes/coupons/pools/:id
func (h *CouponHandler) UpdateRewardPool(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	var request couponRewardPoolRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "invalid coupon reward pool: "+err.Error())
		return
	}
	pool, err := h.service.SaveRewardPool(c.Request.Context(), request.pool(id), adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

// DeleteRewardPool DELETE /admin/promo-codes/coupons/pools/:id
// Only draft versions can be removed. Published and retired versions are
// retained as settlement audit history.
func (h *CouponHandler) DeleteRewardPool(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteRewardPool(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// PublishRewardPool POST /admin/promo-codes/coupons/pools/:id/publish
func (h *CouponHandler) PublishRewardPool(c *gin.Context) {
	id, ok := couponPathID(c)
	if !ok {
		return
	}
	pool, err := h.service.PublishRewardPool(c.Request.Context(), id, adminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

func couponPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return id, true
}

func couponOptionalInt64(raw string) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func couponOptionalTime(raw string) *time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return &parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed
	}
	return nil
}
