package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLocalizedPaymentPlanCopiesDoNotMutateOrExposeTranslations(t *testing.T) {
	translations := service.GroupI18n{"en": {Name: "Standard"}, "zh": {Name: "标准"}}
	plans := []paymentPlanResult{{GroupName: "标准", I18n: translations}}
	checkout := []checkoutPlan{{GroupName: "标准", I18n: translations}}

	englishPlans := localizePaymentPlanResults(plans, "en")
	englishCheckout := localizeCheckoutPlans(checkout, "en")
	require.Equal(t, "Standard", englishPlans[0].GroupName)
	require.Equal(t, "Standard", englishCheckout[0].GroupName)
	require.Equal(t, "标准", plans[0].GroupName)
	require.Equal(t, "标准", checkout[0].GroupName)

	chinesePlans := localizePaymentPlanResults(plans, "zh")
	chineseCheckout := localizeCheckoutPlans(checkout, "zh")
	require.Equal(t, "标准", chinesePlans[0].GroupName)
	require.Equal(t, "标准", chineseCheckout[0].GroupName)

	for _, value := range []any{englishPlans[0], englishCheckout[0]} {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(data), `"i18n"`)
	}
}

func TestLocalizedPaymentPlanResponsesPreserveEmptyArrays(t *testing.T) {
	require.NotNil(t, localizePaymentPlanResults([]paymentPlanResult{}, "en"))
	require.NotNil(t, localizeCheckoutPlans([]checkoutPlan{}, "en"))
}
