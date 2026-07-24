//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIOnboardingConfigPublicViewFiltersDisabledAndInvalidItems(t *testing.T) {
	raw, err := json.Marshal(APIOnboardingConfig{
		Enabled:  true,
		Title:    " 新手接入 ",
		Subtitle: " 按推荐创建 Key ",
		Items: []APIOnboardingItem{
			{ID: "disabled", Title: "Hidden", Enabled: false, CTA: "create_key", SortOrder: 1},
			{ID: "bad-cta", Title: "Bad CTA", Enabled: true, CTA: "launch", SortOrder: 2},
			{ID: "missing-plan", Title: "Missing Plan", Enabled: true, CTA: "buy_plan", SortOrder: 3},
			{ID: "create", Title: "创建 Claude Key", Description: "推荐先走稳定分组", Badge: "推荐", Enabled: true, CTA: "create_key", Audience: "all_users", SortOrder: 4, GroupID: onboardingInt64Ptr(12)},
		},
	})
	require.NoError(t, err)

	cfg := BuildPublicAPIOnboardingConfig(string(raw))

	require.True(t, cfg.Enabled)
	require.Equal(t, "新手接入", cfg.Title)
	require.Equal(t, "按推荐创建 Key", cfg.Subtitle)
	require.Len(t, cfg.Items, 1)
	require.Equal(t, "create", cfg.Items[0].ID)
	require.Equal(t, "all_users", cfg.Items[0].Audience)
	require.EqualValues(t, 12, *cfg.Items[0].GroupID)
}

func TestUpdateAPIOnboardingConfigPersistsNormalizedConfig(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigPlansTestClient(t)
	repo := &paymentStorefrontSettingRepo{values: map[string]string{}}
	svc := NewPaymentConfigService(client, repo, nil)

	plan, err := svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:      9,
		Name:         "Pro Monthly",
		Description:  "Monthly plan",
		Price:        29.9,
		ValidityDays: 30,
		ValidityUnit: "days",
		ForSale:      true,
	})
	require.NoError(t, err)

	cfg, err := svc.UpdateAPIOnboardingConfig(ctx, APIOnboardingConfig{
		Enabled:  true,
		Title:    " API 新手接入 ",
		Subtitle: " 创建 Key 或购买套餐 ",
		Items: []APIOnboardingItem{
			{ID: " Buy Plan ", Title: " 购买月卡 ", Description: " 余额不足时优先购买套餐 ", Badge: "月卡", Enabled: true, CTA: "buy_plan", Audience: "", SortOrder: 2, PlanID: &plan.ID},
			{ID: "", Title: "创建 Key", Enabled: true, CTA: "", SortOrder: 1, GroupID: onboardingInt64Ptr(9), MinBalance: -10},
		},
	})
	require.NoError(t, err)

	require.True(t, cfg.Enabled)
	require.Len(t, cfg.Items, 2)
	require.Equal(t, "创建 Key", cfg.Items[0].Title)
	require.Equal(t, "create_key", cfg.Items[0].CTA)
	require.Equal(t, "new_users", cfg.Items[0].Audience)
	require.Zero(t, cfg.Items[0].MinBalance)
	require.Equal(t, "buy-plan", cfg.Items[1].ID)
	require.Equal(t, "buy_plan", cfg.Items[1].CTA)
	require.Equal(t, plan.ID, *cfg.Items[1].PlanID)

	var saved APIOnboardingConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyAPIOnboardingConfig]), &saved))
	require.Equal(t, cfg.Items, saved.Items)
}

func TestUpdateAPIOnboardingConfigRejectsInvalidPlan(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, &paymentStorefrontSettingRepo{values: map[string]string{}}, nil)

	_, err := svc.UpdateAPIOnboardingConfig(context.Background(), APIOnboardingConfig{
		Enabled: true,
		Items: []APIOnboardingItem{
			{ID: "bad-plan", Title: "购买套餐", Enabled: true, CTA: "buy_plan", PlanID: onboardingInt64Ptr(999)},
		},
	})
	require.Error(t, err)
	require.Equal(t, "API_ONBOARDING_ITEM_PLAN_INVALID", infraErrorReason(err))
}

func TestUpdateAPIOnboardingConfigRejectsDuplicateTitles(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, &paymentStorefrontSettingRepo{values: map[string]string{}}, nil)

	_, err := svc.UpdateAPIOnboardingConfig(context.Background(), APIOnboardingConfig{
		Enabled: true,
		Items: []APIOnboardingItem{
			{ID: "a", Title: "推荐接入", Enabled: true, CTA: "create_key"},
			{ID: "b", Title: "推荐接入", Enabled: true, CTA: "recharge"},
		},
	})
	require.Error(t, err)
	require.Equal(t, "API_ONBOARDING_ITEM_TITLE_DUPLICATE", infraErrorReason(err))
}

func TestUpdateAPIOnboardingConfigAllowsDisabledDuplicateTitles(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, &paymentStorefrontSettingRepo{values: map[string]string{}}, nil)

	cfg, err := svc.UpdateAPIOnboardingConfig(context.Background(), APIOnboardingConfig{
		Enabled: true,
		Items: []APIOnboardingItem{
			{ID: "a", Title: "推荐接入", Enabled: true, CTA: "create_key"},
			{ID: "b", Title: "推荐接入", Enabled: false, CTA: "recharge"},
		},
	})
	require.NoError(t, err)
	require.Len(t, cfg.Items, 2)
}

func TestSettingServiceGetPublicSettingsExposesAPIOnboarding(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyAPIOnboardingConfig: `{
				"enabled": true,
				"title": "创建 API Key",
				"subtitle": "先选择推荐接入方式",
				"items": [
					{"id":"starter","title":"创建稳定 Key","enabled":true,"cta":"create_key","group_id":7,"sort_order":1},
					{"id":"hidden","title":"隐藏项","enabled":false,"cta":"recharge","sort_order":2}
				]
			}`,
		},
	}, nil)

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.APIOnboarding.Enabled)
	require.Equal(t, "创建 API Key", settings.APIOnboarding.Title)
	require.Len(t, settings.APIOnboarding.Items, 1)
	require.Equal(t, "starter", settings.APIOnboarding.Items[0].ID)
	require.EqualValues(t, 7, *settings.APIOnboarding.Items[0].GroupID)
}

func onboardingInt64Ptr(value int64) *int64 {
	return &value
}
