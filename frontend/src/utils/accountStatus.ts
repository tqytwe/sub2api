/**
 * Keep backend-provided status values from becoming arbitrary vue-i18n keys.
 * A newer backend status must render the localized unknown-state label until
 * both locale fragments deliberately support it.
 */
const ACCOUNT_STATUS_KEYS = {
  active: 'admin.accounts.status.active',
  inactive: 'admin.accounts.status.inactive',
  expired: 'admin.accounts.status.expired',
  error: 'admin.accounts.status.error',
  cooldown: 'admin.accounts.status.cooldown',
  paused: 'admin.accounts.status.paused',
  limited: 'admin.accounts.status.limited',
  rate_limited: 'admin.accounts.status.rateLimited',
  overloaded: 'admin.accounts.status.overloaded',
  temp_unschedulable: 'admin.accounts.status.tempUnschedulable',
  quota_exceeded: 'admin.accounts.status.quotaExceeded',
  unschedulable: 'admin.accounts.status.unschedulable',
} as const

export type AccountStatusTranslationKey =
  | typeof ACCOUNT_STATUS_KEYS[keyof typeof ACCOUNT_STATUS_KEYS]
  | 'status.unknown'

export function accountStatusTranslationKey(status: unknown): AccountStatusTranslationKey {
  if (typeof status !== 'string') return 'status.unknown'
  return ACCOUNT_STATUS_KEYS[status as keyof typeof ACCOUNT_STATUS_KEYS] ?? 'status.unknown'
}
