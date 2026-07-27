package handler

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPlayCouponRewardDTOReturnsCouponTermsAndExpiry(t *testing.T) {
	validFrom := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	expiresAt := validFrom.Add(72 * time.Hour)
	input := &service.PlayCouponRewardSummary{
		UserCouponID:       701,
		TemplateID:         601,
		Name:               "充值满10减1",
		BenefitType:        service.CouponBenefitTypeFixedAmount,
		BenefitValue:       1,
		Currency:           "CNY",
		ApplicableScopes:   []service.CouponScope{service.CouponScopeBalance},
		MinimumOrderAmount: 10,
		ValidFrom:          validFrom,
		ExpiresAt:          expiresAt,
	}

	dto := toPlayCouponRewardDTO(input)
	require.NotNil(t, dto)
	require.Equal(t, int64(701), dto.UserCouponID)
	require.Equal(t, "充值满10减1", dto.Name)
	require.Equal(t, service.CouponBenefitTypeFixedAmount, dto.BenefitType)
	require.Equal(t, "2026-07-27T12:00:00Z", dto.ValidFrom)
	require.Equal(t, "2026-07-30T12:00:00Z", dto.ExpiresAt)

	input.ApplicableScopes[0] = service.CouponScopeSubscription
	require.Equal(t, []service.CouponScope{service.CouponScopeBalance}, dto.ApplicableScopes)
	require.Nil(t, toPlayCouponRewardDTO(nil))
}
