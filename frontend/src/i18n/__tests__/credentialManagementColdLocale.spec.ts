import { describe, expect, it, vi } from 'vitest'

describe('credential management cold routes', () => {
  it.each(['zh', 'en'] as const)('loads SIWC and credential management messages in %s', async locale => {
    vi.resetModules()
    const { ensureLocaleMessagesForRoute, i18n } = await import('../index')
    await ensureLocaleMessagesForRoute('AdminAccounts', locale)
    i18n.global.locale.value = locale
    expect(i18n.global.te('siwc.protocol')).toBe(true)
    expect(i18n.global.te('siwc.failed')).toBe(true)
    await ensureLocaleMessagesForRoute('AdminTokenGuardV2', locale)
    expect(i18n.global.te('tokenGuardV2.encryption.title')).toBe(true)
    expect(i18n.global.te('tokenGuardV2.totpManage.title')).toBe(true)
  })
})
