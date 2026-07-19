//go:build unit

package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckoutPlanJSONIncludesProductDisplayFields(t *testing.T) {
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
		"features": ["Priority models"],
		"product_name": "GPT Pro Workbench",
		"cover_image_url": "/assets/plans/pro.webp",
		"detail_description": "Long copy"
	}`, string(body))
}
