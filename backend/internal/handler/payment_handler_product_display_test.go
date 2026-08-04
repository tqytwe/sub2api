//go:build unit

package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCheckoutPlanJSONIncludesProductDisplayFields(t *testing.T) {
	quotaLimit := 10.0
	durationHours := 24
	body, err := json.Marshal(checkoutPlan{
		ID:                7,
		GroupID:           3,
		Name:              "Pro",
		ProductName:       "GPT Pro Workbench",
		CoverImageURL:     "/assets/plans/pro.webp",
		DetailDescription: "Long copy",
		Price:             19.99,
		ValidityDays:      30,
		ValidityUnit:      "days",
		QuotaMode:         service.DailyCardQuotaModeOneTime,
		QuotaLimitUSD:     &quotaLimit,
		DurationHours:     &durationHours,
		Features:          []string{"Priority models"},
	})
	require.NoError(t, err)

	require.JSONEq(t, `{
		"id": 7,
		"group_id": 3,
		"group_platform": "",
		"group_name": "",
		"rate_multiplier": 0,
		"peak_rate_enabled": false,
		"peak_start": "",
		"peak_end": "",
		"peak_rate_multiplier": 0,
		"daily_limit_usd": null,
		"weekly_limit_usd": null,
		"monthly_limit_usd": null,
		"supported_model_scopes": null,
		"name": "Pro",
		"description": "",
		"price": 19.99,
		"validity_days": 30,
		"validity_unit": "days",
		"quota_mode": "one_time",
		"quota_limit_usd": 10,
		"duration_hours": 24,
		"features": ["Priority models"],
		"product_name": "GPT Pro Workbench",
		"cover_image_url": "/assets/plans/pro.webp",
		"detail_description": "Long copy",
		"storefront_platform": "",
		"storefront_category": "",
		"storefront_featured": false,
		"storefront_badge": ""
	}`, string(body))
}

func TestCheckoutInfoJSONIncludesStorefrontConfig(t *testing.T) {
	defaultPlanID := int64(7)
	body, err := json.Marshal(checkoutInfoResponse{
		Plans: []checkoutPlan{{ID: 7, Name: "Monthly 29.9"}},
		StorefrontConfig: &service.PaymentStorefrontConfig{
			Shelves: []service.PaymentStorefrontShelf{
				{ID: "monthly", Label: "月卡", Enabled: true, SortOrder: 1, PlanIDs: []int64{7}, DefaultPlanID: &defaultPlanID},
			},
			Tags: []service.PaymentStorefrontTag{
				{ID: "best-value", Label: "高性价比", Tone: "success", Enabled: true, SortOrder: 1, PlanIDs: []int64{7}},
			},
		},
	})
	require.NoError(t, err)

	require.JSONEq(t, `{
		"methods": null,
		"global_min": 0,
		"global_max": 0,
		"plans": [{
			"id": 7,
			"group_id": 0,
			"group_platform": "",
			"group_name": "",
			"rate_multiplier": 0,
			"peak_rate_enabled": false,
			"peak_start": "",
			"peak_end": "",
			"peak_rate_multiplier": 0,
			"daily_limit_usd": null,
			"weekly_limit_usd": null,
			"monthly_limit_usd": null,
			"supported_model_scopes": null,
			"name": "Monthly 29.9",
			"description": "",
			"price": 0,
			"validity_days": 0,
			"validity_unit": "",
			"quota_mode": "",
			"quota_limit_usd": null,
			"duration_hours": null,
			"features": null,
			"product_name": "",
			"cover_image_url": "",
			"detail_description": "",
			"storefront_platform": "",
			"storefront_category": "",
			"storefront_featured": false,
			"storefront_badge": ""
		}],
		"balance_disabled": false,
		"balance_recharge_multiplier": 0,
		"subscription_usd_to_cny_rate": 0,
		"recharge_fee_rate": 0,
		"storefront_config": {
			"shelves": [{
				"id": "monthly",
				"label": "月卡",
				"enabled": true,
				"sort_order": 1,
				"plan_ids": [7],
				"default_plan_id": 7
			}],
			"tags": [{
				"id": "best-value",
				"label": "高性价比",
				"tone": "success",
				"enabled": true,
				"sort_order": 1,
				"plan_ids": [7]
			}]
		},
		"help_text": "",
		"help_image_url": "",
		"stripe_publishable_key": "",
		"alipay_force_qrcode": false,
		"alipay_mobile_precreate_deep_link": false
	}`, string(body))
}

func TestBuildDailyCardResultsShowsRemainingQuotaAndQueuePosition(t *testing.T) {
	now := time.Date(2026, 7, 28, 20, 0, 0, 0, time.UTC)
	expiresAt := now.Add(22 * time.Hour)
	cards := []service.DailyCardEntitlement{
		{ID: 1, GroupID: 7, Status: service.DailyCardStatusActive, QuotaLimitUSD: 10, QuotaUsedUSD: 9, ExpiresAt: &expiresAt},
		{ID: 2, GroupID: 7, Status: service.DailyCardStatusPending, QuotaLimitUSD: 10},
	}

	got := buildDailyCardResults(cards, now)

	require.Len(t, got, 2)
	require.Equal(t, 1.0, got[0].RemainingQuotaUSD)
	require.Zero(t, got[0].QuotaReservedUSD)
	require.Equal(t, int64(22*time.Hour/time.Second), got[0].RemainingSeconds)
	require.Zero(t, got[0].QueuePosition)
	require.Equal(t, 1, got[1].QueuePosition)
}
