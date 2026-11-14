import { describe, expect, it, vi } from 'vitest'

describe('Key calendar consumption on a cold route', () => {
  it.each(['zh', 'en'] as const)('loads %s labels without opening another route', async locale => {
    vi.resetModules()
    const { ensureLocaleMessagesForPath, i18n } = await import('../index')
    await ensureLocaleMessagesForPath('/keys', locale)
    i18n.global.locale.value = locale
    for (const key of ['usageDateRange', 'selectedPeriod', 'invalidUsageDateRange', 'failedToLoadUsage']) {
      expect(i18n.global.te(`keys.${key}`, locale)).toBe(true)
      expect(i18n.global.tm(`keys.${key}`)).toEqual(expect.any(String))
      expect(i18n.global.tm(`keys.${key}`)).not.toBe(`keys.${key}`)
    }
  })
})
