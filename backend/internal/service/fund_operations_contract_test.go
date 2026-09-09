package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestFundOperationRecordJSONDoesNotExposeInternalIdentifiers(t *testing.T) {
	record := FundOperationRecord{
		OperationNo:       "FM-20260908-000001",
		OperationKind:     FundOperationKindOfflineRecharge,
		Status:            FundOperationStatusCompleted,
		AccountEmail:      "member@example.com",
		ActorAccountEmail: "admin@example.com",
		Amount:            decimal.RequireFromString("6.75600000"),
		Currency:          "USD",
		CreatedAt:         time.Date(2026, time.September, 8, 9, 13, 49, 0, time.UTC),
	}

	payload, err := json.Marshal(record)
	require.NoError(t, err)
	jsonText := string(payload)
	require.NotContains(t, jsonText, "user_id")
	require.NotContains(t, jsonText, "actor_user_id")
	require.NotContains(t, jsonText, "balance_transaction_id")
	require.True(t, strings.Contains(jsonText, "operation_no"))
	require.True(t, strings.Contains(jsonText, "account_email"))
}

func TestMaskFundOperationReference(t *testing.T) {
	require.Equal(t, "wire-***-1001", maskFundOperationReference("wire-transfer-1001"))
	require.Equal(t, "***", maskFundOperationReference("abc"))
	require.Equal(t, "", maskFundOperationReference(""))
}
