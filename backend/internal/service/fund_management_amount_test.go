package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFundCreditAmountAllowsPositiveLedgerPrecision(t *testing.T) {
	for _, raw := range []string{"0.5", "30", "30.00", "0.00000001"} {
		amount, err := parseFundCreditAmount(raw)
		require.NoError(t, err, raw)
		require.True(t, amount.IsPositive(), raw)
	}

	for _, raw := range []string{"", "0", "-0.5", "not-a-number", "0.000000001"} {
		_, err := parseFundCreditAmount(raw)
		require.ErrorIs(t, err, ErrFundInvalidAmount, raw)
	}
}
