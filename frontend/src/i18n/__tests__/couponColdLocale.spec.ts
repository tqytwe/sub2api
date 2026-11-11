import { describe, expect, it, vi } from 'vitest'

const expected = {
  zh: { available: '可用', locked: '订单处理中', used: '已使用', expired: '已失效', voided: '已作废' },
  en: { available: 'Available', locked: 'Order pending', used: 'Used', expired: 'Expired', voided: 'Voided' },
}

describe('coupon status labels on a cold user route', () => {
  it.each([
    ['zh', '/wallet'], ['en', '/wallet'],
    ['zh', '/purchase'], ['en', '/purchase'],
  ] as const)('loads all %s coupon statuses for %s without administrator messages', async (locale, path) => {
    vi.resetModules()
    const { ensureLocaleMessagesForPath, i18n } = await import('../index')
    await ensureLocaleMessagesForPath(path, locale)
    i18n.global.locale.value = locale
    expect(i18n.global.te('coupon.admin.templatesTitle')).toBe(false)
    for (const [status, label] of Object.entries(expected[locale])) {
      expect(i18n.global.te(`coupon.wallet.status.${status}`)).toBe(true)
      expect(i18n.global.tm(`coupon.wallet.status.${status}`)).toBe(label)
    }
    expect(i18n.global.te('coupon.wallet.status.awaiting_reconciliation')).toBe(false)
  })
})
