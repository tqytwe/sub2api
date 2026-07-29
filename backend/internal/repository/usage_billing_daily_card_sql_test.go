package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCardEntitlementSettlementUpdateSQLUsesExplicitParameterTypes(t *testing.T) {
	require.Contains(t, dailyCardEntitlementSettlementUpdateSQL, "$4::varchar")
	require.Contains(t, dailyCardEntitlementSettlementUpdateSQL, "$6::timestamptz")
}
