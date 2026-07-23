package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	maxAPIOnboardingItems       = 24
	maxAPIOnboardingTitleLength = 64
	maxAPIOnboardingDescLength  = 180
	maxAPIOnboardingBadgeLength = 20
	maxAPIOnboardingMinBalance  = 1_000_000
)

var allowedAPIOnboardingCTAs = map[string]bool{
	"create_key": true,
	"recharge":   true,
	"buy_plan":   true,
	"open_docs":  true,
}

var allowedAPIOnboardingAudiences = map[string]bool{
	"new_users": true,
	"all_users": true,
}

type APIOnboardingConfig struct {
	Enabled  bool                `json:"enabled"`
	Title    string              `json:"title"`
	Subtitle string              `json:"subtitle"`
	Items    []APIOnboardingItem `json:"items"`
}

type APIOnboardingItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Badge       string  `json:"badge"`
	Enabled     bool    `json:"enabled"`
	SortOrder   int     `json:"sort_order"`
	GroupID     *int64  `json:"group_id,omitempty"`
	PlanID      *int64  `json:"plan_id,omitempty"`
	MinBalance  float64 `json:"min_balance,omitempty"`
	CTA         string  `json:"cta"`
	Audience    string  `json:"audience"`
}

func (s *PaymentConfigService) GetAPIOnboardingConfig(ctx context.Context) (*APIOnboardingConfig, error) {
	return s.loadAPIOnboardingConfig(ctx, nil, true)
}

func (s *PaymentConfigService) GetPublicAPIOnboardingConfig(ctx context.Context, plans []*dbent.SubscriptionPlan) (*APIOnboardingConfig, error) {
	return s.loadAPIOnboardingConfig(ctx, planIDSet(plans), false)
}

func (s *PaymentConfigService) UpdateAPIOnboardingConfig(ctx context.Context, req APIOnboardingConfig) (*APIOnboardingConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, infraerrors.InternalServer("API_ONBOARDING_CONFIG_UNAVAILABLE", "api onboarding config store is unavailable")
	}
	plans, err := s.ListPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf("list payment plans for api onboarding config: %w", err)
	}
	cfg, err := normalizeAPIOnboardingConfig(req, planIDSet(plans), true, true)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal api onboarding config: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyAPIOnboardingConfig, string(raw)); err != nil {
		return nil, fmt.Errorf("save api onboarding config: %w", err)
	}
	return cfg, nil
}

func BuildPublicAPIOnboardingConfig(raw string) APIOnboardingConfig {
	cfg, err := parseAPIOnboardingConfig(raw)
	if err != nil {
		return defaultAPIOnboardingConfig()
	}
	normalized, err := normalizeAPIOnboardingConfig(cfg, nil, false, false)
	if err != nil {
		return defaultAPIOnboardingConfig()
	}
	return *normalized
}

func (s *PaymentConfigService) loadAPIOnboardingConfig(ctx context.Context, validPlanIDs map[int64]bool, includeDisabled bool) (*APIOnboardingConfig, error) {
	if s == nil || s.settingRepo == nil {
		cfg := defaultAPIOnboardingConfig()
		return normalizeAPIOnboardingConfig(cfg, validPlanIDs, includeDisabled, false)
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAPIOnboardingConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			cfg := defaultAPIOnboardingConfig()
			return normalizeAPIOnboardingConfig(cfg, validPlanIDs, includeDisabled, false)
		}
		return nil, fmt.Errorf("get api onboarding config: %w", err)
	}
	cfg, err := parseAPIOnboardingConfig(raw)
	if err != nil {
		return nil, infraerrors.BadRequest("API_ONBOARDING_CONFIG_INVALID", "api onboarding config is invalid JSON")
	}
	return normalizeAPIOnboardingConfig(cfg, validPlanIDs, includeDisabled, !includeDisabled)
}

func parseAPIOnboardingConfig(raw string) (APIOnboardingConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return defaultAPIOnboardingConfig(), nil
	}
	var cfg APIOnboardingConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return APIOnboardingConfig{}, err
	}
	return cfg, nil
}

func defaultAPIOnboardingConfig() APIOnboardingConfig {
	return APIOnboardingConfig{
		Enabled:  false,
		Title:    "创建 API Key",
		Subtitle: "选择适合的接入方式，创建 Key 后即可在代码或客户端中调用模型。",
		Items:    []APIOnboardingItem{},
	}
}

func normalizeAPIOnboardingConfig(cfg APIOnboardingConfig, validPlanIDs map[int64]bool, includeDisabled bool, rejectInvalid bool) (*APIOnboardingConfig, error) {
	if len(cfg.Items) > maxAPIOnboardingItems {
		if rejectInvalid {
			return nil, infraerrors.BadRequest("API_ONBOARDING_ITEMS_TOO_MANY", "too many api onboarding items")
		}
		cfg.Items = cfg.Items[:maxAPIOnboardingItems]
	}
	out := APIOnboardingConfig{
		Enabled:  cfg.Enabled,
		Title:    trimAPIOnboardingText(cfg.Title, maxAPIOnboardingTitleLength),
		Subtitle: trimAPIOnboardingText(cfg.Subtitle, maxAPIOnboardingDescLength),
		Items:    []APIOnboardingItem{},
	}
	if out.Title == "" {
		out.Title = defaultAPIOnboardingConfig().Title
	}
	if out.Subtitle == "" {
		out.Subtitle = defaultAPIOnboardingConfig().Subtitle
	}
	if !out.Enabled && !includeDisabled {
		return &out, nil
	}

	seenIDs := map[string]bool{}
	seenTitles := map[string]bool{}
	for i, item := range cfg.Items {
		if !includeDisabled && !item.Enabled {
			continue
		}
		normalized, err := normalizeAPIOnboardingItem(item, i, validPlanIDs, rejectInvalid)
		if err != nil {
			return nil, err
		}
		if normalized == nil {
			continue
		}
		if seenIDs[normalized.ID] {
			if rejectInvalid {
				return nil, infraerrors.BadRequest("API_ONBOARDING_ITEM_ID_DUPLICATE", "api onboarding item id must be unique")
			}
			continue
		}
		if normalized.Enabled {
			titleKey := strings.ToLower(normalized.Title)
			if seenTitles[titleKey] {
				if rejectInvalid {
					return nil, infraerrors.BadRequest("API_ONBOARDING_ITEM_TITLE_DUPLICATE", "api onboarding item title must be unique")
				}
				continue
			}
			seenTitles[titleKey] = true
		}
		seenIDs[normalized.ID] = true
		out.Items = append(out.Items, *normalized)
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		if out.Items[i].SortOrder != out.Items[j].SortOrder {
			return out.Items[i].SortOrder < out.Items[j].SortOrder
		}
		return out.Items[i].ID < out.Items[j].ID
	})
	for i := range out.Items {
		out.Items[i].SortOrder = i + 1
	}
	return &out, nil
}

func normalizeAPIOnboardingItem(item APIOnboardingItem, index int, validPlanIDs map[int64]bool, rejectInvalid bool) (*APIOnboardingItem, error) {
	title := trimAPIOnboardingText(item.Title, maxAPIOnboardingTitleLength)
	if title == "" {
		if rejectInvalid {
			return nil, infraerrors.BadRequest("API_ONBOARDING_ITEM_TITLE_REQUIRED", "api onboarding item title is required")
		}
		return nil, nil
	}
	cta := normalizeAPIOnboardingCTA(item.CTA)
	if cta == "" {
		if rejectInvalid {
			return nil, infraerrors.BadRequest("API_ONBOARDING_ITEM_CTA_INVALID", "api onboarding item cta is invalid")
		}
		return nil, nil
	}
	audience := normalizeAPIOnboardingAudience(item.Audience)
	groupID := normalizeAPIOnboardingPositiveID(item.GroupID)
	planID, planErr := normalizeAPIOnboardingPlanID(item.PlanID, validPlanIDs, rejectInvalid)
	if planErr != nil {
		return nil, planErr
	}
	if cta == "buy_plan" && planID == nil {
		if rejectInvalid {
			return nil, infraerrors.BadRequest("API_ONBOARDING_ITEM_PLAN_REQUIRED", "buy_plan onboarding item requires a valid plan_id")
		}
		return nil, nil
	}
	return &APIOnboardingItem{
		ID:          normalizeStorefrontItemID(item.ID, title, "onboarding", index+1),
		Title:       title,
		Description: trimAPIOnboardingText(item.Description, maxAPIOnboardingDescLength),
		Badge:       trimAPIOnboardingText(item.Badge, maxAPIOnboardingBadgeLength),
		Enabled:     item.Enabled,
		SortOrder:   normalizedSortOrder(item.SortOrder, index),
		GroupID:     groupID,
		PlanID:      planID,
		MinBalance:  normalizeAPIOnboardingMinBalance(item.MinBalance),
		CTA:         cta,
		Audience:    audience,
	}, nil
}

func normalizeAPIOnboardingCTA(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "create_key"
	}
	if allowedAPIOnboardingCTAs[value] {
		return value
	}
	return ""
}

func normalizeAPIOnboardingAudience(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if allowedAPIOnboardingAudiences[value] {
		return value
	}
	return "new_users"
}

func normalizeAPIOnboardingPositiveID(value *int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	v := *value
	return &v
}

func normalizeAPIOnboardingPlanID(value *int64, validPlanIDs map[int64]bool, rejectInvalid bool) (*int64, error) {
	normalized := normalizeAPIOnboardingPositiveID(value)
	if normalized == nil {
		return nil, nil
	}
	if validPlanIDs != nil && !validPlanIDs[*normalized] {
		if rejectInvalid {
			return nil, infraerrors.BadRequest("API_ONBOARDING_ITEM_PLAN_INVALID", "api onboarding item plan_id is invalid")
		}
		return nil, nil
	}
	return normalized, nil
}

func normalizeAPIOnboardingMinBalance(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value > maxAPIOnboardingMinBalance {
		return maxAPIOnboardingMinBalance
	}
	return value
}

func trimAPIOnboardingText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
