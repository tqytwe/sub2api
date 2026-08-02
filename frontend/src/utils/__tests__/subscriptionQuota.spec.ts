import { describe, expect, it } from 'vitest'
import type { UserSubscription } from '@/types'
import { subscriptionDailyLimit, subscriptionDailyUsage } from '../subscriptionQuota'
import { getExpirationDateRelation, getRemainingExpiryDuration } from '../subscriptionQuota'

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

describe('subscription expiry timing', () => {
  it('uses local calendar dates for today and tomorrow', () => {
    const now = new Date(2026, 2, 7, 23, 30)

    expect(getExpirationDateRelation(new Date(2026, 2, 7, 23, 45), now)).toBe('today')
    expect(getExpirationDateRelation(new Date(2026, 2, 8, 3, 30), now)).toBe('tomorrow')
  })

  it('treats the exact expiry instant and elapsed expiries as expired', () => {
    const now = new Date(2026, 6, 30, 9, 0)

    expect(getExpirationDateRelation(now, now)).toBe('expired')
    expect(getRemainingExpiryDuration(now, now)).toBeNull()
    expect(getExpirationDateRelation(new Date(2026, 6, 30, 8, 59), now)).toBe('expired')
    expect(getRemainingExpiryDuration(new Date(2026, 6, 30, 8, 59), now)).toBeNull()
  })

  it('rejects invalid target and current dates', () => {
    const invalid = new Date('invalid')
    const valid = new Date(2026, 6, 30, 9, 0)

    expect(getExpirationDateRelation(invalid, valid)).toBeNull()
    expect(getExpirationDateRelation(valid, invalid)).toBeNull()
    expect(getRemainingExpiryDuration(invalid, valid)).toBeNull()
    expect(getRemainingExpiryDuration(valid, invalid)).toBeNull()
  })

  it('returns rounded-up hours and minutes for an expiry under 24 hours away', () => {
    const now = new Date(2026, 6, 30, 9, 0)

    expect(getRemainingExpiryDuration(new Date(2026, 6, 31, 8, 30), now)).toEqual({
      unit: 'hoursMinutes',
      hours: 23,
      minutes: 30
    })
    expect(getRemainingExpiryDuration(new Date(now.getTime() + 1), now)).toEqual({
      unit: 'hoursMinutes',
      hours: 0,
      minutes: 1
    })
    expect(getRemainingExpiryDuration(new Date(now.getTime() + 23 * 60 * 60 * 1000 + 1), now)).toEqual({
      unit: 'hoursMinutes',
      hours: 23,
      minutes: 1
    })
  })

  it('preserves rounded-up day display from 24 hours onward', () => {
    const now = new Date(2026, 6, 30, 9, 0)

    expect(getRemainingExpiryDuration(new Date(now.getTime() + 24 * 60 * 60 * 1000), now)).toEqual({
      unit: 'days',
      days: 1
    })
    expect(getRemainingExpiryDuration(new Date(now.getTime() + 24 * 60 * 60 * 1000 + 1), now)).toEqual({
      unit: 'days',
      days: 2
    })
  })
})
