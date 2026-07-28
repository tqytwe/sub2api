package service

import (
	"context"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const PlanQuotaModeRecurring = "recurring"

func normalizePlanQuotaMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return PlanQuotaModeRecurring
	}
	return mode
}

func validatePlanQuotaConfig(rawMode string, quotaLimitUSD *float64, durationHours *int) error {
	mode := normalizePlanQuotaMode(rawMode)
	switch mode {
	case PlanQuotaModeRecurring:
		if quotaLimitUSD != nil || durationHours != nil {
			return infraerrors.BadRequest("PLAN_QUOTA_CONFIG_INVALID", "recurring plans cannot define one-time quota fields")
		}
		return nil
	case DailyCardQuotaModeOneTime:
		if quotaLimitUSD == nil || *quotaLimitUSD <= 0 {
			return infraerrors.BadRequest("PLAN_QUOTA_REQUIRED", "one-time plans require quota_limit_usd > 0")
		}
		if durationHours == nil || *durationHours <= 0 {
			return infraerrors.BadRequest("PLAN_DURATION_REQUIRED", "one-time plans require duration_hours > 0")
		}
		return nil
	default:
		return infraerrors.BadRequest("PLAN_QUOTA_MODE_INVALID", "quota_mode must be recurring or one_time")
	}
}

func validateOneTimePlanDuration(rawMode string, durationHours *int, validityDays int, validityUnit string) error {
	if normalizePlanQuotaMode(rawMode) != DailyCardQuotaModeOneTime || durationHours == nil {
		return nil
	}
	unit := strings.ToLower(strings.TrimSpace(validityUnit))
	if unit != "day" && unit != "days" {
		return infraerrors.BadRequest("PLAN_DURATION_VALIDITY_INVALID", "one-time duration_hours requires day validity_unit")
	}
	if *durationHours > validityDays*24 {
		return infraerrors.BadRequest("PLAN_DURATION_EXCEEDS_VALIDITY", "duration_hours cannot exceed the parent subscription validity")
	}
	return nil
}

// normalizePlanCurrency validates and normalizes the display-only currency label.
// Empty means "no label" and is kept as-is so existing plans stay unchanged.
func normalizePlanCurrency(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	currency, err := payment.NormalizePaymentCurrency(raw)
	if err != nil {
		return "", infraerrors.BadRequest("PLAN_CURRENCY_INVALID", "currency must be a 3-letter ISO currency code")
	}
	return currency, nil
}

func inferPlanStorefrontCategory(name string, validityDays int) string {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	if validityDays == 1 || strings.Contains(normalizedName, "日卡") || strings.Contains(normalizedName, "daily") {
		return "daily"
	}
	if strings.Contains(normalizedName, "团队") || strings.Contains(normalizedName, "team") {
		return "team"
	}
	if strings.Contains(normalizedName, "企业") || strings.Contains(normalizedName, "enterprise") {
		return "enterprise"
	}
	if strings.Contains(normalizedName, "额度") || strings.Contains(normalizedName, "credit") {
		return "credit"
	}
	if strings.Contains(normalizedName, "图片") || strings.Contains(normalizedName, "image") {
		return "image"
	}
	return "pro"
}

func normalizePlanStorefrontCategory(raw string, name string, validityDays int) string {
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		return trimmed
	}
	return inferPlanStorefrontCategory(name, validityDays)
}

// validatePlanRequired checks that all required fields for a plan are provided.
func validatePlanRequired(name string, groupID int64, price float64, validityDays int, validityUnit string, originalPrice *float64) error {
	if strings.TrimSpace(name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if groupID <= 0 {
		return infraerrors.BadRequest("PLAN_GROUP_REQUIRED", "group is required")
	}
	if price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if validityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if strings.TrimSpace(validityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if originalPrice != nil && *originalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	return nil
}

// validatePlanPatch validates only the non-nil fields in a patch update.
func validatePlanPatch(req UpdatePlanRequest) error {
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if req.GroupID != nil && *req.GroupID <= 0 {
		return infraerrors.BadRequest("PLAN_GROUP_REQUIRED", "group is required")
	}
	if req.Price != nil && *req.Price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if req.ValidityDays != nil && *req.ValidityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if req.ValidityUnit != nil && strings.TrimSpace(*req.ValidityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if req.OriginalPrice != nil && *req.OriginalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	return nil
}

// --- Plan CRUD ---

// PlanGroupInfo holds the group details needed for subscription plan display.
type PlanGroupInfo struct {
	Platform           string   `json:"platform"`
	Name               string   `json:"name"`
	RateMultiplier     float64  `json:"rate_multiplier"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakStart          string   `json:"peak_start"`
	PeakEnd            string   `json:"peak_end"`
	PeakRateMultiplier float64  `json:"peak_rate_multiplier"`
	DailyLimitUSD      *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD     *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD    *float64 `json:"monthly_limit_usd"`
	ModelScopes        []string `json:"supported_model_scopes"`
}

// GetGroupInfoMap returns a map of group_id → PlanGroupInfo for the given plans.
func (s *PaymentConfigService) GetGroupInfoMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]PlanGroupInfo {
	ids := make([]int64, 0, len(plans))
	seen := make(map[int64]bool)
	for _, p := range plans {
		if !seen[p.GroupID] {
			seen[p.GroupID] = true
			ids = append(ids, p.GroupID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	groups, err := s.entClient.Group.Query().Where(group.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil
	}
	m := make(map[int64]PlanGroupInfo, len(groups))
	for _, g := range groups {
		m[int64(g.ID)] = PlanGroupInfo{
			Platform:           g.Platform,
			Name:               g.Name,
			RateMultiplier:     g.RateMultiplier,
			PeakRateEnabled:    g.PeakRateEnabled,
			PeakStart:          g.PeakStart,
			PeakEnd:            g.PeakEnd,
			PeakRateMultiplier: g.PeakRateMultiplier,
			DailyLimitUSD:      g.DailyLimitUsd,
			WeeklyLimitUSD:     g.WeeklyLimitUsd,
			MonthlyLimitUSD:    g.MonthlyLimitUsd,
			ModelScopes:        g.SupportedModelScopes,
		}
	}
	return m
}

func (s *PaymentConfigService) ListPlans(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	return s.entClient.SubscriptionPlan.Query().Order(subscriptionplan.BySortOrder()).All(ctx)
}

func (s *PaymentConfigService) ListPlansForSale(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	return s.entClient.SubscriptionPlan.Query().Where(subscriptionplan.ForSaleEQ(true)).Order(subscriptionplan.BySortOrder()).All(ctx)
}

func (s *PaymentConfigService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := validatePlanRequired(req.Name, req.GroupID, req.Price, req.ValidityDays, req.ValidityUnit, req.OriginalPrice); err != nil {
		return nil, err
	}
	currency, err := normalizePlanCurrency(req.Currency)
	if err != nil {
		return nil, err
	}
	quotaMode := normalizePlanQuotaMode(req.QuotaMode)
	if err := validatePlanQuotaConfig(quotaMode, req.QuotaLimitUSD, req.DurationHours); err != nil {
		return nil, err
	}
	if err := validateOneTimePlanDuration(quotaMode, req.DurationHours, req.ValidityDays, req.ValidityUnit); err != nil {
		return nil, err
	}
	b := s.entClient.SubscriptionPlan.Create().
		SetGroupID(req.GroupID).SetName(req.Name).SetDescription(req.Description).
		SetPrice(req.Price).SetCurrency(currency).SetValidityDays(req.ValidityDays).SetValidityUnit(req.ValidityUnit).
		SetQuotaMode(quotaMode).
		SetFeatures(req.Features).SetProductName(req.ProductName).
		SetCoverImageURL(req.CoverImageURL).SetDetailDescription(req.DetailDescription).
		SetStorefrontPlatform(strings.TrimSpace(req.StorefrontPlatform)).
		SetStorefrontCategory(normalizePlanStorefrontCategory(req.StorefrontCategory, req.Name, req.ValidityDays)).
		SetStorefrontFeatured(req.StorefrontFeatured).
		SetStorefrontBadge(strings.TrimSpace(req.StorefrontBadge)).
		SetForSale(req.ForSale).SetSortOrder(req.SortOrder)
	if req.OriginalPrice != nil {
		b.SetOriginalPrice(*req.OriginalPrice)
	}
	if req.QuotaLimitUSD != nil {
		b.SetQuotaLimitUsd(*req.QuotaLimitUSD)
	}
	if req.DurationHours != nil {
		b.SetDurationHours(*req.DurationHours)
	}
	return b.Save(ctx)
}

// UpdatePlan updates a subscription plan by ID (patch semantics).
// NOTE: This function exceeds 30 lines due to per-field nil-check patch update boilerplate
// plus a validation guard for non-nil fields.
func (s *PaymentConfigService) UpdatePlan(ctx context.Context, id int64, req UpdatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := validatePlanPatch(req); err != nil {
		return nil, err
	}
	current, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	quotaMode := current.QuotaMode
	if req.QuotaMode != nil {
		quotaMode = normalizePlanQuotaMode(*req.QuotaMode)
	}
	quotaLimitUSD := current.QuotaLimitUsd
	durationHours := current.DurationHours
	if req.QuotaLimitUSD != nil {
		quotaLimitUSD = req.QuotaLimitUSD
	}
	if req.DurationHours != nil {
		durationHours = req.DurationHours
	}
	if quotaMode == PlanQuotaModeRecurring && req.QuotaMode != nil {
		quotaLimitUSD = nil
		durationHours = nil
	}
	if err := validatePlanQuotaConfig(quotaMode, quotaLimitUSD, durationHours); err != nil {
		return nil, err
	}
	validityDays := current.ValidityDays
	validityUnit := current.ValidityUnit
	if req.ValidityDays != nil {
		validityDays = *req.ValidityDays
	}
	if req.ValidityUnit != nil {
		validityUnit = *req.ValidityUnit
	}
	if err := validateOneTimePlanDuration(quotaMode, durationHours, validityDays, validityUnit); err != nil {
		return nil, err
	}
	u := s.entClient.SubscriptionPlan.UpdateOneID(id).SetQuotaMode(quotaMode)
	if quotaLimitUSD == nil {
		u.ClearQuotaLimitUsd()
	} else {
		u.SetQuotaLimitUsd(*quotaLimitUSD)
	}
	if durationHours == nil {
		u.ClearDurationHours()
	} else {
		u.SetDurationHours(*durationHours)
	}
	if req.GroupID != nil {
		u.SetGroupID(*req.GroupID)
	}
	if req.Name != nil {
		u.SetName(*req.Name)
	}
	if req.Description != nil {
		u.SetDescription(*req.Description)
	}
	if req.Price != nil {
		u.SetPrice(*req.Price)
	}
	if req.OriginalPrice != nil {
		u.SetOriginalPrice(*req.OriginalPrice)
	}
	if req.Currency != nil {
		currency, err := normalizePlanCurrency(*req.Currency)
		if err != nil {
			return nil, err
		}
		u.SetCurrency(currency)
	}
	if req.ValidityDays != nil {
		u.SetValidityDays(*req.ValidityDays)
	}
	if req.ValidityUnit != nil {
		u.SetValidityUnit(*req.ValidityUnit)
	}
	if req.Features != nil {
		u.SetFeatures(*req.Features)
	}
	if req.ProductName != nil {
		u.SetProductName(*req.ProductName)
	}
	if req.CoverImageURL != nil {
		u.SetCoverImageURL(*req.CoverImageURL)
	}
	if req.DetailDescription != nil {
		u.SetDetailDescription(*req.DetailDescription)
	}
	if req.StorefrontPlatform != nil {
		u.SetStorefrontPlatform(strings.TrimSpace(*req.StorefrontPlatform))
	}
	if req.StorefrontCategory != nil {
		u.SetStorefrontCategory(strings.TrimSpace(*req.StorefrontCategory))
	}
	if req.StorefrontFeatured != nil {
		u.SetStorefrontFeatured(*req.StorefrontFeatured)
	}
	if req.StorefrontBadge != nil {
		u.SetStorefrontBadge(strings.TrimSpace(*req.StorefrontBadge))
	}
	if req.ForSale != nil {
		u.SetForSale(*req.ForSale)
	}
	if req.SortOrder != nil {
		u.SetSortOrder(*req.SortOrder)
	}
	return u.Save(ctx)
}

func (s *PaymentConfigService) DeletePlan(ctx context.Context, id int64) error {
	count, err := s.countPendingOrdersByPlan(ctx, id)
	if err != nil {
		return fmt.Errorf("check pending orders: %w", err)
	}
	if count > 0 {
		return infraerrors.Conflict("PENDING_ORDERS",
			fmt.Sprintf("this plan has %d in-progress orders and cannot be deleted — wait for orders to complete first", count))
	}
	return s.entClient.SubscriptionPlan.DeleteOneID(id).Exec(ctx)
}

// GetPlan returns a subscription plan by ID.
func (s *PaymentConfigService) GetPlan(ctx context.Context, id int64) (*dbent.SubscriptionPlan, error) {
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	}
	return plan, nil
}
