package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCouponRewardPoolReleaseGateMigrationDisablesBothActivities(t *testing.T) {
	content, err := FS.ReadFile("225_coupon_reward_pool_release_gate.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))
	require.Contains(t, sql, "INSERT INTO SETTINGS (KEY, VALUE, UPDATED_AT)")
	require.Contains(t, sql, "('PLAY_BLINDBOX_ENABLED', 'FALSE', NOW())")
	require.Contains(t, sql, "('PLAY_QUIZ_ENABLED', 'FALSE', NOW())")
	require.Contains(t, sql, "ON CONFLICT (KEY) DO UPDATE")
	require.Contains(t, sql, "SET VALUE = EXCLUDED.VALUE")
	require.Contains(t, sql, "UPDATED_AT = EXCLUDED.UPDATED_AT")
	require.NotContains(t, sql, "COUPON_TEMPLATES")
	require.NotContains(t, sql, "COUPON_REWARD_POOL_VERSIONS")
}
