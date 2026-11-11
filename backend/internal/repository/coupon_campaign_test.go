package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCouponAudienceQueryUsesBoundParametersAndActiveUsers(t *testing.T) {
	tier := 2
	q, args := couponAudienceQuery(service.CouponAudience{Mode: "filtered", Search: "x' OR TRUE", GroupName: "vip", APIKeyGroupID: 9, VIPTier: &tier, Attributes: map[int64]string{3: "abc"}}, 77)
	require.Contains(t, q, "u.deleted_at IS NULL")
	require.Contains(t, q, "u.status = 'active'")
	require.Contains(t, q, "u.role = 'user'")
	require.NotContains(t, q, "x' OR TRUE")
	require.Contains(t, q, "user_attribute_values")
	require.Contains(t, q, "play_membership_verified_contributions")
	require.Contains(t, q, "api_keys")
	require.Contains(t, args, int64(77))
	require.Contains(t, args, "%x' OR TRUE%")
}
