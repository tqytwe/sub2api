import { describe, expect, it, vi } from 'vitest'

describe('authorization connections on a cold administrator accounts route', () => {
  it.each(['zh', 'en'] as const)('loads %s connection controls without another route', async locale => {
    vi.resetModules()
    const { ensureLocaleMessagesForPath, i18n } = await import('../index')
    await ensureLocaleMessagesForPath('/admin/accounts', locale)
    i18n.global.locale.value = locale
    for (const key of ['title', 'hint', 'useLegacy', 'authorize', 'refreshUncertain', 'revocationUnconfirmed', 'status.pending']) {
      expect(i18n.global.te(`openaiAuth.${key}`, locale)).toBe(true)
      expect(i18n.global.tm(`openaiAuth.${key}`)).toEqual(expect.any(String))
    }
  })
})
