package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminWithdrawalRecomputeAnomalyFromRawUsesStableCodes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		code    string
		details map[string]string
	}{
		{
			name: "missing balance before",
			raw:  "transaction 201 missing reliable balance_before",
			code: "missing_balance_before",
			details: map[string]string{
				"transaction_id": "201",
			},
		},
		{
			name: "ledger replay mismatch",
			raw:  "ledger replay balance 8.00000000 does not match users.balance 9.00000000",
			code: "replay_balance_mismatch",
			details: map[string]string{
				"ledger_balance": "8.00000000",
				"user_balance":   "9.00000000",
			},
		},
		{
			name: "legacy existing entitlement guard",
			raw:  "existing withdrawable entitlements require manual review before execute",
			code: "existing_entitlements",
		},
		{
			name: "existing entitlement mismatch",
			raw:  "existing withdrawable entitlements do not match recompute report: existing_count=2 recompute_count=1 existing_total=1.00000000 recompute_total=0.50000000",
			code: "existing_entitlements_mismatch",
			details: map[string]string{
				"existing_count":  "2",
				"recompute_count": "1",
				"existing_total":  "1.00000000",
				"recompute_total": "0.50000000",
			},
		},
		{
			name:    "unknown raw anomaly",
			raw:     "unexpected replay failure detail",
			code:    "unknown",
			details: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			anomaly := adminWithdrawalRecomputeAnomalyFromRaw(tc.raw)
			require.Equal(t, tc.code, anomaly.Code)
			require.Equal(t, tc.details, anomaly.Details)
		})
	}
}
