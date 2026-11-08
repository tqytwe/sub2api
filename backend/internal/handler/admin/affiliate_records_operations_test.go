package admin

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAffiliateCSVDoesNotExecuteUserFormulas(t *testing.T) {
	for _, input := range []string{"=HYPERLINK(\"https://example.com\")", " +cmd", "\t@SUM(A1:A2)", "-formula"} {
		require.Equal(t, "'"+input, affiliateCSVCell(input))
	}
	require.Equal(t, "-2.5", affiliateCSVCell(-2.5))
	require.Equal(t, "ordinary@example.com", affiliateCSVCell("ordinary@example.com"))
	require.Empty(t, affiliateCSVCell(nil))
	require.Equal(t, "9007199254740993", affiliateCSVCell(json.Number("9007199254740993")))
}
