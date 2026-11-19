import { describe, expect, it, vi } from 'vitest'

describe('OAuth automation cold quality operations locale', () => {
  it.each(['zh', 'en'] as const)('loads %s controls and all event kinds without visiting another page', async locale => {
    vi.resetModules()
    const { ensureLocaleMessagesForPath, i18n } = await import('../index')
    await ensureLocaleMessagesForPath('/admin/account-quality', locale)
    i18n.global.locale.value = locale
    for (const key of ['title', 'description', 'ws.accelerationLabel', 'accountPicker.search', 'quality.active', 'logs.kinds.apikey_fallback_opened', 'logs.kinds.manual_recovery', 'apikeyFallback.event']) {
      expect(i18n.global.te(`autoConfig.${key}`, locale), key).toBe(true)
      expect(i18n.global.tm(`autoConfig.${key}`), key).toEqual(expect.any(String))
    }
  })
})
