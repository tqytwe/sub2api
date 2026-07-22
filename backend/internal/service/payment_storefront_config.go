package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingPaymentStorefrontConfig = "PAYMENT_STOREFRONT_CONFIG"

const (
	maxPaymentStorefrontShelves = 24
	maxPaymentStorefrontTags    = 48
)

type PaymentStorefrontConfig struct {
	Shelves []PaymentStorefrontShelf `json:"shelves"`
	Tags    []PaymentStorefrontTag   `json:"tags"`
}

type PaymentStorefrontShelf struct {
	ID            string  `json:"id"`
	Label         string  `json:"label"`
	Enabled       bool    `json:"enabled"`
	SortOrder     int     `json:"sort_order"`
	PlanIDs       []int64 `json:"plan_ids"`
	DefaultPlanID *int64  `json:"default_plan_id,omitempty"`
}

type PaymentStorefrontTag struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Tone      string  `json:"tone"`
	Enabled   bool    `json:"enabled"`
	SortOrder int     `json:"sort_order"`
	PlanIDs   []int64 `json:"plan_ids"`
}

func (s *PaymentConfigService) GetPaymentStorefrontConfig(ctx context.Context, plans []*dbent.SubscriptionPlan) (*PaymentStorefrontConfig, error) {
	return s.loadPaymentStorefrontConfig(ctx, plans, false)
}

func (s *PaymentConfigService) GetPublicPaymentStorefrontConfig(ctx context.Context, plans []*dbent.SubscriptionPlan) (*PaymentStorefrontConfig, error) {
	return s.loadPaymentStorefrontConfig(ctx, plans, true)
}

func (s *PaymentConfigService) UpdatePaymentStorefrontConfig(ctx context.Context, req PaymentStorefrontConfig) (*PaymentStorefrontConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, infraerrors.InternalServer("PAYMENT_STOREFRONT_CONFIG_UNAVAILABLE", "payment storefront config store is unavailable")
	}
	plans, err := s.ListPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("list payment plans for storefront config: %w", err)
	}
	cfg, err := normalizePaymentStorefrontConfig(req, planIDSet(plans), false)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal payment storefront config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingPaymentStorefrontConfig, string(raw)); err != nil {
		return nil, fmt.Errorf("save payment storefront config: %w", err)
	}
	return cfg, nil
}

func (s *PaymentConfigService) loadPaymentStorefrontConfig(ctx context.Context, plans []*dbent.SubscriptionPlan, publicOnly bool) (*PaymentStorefrontConfig, error) {
	if s == nil || s.settingRepo == nil {
		cfg := defaultPaymentStorefrontConfig(plans)
		return normalizePaymentStorefrontConfig(cfg, planIDSet(plans), publicOnly)
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingPaymentStorefrontConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			cfg := defaultPaymentStorefrontConfig(plans)
			return normalizePaymentStorefrontConfig(cfg, planIDSet(plans), publicOnly)
		}
		return nil, fmt.Errorf("get payment storefront config: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		cfg := defaultPaymentStorefrontConfig(plans)
		return normalizePaymentStorefrontConfig(cfg, planIDSet(plans), publicOnly)
	}

	var cfg PaymentStorefrontConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_CONFIG_INVALID", "payment storefront config is invalid JSON")
	}
	normalized, err := normalizePaymentStorefrontConfig(cfg, planIDSet(plans), publicOnly)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizePaymentStorefrontConfig(cfg PaymentStorefrontConfig, validPlanIDs map[int64]bool, publicOnly bool) (*PaymentStorefrontConfig, error) {
	if len(cfg.Shelves) > maxPaymentStorefrontShelves {
		return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_SHELVES_TOO_MANY", "too many storefront shelves")
	}
	if len(cfg.Tags) > maxPaymentStorefrontTags {
		return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_TAGS_TOO_MANY", "too many storefront tags")
	}

	shelves, err := normalizePaymentStorefrontShelves(cfg.Shelves, validPlanIDs, publicOnly)
	if err != nil {
		return nil, err
	}
	tags, err := normalizePaymentStorefrontTags(cfg.Tags, validPlanIDs, publicOnly)
	if err != nil {
		return nil, err
	}
	return &PaymentStorefrontConfig{Shelves: shelves, Tags: tags}, nil
}

func normalizePaymentStorefrontShelves(items []PaymentStorefrontShelf, validPlanIDs map[int64]bool, publicOnly bool) ([]PaymentStorefrontShelf, error) {
	seenIDs := map[string]bool{}
	seenLabels := map[string]bool{}
	out := make([]PaymentStorefrontShelf, 0, len(items))
	for i, item := range items {
		if publicOnly && !item.Enabled {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_SHELF_LABEL_REQUIRED", "storefront shelf label is required")
		}
		if len([]rune(label)) > 32 {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_SHELF_LABEL_TOO_LONG", "storefront shelf label is too long")
		}
		id := normalizeStorefrontItemID(item.ID, label, "shelf", i+1)
		if seenIDs[id] {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_SHELF_ID_DUPLICATE", "storefront shelf id must be unique")
		}
		seenIDs[id] = true
		labelKey := strings.ToLower(label)
		if seenLabels[labelKey] {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_SHELF_LABEL_DUPLICATE", "storefront shelf label must be unique")
		}
		seenLabels[labelKey] = true

		planIDs := normalizeStorefrontPlanIDs(item.PlanIDs, validPlanIDs)
		if publicOnly && len(planIDs) == 0 {
			continue
		}
		defaultPlanID := normalizeStorefrontDefaultPlanID(item.DefaultPlanID, planIDs)
		out = append(out, PaymentStorefrontShelf{
			ID:            id,
			Label:         label,
			Enabled:       item.Enabled,
			SortOrder:     normalizedSortOrder(item.SortOrder, i),
			PlanIDs:       planIDs,
			DefaultPlanID: defaultPlanID,
		})
	}
	sortPaymentStorefrontShelves(out)
	for i := range out {
		out[i].SortOrder = i + 1
	}
	return out, nil
}

func normalizePaymentStorefrontTags(items []PaymentStorefrontTag, validPlanIDs map[int64]bool, publicOnly bool) ([]PaymentStorefrontTag, error) {
	seenIDs := map[string]bool{}
	seenLabels := map[string]bool{}
	out := make([]PaymentStorefrontTag, 0, len(items))
	for i, item := range items {
		if publicOnly && !item.Enabled {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_TAG_LABEL_REQUIRED", "storefront tag label is required")
		}
		if len([]rune(label)) > 24 {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_TAG_LABEL_TOO_LONG", "storefront tag label is too long")
		}
		id := normalizeStorefrontItemID(item.ID, label, "tag", i+1)
		if seenIDs[id] {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_TAG_ID_DUPLICATE", "storefront tag id must be unique")
		}
		seenIDs[id] = true
		labelKey := strings.ToLower(label)
		if seenLabels[labelKey] {
			return nil, infraerrors.BadRequest("PAYMENT_STOREFRONT_TAG_LABEL_DUPLICATE", "storefront tag label must be unique")
		}
		seenLabels[labelKey] = true

		planIDs := normalizeStorefrontPlanIDs(item.PlanIDs, validPlanIDs)
		if publicOnly && len(planIDs) == 0 {
			continue
		}
		out = append(out, PaymentStorefrontTag{
			ID:        id,
			Label:     label,
			Tone:      normalizeStorefrontTagTone(item.Tone),
			Enabled:   item.Enabled,
			SortOrder: normalizedSortOrder(item.SortOrder, i),
			PlanIDs:   planIDs,
		})
	}
	sortPaymentStorefrontTags(out)
	for i := range out {
		out[i].SortOrder = i + 1
	}
	return out, nil
}

func defaultPaymentStorefrontConfig(plans []*dbent.SubscriptionPlan) PaymentStorefrontConfig {
	sortedPlans := append([]*dbent.SubscriptionPlan(nil), plans...)
	sort.SliceStable(sortedPlans, func(i, j int) bool {
		if sortedPlans[i].StorefrontFeatured != sortedPlans[j].StorefrontFeatured {
			return sortedPlans[i].StorefrontFeatured
		}
		if sortedPlans[i].SortOrder != sortedPlans[j].SortOrder {
			return sortedPlans[i].SortOrder < sortedPlans[j].SortOrder
		}
		return sortedPlans[i].ID < sortedPlans[j].ID
	})

	allIDs := make([]int64, 0, len(sortedPlans))
	featuredIDs := []int64{}
	categoryIDs := map[string][]int64{}
	badgeIDs := map[string][]int64{}
	for _, plan := range sortedPlans {
		id := int64(plan.ID)
		allIDs = append(allIDs, id)
		if plan.StorefrontFeatured {
			featuredIDs = append(featuredIDs, id)
		}
		category := normalizePlanStorefrontCategory(plan.StorefrontCategory, plan.Name, plan.ValidityDays)
		categoryIDs[category] = append(categoryIDs[category], id)
		if badge := strings.TrimSpace(plan.StorefrontBadge); badge != "" {
			badgeIDs[badge] = append(badgeIDs[badge], id)
		}
	}

	shelves := []PaymentStorefrontShelf{}
	if len(featuredIDs) > 0 {
		shelves = append(shelves, PaymentStorefrontShelf{
			ID:            "featured",
			Label:         "推荐套餐",
			Enabled:       true,
			SortOrder:     len(shelves) + 1,
			PlanIDs:       featuredIDs,
			DefaultPlanID: defaultStorefrontPlanID(featuredIDs, sortedPlans),
		})
	}
	if len(allIDs) > 0 {
		shelves = append(shelves, PaymentStorefrontShelf{
			ID:            "all",
			Label:         "全部套餐",
			Enabled:       true,
			SortOrder:     len(shelves) + 1,
			PlanIDs:       allIDs,
			DefaultPlanID: defaultStorefrontPlanID(allIDs, sortedPlans),
		})
	}
	for _, category := range []string{"daily", "pro", "credit", "image", "team", "enterprise"} {
		ids := categoryIDs[category]
		if len(ids) == 0 {
			continue
		}
		shelves = append(shelves, PaymentStorefrontShelf{
			ID:            category,
			Label:         defaultStorefrontCategoryLabel(category),
			Enabled:       true,
			SortOrder:     len(shelves) + 1,
			PlanIDs:       ids,
			DefaultPlanID: defaultStorefrontPlanID(ids, sortedPlans),
		})
	}

	tags := []PaymentStorefrontTag{}
	if len(featuredIDs) > 0 {
		tags = append(tags, PaymentStorefrontTag{
			ID:        "featured",
			Label:     "推荐",
			Tone:      "primary",
			Enabled:   true,
			SortOrder: len(tags) + 1,
			PlanIDs:   featuredIDs,
		})
	}
	badges := make([]string, 0, len(badgeIDs))
	for badge := range badgeIDs {
		badges = append(badges, badge)
	}
	sort.Strings(badges)
	for _, badge := range badges {
		tags = append(tags, PaymentStorefrontTag{
			ID:        normalizeStorefrontItemID("", badge, "badge", len(tags)+1),
			Label:     badge,
			Tone:      "neutral",
			Enabled:   true,
			SortOrder: len(tags) + 1,
			PlanIDs:   badgeIDs[badge],
		})
	}

	return PaymentStorefrontConfig{Shelves: shelves, Tags: tags}
}

func planIDSet(plans []*dbent.SubscriptionPlan) map[int64]bool {
	out := make(map[int64]bool, len(plans))
	for _, plan := range plans {
		out[int64(plan.ID)] = true
	}
	return out
}

func normalizeStorefrontPlanIDs(values []int64, validPlanIDs map[int64]bool) []int64 {
	out := make([]int64, 0, len(values))
	seen := map[int64]bool{}
	for _, id := range values {
		if id <= 0 || seen[id] {
			continue
		}
		if validPlanIDs != nil && !validPlanIDs[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func normalizeStorefrontDefaultPlanID(value *int64, planIDs []int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	for _, id := range planIDs {
		if id == *value {
			v := *value
			return &v
		}
	}
	return nil
}

func normalizeStorefrontItemID(raw string, label string, prefix string, index int) string {
	source := strings.TrimSpace(strings.ToLower(raw))
	if source == "" {
		source = strings.TrimSpace(strings.ToLower(label))
	}
	var b strings.Builder
	lastDash := false
	for _, r := range source {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			_, _ = b.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-':
			if !lastDash {
				_, _ = b.WriteRune('-')
				lastDash = true
			}
		case unicode.IsSpace(r):
			if !lastDash {
				_, _ = b.WriteRune('-')
				lastDash = true
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = fmt.Sprintf("%s-%d", prefix, index)
	}
	if len(id) > 64 {
		id = strings.Trim(id[:64], "-")
	}
	if id == "" {
		return fmt.Sprintf("%s-%d", prefix, index)
	}
	return id
}

func normalizeStorefrontTagTone(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "primary", "success", "warning", "danger", "info", "neutral":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "neutral"
	}
}

func normalizedSortOrder(value int, index int) int {
	if value > 0 {
		return value
	}
	return index + 1
}

func sortPaymentStorefrontShelves(items []PaymentStorefrontShelf) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].ID < items[j].ID
	})
}

func sortPaymentStorefrontTags(items []PaymentStorefrontTag) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].ID < items[j].ID
	})
}

func defaultStorefrontCategoryLabel(category string) string {
	switch category {
	case "daily":
		return "日卡"
	case "credit":
		return "额度包"
	case "image":
		return "图片套餐"
	case "team":
		return "团队套餐"
	case "enterprise":
		return "企业套餐"
	default:
		return "月卡套餐"
	}
}

func defaultStorefrontPlanID(ids []int64, plans []*dbent.SubscriptionPlan) *int64 {
	if len(ids) == 0 {
		return nil
	}
	allowed := map[int64]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	var best *dbent.SubscriptionPlan
	for _, plan := range plans {
		if !allowed[int64(plan.ID)] {
			continue
		}
		if best == nil || storefrontDefaultPlanScore(plan) < storefrontDefaultPlanScore(best) {
			best = plan
		}
	}
	if best == nil {
		v := ids[0]
		return &v
	}
	v := int64(best.ID)
	return &v
}

func storefrontDefaultPlanScore(plan *dbent.SubscriptionPlan) float64 {
	score := 0.0
	category := normalizePlanStorefrontCategory(plan.StorefrontCategory, plan.Name, plan.ValidityDays)
	if category == "daily" || plan.ValidityDays <= 1 {
		score += 100000
	}
	if plan.ValidityDays >= 20 && plan.ValidityDays <= 40 {
		score -= 200
	}
	priceDistance := plan.Price - 30
	if priceDistance < 0 {
		priceDistance = -priceDistance
	}
	score += priceDistance * 10
	score += float64(plan.SortOrder) * 0.01
	score += float64(plan.ID) * 0.0001
	return score
}
