package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBindReferralAttributionCastsRegisteredAtBeforeIntervalArithmetic(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	sql := strings.ToLower(string(source))

	require.Contains(t, sql, "u.created_at >= $4::timestamptz - interval '24 hours'")
	require.NotContains(t, sql, "u.created_at >= $4 - interval '24 hours'")
}

func TestRevokeReferralRewardsLocksOnlyRewardRows(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	sql := strings.ToLower(string(source))

	require.Contains(t, sql, "order by r.id for update of r")
}
