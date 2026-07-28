import { describe, expect, it } from 'vitest'
import type { UserSubscription } from '@/types'
import { subscriptionDailyLimit, subscriptionDailyUsage } from '../subscriptionQuota'

const subscription = (overrides: Partial<UserSubscription> = {}): UserSubscription => ({
  id: 1,
  user_id: 2,
  group_id: 3,
  starts_at: '2026-07-28T18:00:00Z',
  expires_at: '2026-07-29T18:00:00Z',
  status: 'exhausted',
  daily_window_start: null,
  weekly_window_start: null,
  monthly_window_start: null,
  daily_usage_usd: 17.28,
  weekly_usage_usd: 17.28,
  monthly_usage_usd: 17.28,
  assigned_at: '2026-07-28T18:00:00Z',
  notes: '',
  created_at: '2026-07-28T18:00:00Z',
  updated_at: '2026-07-29T01:11:00Z',
  group: { id: 3, daily_limit_usd: 16 } as UserSubscription['group'],
  ...overrides
})

describe('subscription quota display', () => {
  it('uses immutable daily-card quota instead of legacy parent usage', () => {
    const row = subscription({
      daily_card: {
        id: 9,
        payment_order_id: 10,
        status: 'exhausted',
        quota_limit_usd: 16,
        quota_used_usd: 16,
        quota_reserved_usd: 0,
        remaining_quota_usd: 0,
        duration_hours: 24,
        starts_at: '2026-07-28T18:00:00Z',
        expires_at: '2026-07-29T18:00:00Z',
        exhausted_at: '2026-07-29T01:11:00Z',
        ended_at: '2026-07-29T01:11:00Z'
      }
    })

    expect(subscriptionDailyUsage(row)).toBe(16)
    expect(subscriptionDailyLimit(row)).toBe(16)
  })

  it('keeps legacy recurring subscription values', () => {
    const row = subscription({ status: 'active', daily_card: null, daily_usage_usd: 5 })
    expect(subscriptionDailyUsage(row)).toBe(5)
    expect(subscriptionDailyLimit(row)).toBe(16)
  })
})
