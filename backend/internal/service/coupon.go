package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

const couponWeightBasisPoints = 10_000

type CouponTemplateStatus string

const (
	CouponTemplateStatusDraft    CouponTemplateStatus = "draft"
	CouponTemplateStatusActive   CouponTemplateStatus = "active"
	CouponTemplateStatusPaused   CouponTemplateStatus = "paused"
	CouponTemplateStatusArchived CouponTemplateStatus = "archived"
)

type CouponBenefitType string

const (
	CouponBenefitTypeFixedAmount CouponBenefitType = "fixed_amount"
	CouponBenefitTypePercentage  CouponBenefitType = "percentage"
)

type CouponScope string

const (
	CouponScopeBalance      CouponScope = "balance"
	CouponScopeSubscription CouponScope = "subscription"
)

type CouponValidityMode string

const (
	CouponValidityModeRelativeDays CouponValidityMode = "relative_days"
	CouponValidityModeEndOfDay     CouponValidityMode = "end_of_day"
	CouponValidityModeEndOfMonth   CouponValidityMode = "end_of_month"
	CouponValidityModeFixed        CouponValidityMode = "fixed"
)

type UserCouponStatus string

const (
	UserCouponStatusAvailable UserCouponStatus = "available"
	UserCouponStatusLocked    UserCouponStatus = "locked"
	UserCouponStatusUsed      UserCouponStatus = "used"
	UserCouponStatusExpired   UserCouponStatus = "expired"
	UserCouponStatusVoided    UserCouponStatus = "voided"
)

type CouponIssueSource string

const (
	CouponIssueSourceBlindbox   CouponIssueSource = "blindbox"
	CouponIssueSourceQuiz       CouponIssueSource = "quiz"
	CouponIssueSourceAdminBatch CouponIssueSource = "admin_batch"
	CouponIssueSourceManual     CouponIssueSource = "manual"
	CouponIssueSourceCompensate CouponIssueSource = "compensation"
)

type CouponIssueBatchStatus string

const (
	CouponIssueBatchStatusCompleted CouponIssueBatchStatus = "completed"
	CouponIssueBatchStatusFailed    CouponIssueBatchStatus = "failed"
)

type CouponRewardActivity string

const (
	CouponRewardActivityBlindbox CouponRewardActivity = "blindbox"
	CouponRewardActivityQuiz     CouponRewardActivity = "quiz"
)

type CouponRewardPoolStatus string

const (
	CouponRewardPoolStatusDraft     CouponRewardPoolStatus = "draft"
	CouponRewardPoolStatusPublished CouponRewardPoolStatus = "published"
	CouponRewardPoolStatusRetired   CouponRewardPoolStatus = "retired"
)

// CouponTemplate defines the immutable financial terms from which user-owned
// coupons are issued. Issued coupons keep their own terms snapshot.
type CouponTemplate struct {
	ID                 int64                `json:"id"`
	Key                string               `json:"key"`
	Version            int                  `json:"version"`
	Name               string               `json:"name"`
	Description        string               `json:"description,omitempty"`
	Status             CouponTemplateStatus `json:"status"`
	BenefitType        CouponBenefitType    `json:"benefit_type"`
	BenefitValue       float64              `json:"benefit_value"`
	MaxDiscountAmount  *float64             `json:"max_discount_amount,omitempty"`
	Currency           string               `json:"currency"`
	ApplicableScopes   []CouponScope        `json:"applicable_scopes"`
	MinimumOrderAmount float64              `json:"minimum_order_amount"`
	EligiblePlanIDs    []int64              `json:"eligible_plan_ids"`
	ValidityMode       CouponValidityMode   `json:"validity_mode"`
	ValidityDays       int                  `json:"validity_days,omitempty"`
	FixedExpiresAt     *time.Time           `json:"fixed_expires_at,omitempty"`
	ValidFrom          *time.Time           `json:"valid_from,omitempty"`
	TotalIssueLimit    *int64               `json:"total_issue_limit,omitempty"`
	IssuedCount        int64                `json:"issued_count"`
	Rules              map[string]any       `json:"rules,omitempty"`
	CreatedBy          *int64               `json:"created_by,omitempty"`
	UpdatedBy          *int64               `json:"updated_by,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

type CouponTemplateInput struct {
	Key                string               `json:"key"`
	Name               string               `json:"name"`
	Description        string               `json:"description,omitempty"`
	Status             CouponTemplateStatus `json:"status"`
	BenefitType        CouponBenefitType    `json:"benefit_type"`
	BenefitValue       float64              `json:"benefit_value"`
	MaxDiscountAmount  *float64             `json:"max_discount_amount,omitempty"`
	Currency           string               `json:"currency"`
	ApplicableScopes   []CouponScope        `json:"applicable_scopes"`
	MinimumOrderAmount float64              `json:"minimum_order_amount"`
	EligiblePlanIDs    []int64              `json:"eligible_plan_ids"`
	ValidityMode       CouponValidityMode   `json:"validity_mode"`
	ValidityDays       int                  `json:"validity_days,omitempty"`
	FixedExpiresAt     *time.Time           `json:"fixed_expires_at,omitempty"`
	ValidFrom          *time.Time           `json:"valid_from,omitempty"`
	TotalIssueLimit    *int64               `json:"total_issue_limit,omitempty"`
	Rules              map[string]any       `json:"rules,omitempty"`
}

func (in CouponTemplateInput) Template() CouponTemplate {
	return CouponTemplate{
		Key:                in.Key,
		Name:               in.Name,
		Description:        in.Description,
		Status:             in.Status,
		BenefitType:        in.BenefitType,
		BenefitValue:       in.BenefitValue,
		MaxDiscountAmount:  cloneFloat64(in.MaxDiscountAmount),
		Currency:           in.Currency,
		ApplicableScopes:   append([]CouponScope(nil), in.ApplicableScopes...),
		MinimumOrderAmount: in.MinimumOrderAmount,
		EligiblePlanIDs:    append([]int64(nil), in.EligiblePlanIDs...),
		ValidityMode:       in.ValidityMode,
		ValidityDays:       in.ValidityDays,
		FixedExpiresAt:     cloneTime(in.FixedExpiresAt),
		ValidFrom:          cloneTime(in.ValidFrom),
		TotalIssueLimit:    cloneInt64(in.TotalIssueLimit),
		Rules:              cloneMap(in.Rules),
	}
}

// CouponTermsSnapshot is copied to every user coupon so a later template edit
// cannot change an already issued discount or expiry rule.
type CouponTermsSnapshot struct {
	TemplateID         int64              `json:"template_id"`
	TemplateVersion    int                `json:"template_version"`
	TemplateKey        string             `json:"template_key"`
	Name               string             `json:"name"`
	Description        string             `json:"description,omitempty"`
	BenefitType        CouponBenefitType  `json:"benefit_type"`
	BenefitValue       float64            `json:"benefit_value"`
	MaxDiscountAmount  *float64           `json:"max_discount_amount,omitempty"`
	Currency           string             `json:"currency"`
	ApplicableScopes   []CouponScope      `json:"applicable_scopes"`
	MinimumOrderAmount float64            `json:"minimum_order_amount"`
	EligiblePlanIDs    []int64            `json:"eligible_plan_ids"`
	ValidityMode       CouponValidityMode `json:"validity_mode"`
	ValidityDays       int                `json:"validity_days,omitempty"`
	Rules              map[string]any     `json:"rules,omitempty"`
}

func CouponTermsFromTemplate(template CouponTemplate) CouponTermsSnapshot {
	return CouponTermsSnapshot{
		TemplateID:         template.ID,
		TemplateVersion:    template.Version,
		TemplateKey:        template.Key,
		Name:               template.Name,
		Description:        template.Description,
		BenefitType:        template.BenefitType,
		BenefitValue:       template.BenefitValue,
		MaxDiscountAmount:  cloneFloat64(template.MaxDiscountAmount),
		Currency:           template.Currency,
		ApplicableScopes:   append([]CouponScope(nil), template.ApplicableScopes...),
		MinimumOrderAmount: template.MinimumOrderAmount,
		EligiblePlanIDs:    append([]int64(nil), template.EligiblePlanIDs...),
		ValidityMode:       template.ValidityMode,
		ValidityDays:       template.ValidityDays,
		Rules:              cloneMap(template.Rules),
	}
}

type UserCoupon struct {
	ID                      int64               `json:"id"`
	TemplateID              int64               `json:"template_id"`
	TemplateName            string              `json:"template_name,omitempty"`
	UserID                  int64               `json:"user_id"`
	UserEmail               string              `json:"user_email,omitempty"`
	UserName                string              `json:"user_name,omitempty"`
	Status                  UserCouponStatus    `json:"status"`
	TermsSnapshot           CouponTermsSnapshot `json:"terms_snapshot"`
	Source                  CouponIssueSource   `json:"source"`
	SourceRef               string              `json:"source_ref,omitempty"`
	IssueBatchID            *int64              `json:"issue_batch_id,omitempty"`
	IdempotencyKey          string              `json:"idempotency_key"`
	IssuedAt                time.Time           `json:"issued_at"`
	ValidFrom               time.Time           `json:"valid_from"`
	ExpiresAt               time.Time           `json:"expires_at"`
	LockedOrderID           *int64              `json:"locked_order_id,omitempty"`
	LockedAt                *time.Time          `json:"locked_at,omitempty"`
	UsedOrderID             *int64              `json:"used_order_id,omitempty"`
	UsedOrderNo             string              `json:"used_order_no,omitempty"`
	UsedOrderType           string              `json:"used_order_type,omitempty"`
	UsedOrderStatus         string              `json:"used_order_status,omitempty"`
	UsedOrderAmount         float64             `json:"used_order_amount,omitempty"`
	UsedOrderPayAmount      float64             `json:"used_order_pay_amount,omitempty"`
	UsedOrderDiscountAmount float64             `json:"used_order_discount_amount,omitempty"`
	UsedOrderCurrency       string              `json:"used_order_currency,omitempty"`
	UsedAt                  *time.Time          `json:"used_at,omitempty"`
	VoidedAt                *time.Time          `json:"voided_at,omitempty"`
	VoidReason              string              `json:"void_reason,omitempty"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
}

type CouponIssueBatch struct {
	ID             int64                  `json:"id"`
	TemplateID     int64                  `json:"template_id"`
	TemplateName   string                 `json:"template_name,omitempty"`
	Source         CouponIssueSource      `json:"source"`
	RequestedCount int                    `json:"requested_count"`
	IssuedCount    int                    `json:"issued_count"`
	FailedCount    int                    `json:"failed_count"`
	Status         CouponIssueBatchStatus `json:"status"`
	IdempotencyKey string                 `json:"idempotency_key"`
	InputSnapshot  map[string]any         `json:"input_snapshot,omitempty"`
	CreatedBy      *int64                 `json:"created_by,omitempty"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

type CouponRewardPoolVersion struct {
	ID                 int64                   `json:"id"`
	Activity           CouponRewardActivity    `json:"activity"`
	Version            string                  `json:"version"`
	Status             CouponRewardPoolStatus  `json:"status"`
	CouponWeightBP     int                     `json:"coupon_weight_bp"`
	BalanceWeightBP    int                     `json:"balance_weight_bp"`
	FallbackTemplateID int64                   `json:"fallback_template_id"`
	Entries            []CouponRewardPoolEntry `json:"entries"`
	CreatedBy          *int64                  `json:"created_by,omitempty"`
	UpdatedBy          *int64                  `json:"updated_by,omitempty"`
	PublishedAt        *time.Time              `json:"published_at,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type CouponRewardPoolEntry struct {
	ID                int64      `json:"id"`
	PoolVersionID     int64      `json:"pool_version_id"`
	TemplateID        int64      `json:"template_id"`
	TemplateName      string     `json:"template_name,omitempty"`
	WeightBP          int        `json:"weight_bp"`
	Enabled           bool       `json:"enabled"`
	StartsAt          *time.Time `json:"starts_at,omitempty"`
	EndsAt            *time.Time `json:"ends_at,omitempty"`
	StockCap          *int64     `json:"stock_cap,omitempty"`
	IssuedCount       int64      `json:"issued_count"`
	PerUserIssueLimit *int       `json:"per_user_issue_limit,omitempty"`
	SortOrder         int        `json:"sort_order"`
}

type CouponTemplateListFilter struct {
	Status   CouponTemplateStatus
	Search   string
	Page     int
	PageSize int
}

type UserCouponListFilter struct {
	UserID     int64
	UserQuery  string
	TemplateID int64
	Source     CouponIssueSource
	Status     UserCouponStatus
	IssuedFrom *time.Time
	IssuedTo   *time.Time
	Page       int
	PageSize   int
}

type CouponIssueBatchListFilter struct {
	TemplateID int64
	Page       int
	PageSize   int
}

type CouponIssueInput struct {
	TemplateID     int64
	UserID         int64
	Source         CouponIssueSource
	SourceRef      string
	IssueBatchID   *int64
	IdempotencyKey string
	ActorID        int64
}

type CouponBatchIssueInput struct {
	TemplateID     int64
	UserIDs        []int64
	Source         CouponIssueSource
	IdempotencyKey string
	ActorID        int64
	Metadata       map[string]any
}

type CouponOrderContext struct {
	UserID      int64
	Scope       CouponScope
	OrderAmount float64
	PlanID      int64
	Currency    string
	At          time.Time
}

type CouponQuote struct {
	UserCouponID   int64   `json:"user_coupon_id"`
	TemplateID     int64   `json:"template_id"`
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	PayableAmount  float64 `json:"payable_amount"`
	Currency       string  `json:"currency"`
}

func ValidateCouponTemplate(template CouponTemplate) error {
	template = normalizeCouponTemplate(template)
	if template.Key == "" || len(template.Key) > 64 {
		return fmt.Errorf("coupon template key is required and must be at most 64 characters")
	}
	if template.Name == "" || len(template.Name) > 120 {
		return fmt.Errorf("coupon template name is required and must be at most 120 characters")
	}
	if !isCouponTemplateStatus(template.Status) {
		return fmt.Errorf("coupon template status is invalid")
	}
	if !isCouponBenefitType(template.BenefitType) {
		return fmt.Errorf("coupon benefit type is invalid")
	}
	if !couponFinitePositive(template.BenefitValue) {
		return fmt.Errorf("coupon benefit value must be positive and finite")
	}
	if template.BenefitType == CouponBenefitTypePercentage && template.BenefitValue > 100 {
		return fmt.Errorf("coupon percentage benefit value must be within (0, 100]")
	}
	if template.MaxDiscountAmount != nil && !couponFinitePositive(*template.MaxDiscountAmount) {
		return fmt.Errorf("coupon max discount amount must be positive and finite")
	}
	if len(template.Currency) != 3 {
		return fmt.Errorf("coupon currency must be a three-letter code")
	}
	if len(template.ApplicableScopes) == 0 {
		return fmt.Errorf("coupon applicable scopes are required")
	}
	for _, scope := range template.ApplicableScopes {
		if !isCouponScope(scope) {
			return fmt.Errorf("coupon applicable scope is invalid")
		}
	}
	if !couponFiniteNonNegative(template.MinimumOrderAmount) {
		return fmt.Errorf("coupon minimum order amount must be non-negative and finite")
	}
	for _, planID := range template.EligiblePlanIDs {
		if planID <= 0 {
			return fmt.Errorf("coupon eligible plan ids must be positive")
		}
	}
	if !isCouponValidityMode(template.ValidityMode) {
		return fmt.Errorf("coupon validity mode is invalid")
	}
	switch template.ValidityMode {
	case CouponValidityModeRelativeDays:
		if template.ValidityDays < 1 || template.ValidityDays > 3650 {
			return fmt.Errorf("coupon validity days must be between 1 and 3650")
		}
	case CouponValidityModeFixed:
		if template.FixedExpiresAt == nil {
			return fmt.Errorf("coupon fixed expiry is required")
		}
		if template.ValidityDays != 0 {
			return fmt.Errorf("coupon validity days are only allowed for relative_days")
		}
		if template.ValidFrom != nil && !template.FixedExpiresAt.After(*template.ValidFrom) {
			return fmt.Errorf("coupon fixed expiry must be after valid from")
		}
	default:
		if template.ValidityDays != 0 {
			return fmt.Errorf("coupon validity days are only allowed for relative_days")
		}
	}
	if template.TotalIssueLimit != nil && *template.TotalIssueLimit < 1 {
		return fmt.Errorf("coupon total issue limit must be positive")
	}
	if template.IssuedCount < 0 {
		return fmt.Errorf("coupon issued count must be non-negative")
	}
	return nil
}

func ValidateCouponRewardPool(pool CouponRewardPoolVersion) error {
	pool = normalizeCouponRewardPool(pool)
	if !isCouponRewardActivity(pool.Activity) {
		return fmt.Errorf("coupon reward pool activity is invalid")
	}
	if pool.Version == "" || len(pool.Version) > 80 {
		return fmt.Errorf("coupon reward pool version is required and must be at most 80 characters")
	}
	if !isCouponRewardPoolStatus(pool.Status) {
		return fmt.Errorf("coupon reward pool status is invalid")
	}
	if pool.CouponWeightBP+pool.BalanceWeightBP != couponWeightBasisPoints {
		return fmt.Errorf("coupon reward pool outer weights must total %d", couponWeightBasisPoints)
	}
	switch pool.Activity {
	case CouponRewardActivityBlindbox:
		if pool.CouponWeightBP != 6000 || pool.BalanceWeightBP != 4000 {
			return fmt.Errorf("blindbox coupon weight must be 6000 and balance weight must be 4000")
		}
	case CouponRewardActivityQuiz:
		if pool.CouponWeightBP != 8000 || pool.BalanceWeightBP != 2000 {
			return fmt.Errorf("quiz coupon weight must be 8000 and balance weight must be 2000")
		}
	}
	if pool.FallbackTemplateID <= 0 {
		return fmt.Errorf("coupon reward pool fallback template is required")
	}
	if len(pool.Entries) == 0 || len(pool.Entries) > 100 {
		return fmt.Errorf("coupon reward pool must contain between 1 and 100 entries")
	}
	var ordinaryWeight int
	var fallbackEntry *CouponRewardPoolEntry
	seenTemplates := make(map[int64]struct{}, len(pool.Entries))
	for i, entry := range pool.Entries {
		if entry.TemplateID <= 0 {
			return fmt.Errorf("coupon reward pool entry %d template is required", i)
		}
		if _, exists := seenTemplates[entry.TemplateID]; exists {
			return fmt.Errorf("coupon reward pool cannot include a template more than once")
		}
		seenTemplates[entry.TemplateID] = struct{}{}
		if entry.WeightBP <= 0 || entry.WeightBP > couponWeightBasisPoints {
			return fmt.Errorf("coupon reward pool entry weights must total %d", couponWeightBasisPoints)
		}
		isFallback := entry.TemplateID == pool.FallbackTemplateID
		if isFallback {
			// PostgreSQL keeps a positive weight constraint for every entry. The
			// fallback is not a weighted outcome, so persist its harmless marker
			// value rather than allowing an operator to mistake it for a chance.
			if entry.WeightBP != 1 {
				return fmt.Errorf("coupon reward pool fallback entry weight must be 1")
			}
		} else {
			ordinaryWeight += entry.WeightBP
			if ordinaryWeight > couponWeightBasisPoints {
				return fmt.Errorf("coupon reward pool entry weights must total %d", couponWeightBasisPoints)
			}
		}
		if entry.StockCap != nil && *entry.StockCap < 1 {
			return fmt.Errorf("coupon reward pool entry stock cap must be positive")
		}
		if entry.PerUserIssueLimit != nil && *entry.PerUserIssueLimit < 1 {
			return fmt.Errorf("coupon reward pool entry per-user issue limit must be positive")
		}
		if entry.StartsAt != nil && entry.EndsAt != nil && !entry.EndsAt.After(*entry.StartsAt) {
			return fmt.Errorf("coupon reward pool entry end must be after start")
		}
		if isFallback && entry.Enabled {
			fallback := entry
			fallbackEntry = &fallback
		}
	}
	if ordinaryWeight != couponWeightBasisPoints {
		return fmt.Errorf("coupon reward pool entry weights must total %d", couponWeightBasisPoints)
	}
	if fallbackEntry == nil {
		return fmt.Errorf("coupon reward pool fallback template must be an enabled entry")
	}
	// The outer play draw has a fixed coupon branch. A user who has already
	// reached every normal entry's per-user cap must still receive a coupon
	// when that branch is selected, otherwise the result would become a random
	// failure and silently distort the configured 60/40 or 80/20 split.
	if fallbackEntry.PerUserIssueLimit != nil {
		return fmt.Errorf("coupon reward pool fallback entry cannot set a per-user issue limit")
	}
	if fallbackEntry.StockCap != nil {
		return fmt.Errorf("coupon reward pool fallback entry cannot set a stock cap")
	}
	if fallbackEntry.StartsAt != nil || fallbackEntry.EndsAt != nil {
		return fmt.Errorf("coupon reward pool fallback entry cannot set an active window")
	}
	return nil
}

func CouponExpiryForIssue(template CouponTemplate, issuedAt time.Time) (time.Time, time.Time, error) {
	if err := ValidateCouponTemplate(template); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if issuedAt.IsZero() {
		issuedAt = timezone.Now()
	}
	// Calendar-based validity follows the configured server business timezone.
	// Store the resulting instants in PostgreSQL, but calculate day/month
	// boundaries in the same timezone used by the rest of the application.
	loc := timezone.Location()
	issuedAt = issuedAt.In(loc)
	validFrom := issuedAt
	if template.ValidFrom != nil && template.ValidFrom.After(validFrom) {
		validFrom = template.ValidFrom.In(loc)
	}
	var expiresAt time.Time
	switch template.ValidityMode {
	case CouponValidityModeRelativeDays:
		expiresAt = validFrom.AddDate(0, 0, template.ValidityDays)
	case CouponValidityModeEndOfDay:
		expiresAt = time.Date(validFrom.Year(), validFrom.Month(), validFrom.Day()+1, 0, 0, 0, 0, validFrom.Location())
	case CouponValidityModeEndOfMonth:
		expiresAt = time.Date(validFrom.Year(), validFrom.Month()+1, 1, 0, 0, 0, 0, validFrom.Location())
	case CouponValidityModeFixed:
		expiresAt = template.FixedExpiresAt.In(loc)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported coupon validity mode")
	}
	if !expiresAt.After(validFrom) {
		return time.Time{}, time.Time{}, fmt.Errorf("coupon expiry must be after valid from")
	}
	return validFrom, expiresAt, nil
}

func QuoteUserCoupon(coupon UserCoupon, context CouponOrderContext) (*CouponQuote, error) {
	at := context.At
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC()
	if coupon.ID <= 0 || coupon.UserID <= 0 || coupon.UserID != context.UserID {
		return nil, infraerrors.NotFound("COUPON_NOT_FOUND", "coupon not found")
	}
	if coupon.Status != UserCouponStatusAvailable {
		return nil, infraerrors.Conflict("COUPON_NOT_AVAILABLE", "coupon is not available")
	}
	if coupon.ValidFrom.After(at) || !coupon.ExpiresAt.After(at) {
		return nil, infraerrors.Conflict("COUPON_EXPIRED", "coupon is outside its validity window")
	}
	terms := coupon.TermsSnapshot
	if !isCouponBenefitType(terms.BenefitType) || !couponFinitePositive(terms.BenefitValue) {
		return nil, infraerrors.Conflict("COUPON_TERMS_INVALID", "coupon terms are invalid")
	}
	if !isCouponScope(context.Scope) || !couponHasScope(terms.ApplicableScopes, context.Scope) {
		return nil, infraerrors.Conflict("COUPON_SCOPE_MISMATCH", "coupon cannot be used for this order")
	}
	if !couponFinitePositive(context.OrderAmount) {
		return nil, infraerrors.BadRequest("INVALID_ORDER_AMOUNT", "order amount must be positive")
	}
	if context.OrderAmount+1e-9 < terms.MinimumOrderAmount {
		return nil, infraerrors.Conflict("COUPON_MINIMUM_NOT_MET", "coupon minimum order amount is not met")
	}
	if len(terms.EligiblePlanIDs) > 0 && !couponContainsInt64(terms.EligiblePlanIDs, context.PlanID) {
		return nil, infraerrors.Conflict("COUPON_PLAN_MISMATCH", "coupon cannot be used for this subscription plan")
	}
	currency := strings.ToUpper(strings.TrimSpace(context.Currency))
	if currency == "" {
		currency = terms.Currency
	}
	if terms.Currency != "" && currency != terms.Currency {
		return nil, infraerrors.Conflict("COUPON_CURRENCY_MISMATCH", "coupon currency does not match this order")
	}

	precision := int32(payment.CurrencyMaxFractionDigits(currency))
	amount := decimal.NewFromFloat(context.OrderAmount).Round(precision)
	var discount decimal.Decimal
	switch terms.BenefitType {
	case CouponBenefitTypeFixedAmount:
		discount = decimal.NewFromFloat(terms.BenefitValue)
	case CouponBenefitTypePercentage:
		discount = amount.Mul(decimal.NewFromFloat(terms.BenefitValue)).Div(decimal.NewFromInt(100))
	default:
		return nil, infraerrors.Conflict("COUPON_TERMS_INVALID", "coupon benefit type is invalid")
	}
	if terms.MaxDiscountAmount != nil {
		cap := decimal.NewFromFloat(*terms.MaxDiscountAmount)
		if discount.GreaterThan(cap) {
			discount = cap
		}
	}
	if discount.GreaterThan(amount) {
		discount = amount
	}
	discount = discount.Round(precision)
	payable := amount.Sub(discount).Round(precision)
	if payable.IsNegative() {
		payable = decimal.Zero
	}
	return &CouponQuote{
		UserCouponID:   coupon.ID,
		TemplateID:     coupon.TemplateID,
		OriginalAmount: amount.InexactFloat64(),
		DiscountAmount: discount.InexactFloat64(),
		PayableAmount:  payable.InexactFloat64(),
		Currency:       currency,
	}, nil
}

func normalizeCouponTemplate(template CouponTemplate) CouponTemplate {
	template.Key = strings.ToLower(strings.TrimSpace(template.Key))
	template.Name = strings.TrimSpace(template.Name)
	template.Description = strings.TrimSpace(template.Description)
	template.Currency = strings.ToUpper(strings.TrimSpace(template.Currency))
	template.ApplicableScopes = normalizeCouponScopes(template.ApplicableScopes)
	template.EligiblePlanIDs = normalizeCouponPlanIDs(template.EligiblePlanIDs)
	if template.Rules == nil {
		template.Rules = map[string]any{}
	}
	return template
}

func normalizeCouponRewardPool(pool CouponRewardPoolVersion) CouponRewardPoolVersion {
	pool.Version = strings.TrimSpace(pool.Version)
	for i := range pool.Entries {
		pool.Entries[i].TemplateName = strings.TrimSpace(pool.Entries[i].TemplateName)
	}
	return pool
}

func isCouponTemplateStatus(status CouponTemplateStatus) bool {
	switch status {
	case CouponTemplateStatusDraft, CouponTemplateStatusActive, CouponTemplateStatusPaused, CouponTemplateStatusArchived:
		return true
	default:
		return false
	}
}

func isCouponBenefitType(benefit CouponBenefitType) bool {
	return benefit == CouponBenefitTypeFixedAmount || benefit == CouponBenefitTypePercentage
}

func isCouponScope(scope CouponScope) bool {
	return scope == CouponScopeBalance || scope == CouponScopeSubscription
}

func isCouponValidityMode(mode CouponValidityMode) bool {
	switch mode {
	case CouponValidityModeRelativeDays, CouponValidityModeEndOfDay, CouponValidityModeEndOfMonth, CouponValidityModeFixed:
		return true
	default:
		return false
	}
}

func isCouponIssueSource(source CouponIssueSource) bool {
	switch source {
	case CouponIssueSourceBlindbox, CouponIssueSourceQuiz, CouponIssueSourceAdminBatch, CouponIssueSourceManual, CouponIssueSourceCompensate:
		return true
	default:
		return false
	}
}

func isCouponRewardActivity(activity CouponRewardActivity) bool {
	return activity == CouponRewardActivityBlindbox || activity == CouponRewardActivityQuiz
}

func isCouponRewardPoolStatus(status CouponRewardPoolStatus) bool {
	return status == CouponRewardPoolStatusDraft || status == CouponRewardPoolStatusPublished || status == CouponRewardPoolStatusRetired
}

func couponFinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func couponFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func couponHasScope(scopes []CouponScope, target CouponScope) bool {
	for _, scope := range scopes {
		if scope == target {
			return true
		}
	}
	return false
}

func couponContainsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func normalizeCouponScopes(scopes []CouponScope) []CouponScope {
	seen := make(map[CouponScope]struct{}, len(scopes))
	out := make([]CouponScope, 0, len(scopes))
	for _, scope := range scopes {
		scope = CouponScope(strings.ToLower(strings.TrimSpace(string(scope))))
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func normalizeCouponPlanIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	copy := make(map[string]any, len(value))
	for key, item := range value {
		copy[key] = item
	}
	return copy
}
