//go:build unit

package handler

import (
	"encoding/json"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProviderFeeRequestsKeepStripeSubtype(t *testing.T) {
	for _, request := range []any{&CreateOrderRequest{}, &QuoteCouponPaymentRequest{}} {
		require.NoError(t, json.Unmarshal([]byte(`{"payment_type":"stripe","payment_subtype":"link","amount":100,"coupon_id":41}`), request))
		raw, err := json.Marshal(request)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		require.Equal(t, "link", payload["payment_subtype"])
	}
}

func TestProviderFeeCheckoutCloneOwnsSubMethods(t *testing.T) {
	original := map[string]service.MethodLimits{"stripe": {
		SubMethods: map[string]service.MethodFeeRates{"card": {FeeRate: 2, FeeRateMin: 2, FeeRateMax: 2}},
	}}
	cloned := cloneMethodLimitsMap(original)
	cloned["stripe"].SubMethods["card"] = service.MethodFeeRates{FeeRate: 77}
	delete(cloned["stripe"].SubMethods, "card")
	require.Equal(t, service.MethodFeeRates{FeeRate: 2, FeeRateMin: 2, FeeRateMax: 2}, original["stripe"].SubMethods["card"])
}

func TestProviderFeeOrderResponsesUseFrozenSubtype(t *testing.T) {
	order := &dbent.PaymentOrder{ID: 1, ProviderSnapshot: map[string]any{"payment_subtype": "link"}}
	for _, response := range []any{buildPublicOrderResult(order), sanitizePaymentOrderForResponse(order)} {
		raw, err := json.Marshal(response)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		require.Equal(t, "link", payload["payment_subtype"])
	}
}
