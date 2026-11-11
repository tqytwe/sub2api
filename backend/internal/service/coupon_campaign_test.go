package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCouponClaimPolicyRequiresBoundedOpenWindow(t *testing.T) {
	now := time.Now().UTC()
	valid := CouponClaimPolicy{Enabled: true, StartsAt: &now, EndsAt: ptrCouponTime(now.Add(time.Hour)), ClaimLimit: 100, PerUserLimit: 1}
	require.NoError(t, ValidateCouponClaimPolicy(valid))
	for _, change := range []func(*CouponClaimPolicy){
		func(p *CouponClaimPolicy) { p.ClaimLimit = 0 },
		func(p *CouponClaimPolicy) { p.PerUserLimit = 0 },
		func(p *CouponClaimPolicy) { p.EndsAt = nil },
		func(p *CouponClaimPolicy) { p.EndsAt = p.StartsAt },
		func(p *CouponClaimPolicy) { p.Audience.VIPTier = ptrCouponInt(-1) },
	} {
		p := valid
		change(&p)
		require.Error(t, ValidateCouponClaimPolicy(p))
	}
}
func TestCouponAudienceRequiresExplicitSelectionAndPreservesLimit(t *testing.T) {
	require.Error(t, ValidateCouponAudience(CouponAudience{}))
	require.Error(t, ValidateCouponAudience(CouponAudience{Mode: "all", UserIDs: []int64{1}}))
	require.Error(t, ValidateCouponAudience(CouponAudience{Mode: "users", UserIDs: []int64{-1}}))
	require.NoError(t, ValidateCouponAudience(CouponAudience{Mode: "users", UserIDs: []int64{1, 2}}))
	require.NoError(t, ValidateCouponAudience(CouponAudience{Mode: "filtered", Search: "example"}))
	ids := make([]int64, 10001)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	require.Error(t, ValidateCouponAudience(CouponAudience{Mode: "users", UserIDs: ids}))
}
func ptrCouponTime(v time.Time) *time.Time { return &v }
func ptrCouponInt(v int) *int              { return &v }
