package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHasOneTimeMonthlyQuota 验证一次性月额度的识别逻辑
func TestHasOneTimeMonthlyQuota(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)

	tests := []struct {
		name        string
		expiresAt   time.Time
		wantOneTime bool
		description string
	}{
		{
			name:        "30天订阅为一次性月额度",
			expiresAt:   startsAt.AddDate(0, 0, 30),
			wantOneTime: true,
			description: "正好30天的订阅应视为一次性月额度",
		},
		{
			name:        "60天订阅不是一次性月额度",
			expiresAt:   startsAt.AddDate(0, 0, 60),
			wantOneTime: false,
			description: "超过30天的订阅应有月度重置",
		},
		{
			name:        "7天订阅是一次性月额度",
			expiresAt:   startsAt.AddDate(0, 0, 7),
			wantOneTime: true,
			description: "少于30天的订阅也视为一次性",
		},
		{
			name:        "31天订阅不是一次性月额度",
			expiresAt:   startsAt.AddDate(0, 0, 31),
			wantOneTime: false,
			description: "刚好超过30天应有重置",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &UserSubscription{
				StartsAt:  startsAt,
				ExpiresAt: tt.expiresAt,
			}
			got := sub.HasOneTimeMonthlyQuota()
			assert.Equal(t, tt.wantOneTime, got, tt.description)
		})
	}
}

// TestMonthlyResetTime_OneTimeQuota 验证一次性月额度的重置时间等于过期时间
func TestMonthlyResetTime_OneTimeQuota(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)
	expiresAt := startsAt.AddDate(0, 0, 30) // 30天一次性套餐
	monthlyWindowStart := startsAt

	sub := &UserSubscription{
		StartsAt:           startsAt,
		ExpiresAt:          expiresAt,
		MonthlyWindowStart: &monthlyWindowStart,
	}

	resetTime := sub.MonthlyResetTime()
	require.NotNil(t, resetTime)
	assert.True(t, expiresAt.Equal(*resetTime),
		"一次性月额度的重置时间应等于过期时间，而不是 window_start+30天")
}

// TestMonthlyResetTime_RecurringQuota 验证多周期订阅的重置时间按窗口计算
func TestMonthlyResetTime_RecurringQuota(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)
	expiresAt := startsAt.AddDate(0, 0, 60) // 60天订阅，有两个月周期
	monthlyWindowStart := startsAt

	sub := &UserSubscription{
		StartsAt:           startsAt,
		ExpiresAt:          expiresAt,
		MonthlyWindowStart: &monthlyWindowStart,
	}

	resetTime := sub.MonthlyResetTime()
	require.NotNil(t, resetTime)
	expectedResetTime := monthlyWindowStart.Add(30 * 24 * time.Hour)
	assert.True(t, expectedResetTime.Equal(*resetTime),
		"多周期订阅的重置时间应按窗口起点+30天计算")
	assert.False(t, expiresAt.Equal(*resetTime),
		"多周期订阅的重置时间不应等于过期时间")
}

// TestNeedsMonthlyReset_OneTimeQuota 验证一次性月额度不会自动重置
func TestNeedsMonthlyReset_OneTimeQuota(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)
	expiresAt := startsAt.AddDate(0, 0, 30)
	monthlyWindowStart := startsAt

	sub := &UserSubscription{
		StartsAt:           startsAt,
		ExpiresAt:          expiresAt,
		MonthlyWindowStart: &monthlyWindowStart,
	}

	// 即使过了30天，一次性额度的 NeedsMonthlyResetAt 会返回 true（时间到了）
	now := monthlyWindowStart.Add(31 * 24 * time.Hour)
	needsReset := sub.NeedsMonthlyResetAt(now)
	assert.True(t, needsReset,
		"NeedsMonthlyResetAt 纯粹判断时间，一次性套餐过期后也返回 true")

	// 但是 canAutomaticallyResetMonthlyAt 返回 false（不允许自动重置）
	canReset := sub.canAutomaticallyResetMonthlyAt(now)
	assert.False(t, canReset,
		"一次性月额度不应在窗口过期后触发自动重置（会导致重复发放额度）")
}

// TestHasOneTimeWeeklyQuota 验证一次性周额度的识别逻辑
func TestHasOneTimeWeeklyQuota(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)

	tests := []struct {
		name        string
		expiresAt   time.Time
		wantOneTime bool
	}{
		{
			name:        "7天订阅为一次性周额度",
			expiresAt:   startsAt.AddDate(0, 0, 7),
			wantOneTime: true,
		},
		{
			name:        "14天订阅不是一次性周额度",
			expiresAt:   startsAt.AddDate(0, 0, 14),
			wantOneTime: false,
		},
		{
			name:        "3天订阅是一次性周额度",
			expiresAt:   startsAt.AddDate(0, 0, 3),
			wantOneTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &UserSubscription{
				StartsAt:  startsAt,
				ExpiresAt: tt.expiresAt,
			}
			got := sub.HasOneTimeWeeklyQuota()
			assert.Equal(t, tt.wantOneTime, got)
		})
	}
}

// TestWeeklyResetTime_OneTimeQuota 验证一次性周额度的重置时间等于过期时间
func TestWeeklyResetTime_OneTimeQuota(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)
	expiresAt := startsAt.AddDate(0, 0, 7) // 7天一次性套餐
	weeklyWindowStart := startsAt

	sub := &UserSubscription{
		StartsAt:          startsAt,
		ExpiresAt:         expiresAt,
		WeeklyWindowStart: &weeklyWindowStart,
	}

	resetTime := sub.WeeklyResetTime()
	require.NotNil(t, resetTime)
	assert.True(t, expiresAt.Equal(*resetTime),
		"一次性周额度的重置时间应等于过期时间")
}

// Test issue #5051 regression: 一次性套餐不应在期限内重复发放额度
func TestOneTimeQuota_NoResetBeforeExpiry(t *testing.T) {
	startsAt := time.Date(2026, 9, 11, 8, 43, 44, 0, time.UTC)
	expiresAt := startsAt.AddDate(0, 0, 30) // 30天一次性套餐
	monthlyWindowStart := startsAt

	sub := &UserSubscription{
		StartsAt:           startsAt,
		ExpiresAt:          expiresAt,
		MonthlyWindowStart: &monthlyWindowStart,
		MonthlyUsageUSD:    1500.0, // 已使用额度
	}

	// 第29天，窗口即将过期但订阅未过期
	now := monthlyWindowStart.Add(29 * 24 * time.Hour)
	needsReset := sub.NeedsMonthlyResetAt(now)
	assert.False(t, needsReset,
		"时间未到窗口边界，NeedsMonthlyResetAt 返回 false")

	canReset := sub.canAutomaticallyResetMonthlyAt(now)
	assert.False(t, canReset,
		"一次性套餐不应在到期前重置，否则用户获得双倍额度（issue #5051）")

	// 第31天，订阅已过期
	nowAfterExpiry := expiresAt.Add(24 * time.Hour)
	needsResetAfterExpiry := sub.NeedsMonthlyResetAt(nowAfterExpiry)
	assert.True(t, needsResetAfterExpiry,
		"时间到达窗口边界后，NeedsMonthlyResetAt 返回 true")

	canResetAfterExpiry := sub.canAutomaticallyResetMonthlyAt(nowAfterExpiry)
	assert.False(t, canResetAfterExpiry,
		"一次性套餐在过期后也不应自动重置（已不可用）")
}
