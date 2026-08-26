import { describe, expect, it } from 'vitest'
import { accountStatusTranslationKey } from '../accountStatus'

describe('accountStatusTranslationKey', () => {
  it.each([
    ['active', 'admin.accounts.status.active'],
    ['inactive', 'admin.accounts.status.inactive'],
    ['expired', 'admin.accounts.status.expired'],
    ['error', 'admin.accounts.status.error'],
    ['cooldown', 'admin.accounts.status.cooldown'],
    ['paused', 'admin.accounts.status.paused'],
    ['limited', 'admin.accounts.status.limited'],
    ['rate_limited', 'admin.accounts.status.rateLimited'],
    ['overloaded', 'admin.accounts.status.overloaded'],
    ['temp_unschedulable', 'admin.accounts.status.tempUnschedulable'],
    ['quota_exceeded', 'admin.accounts.status.quotaExceeded'],
    ['unschedulable', 'admin.accounts.status.unschedulable'],
  ])('maps known %s statuses to an explicit locale key', (status, expected) => {
    expect(accountStatusTranslationKey(status)).toBe(expected)
  })

  it.each([undefined, null, '', 'future_backend_state', 'admin.accounts.status.active'])(
    'does not expose an unrecognized status as an i18n key: %s',
    (status) => {
      expect(accountStatusTranslationKey(status)).toBe('status.unknown')
    },
  )
})
