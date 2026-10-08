import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest'
const tabs = [
  'general',
  'agreement',
  'features',
  'security',
  'users',
  'gateway',
  'payment',
  'email',
  'backup'
]
// Match production JIT only for these real translation tests, without changing
// unrelated suites that intentionally assert uncompiled fallback keys.
beforeEach(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
afterEach(() => vi.unstubAllGlobals())

describe('production admin tab diagnosis', () => {
  it('cold Chinese system settings tabs are Chinese', async () => {
    vi.resetModules()
    const m = await import('../index')
    await m.ensureLocaleMessagesForRoute('AdminSettings', 'zh')
    m.i18n.global.locale.value = 'zh'
    const actual = Object.fromEntries(
      tabs.map((k) => [k, m.i18n.global.t(`admin.settings.tabs.${k}`)])
    )
    console.log('COLD_SETTINGS_ZH', JSON.stringify(actual))
    for (const v of Object.values(actual)) expect(v).toMatch(/[\u4e00-\u9fff]/)
  })
  it('English to Chinese navigation preserves Chinese settings tabs', async () => {
    vi.resetModules()
    const m = await import('../index')
    await m.ensureLocaleMessagesForRoute('AdminSettings', 'en')
    m.i18n.global.locale.value = 'en'
    await m.ensureLocaleMessagesForRoute('AdminPlayOps', 'zh')
    await m.ensureLocaleMessagesForRoute('AdminSettings', 'zh')
    m.i18n.global.locale.value = 'zh'
    for (const k of tabs)
      expect(m.i18n.global.t(`admin.settings.tabs.${k}`)).toMatch(
        /[\u4e00-\u9fff]/
      )
  })
  for (const locale of ['zh', 'en'] as const)
    it(`checkin tab has runtime translation ${locale}`, async () => {
      vi.resetModules()
      const m = await import('../index')
      await m.ensureLocaleMessagesForRoute('AdminPlayOps', locale)
      m.i18n.global.locale.value = locale
      const key = 'admin.playOps.tabs.checkin'
      console.log('CHECKIN_RUNTIME', locale, m.i18n.global.t(key))
      expect(m.i18n.global.te(key, locale)).toBe(true)
    })
  it('other Chinese scopes do not overwrite settings tab labels', async () => {
    vi.resetModules()
    const m = await import('../index')
    const { LOCALE_LOAD_SCOPES } = await import('../routeScopes')
    await m.ensureLocaleMessagesForRoute('AdminSettings', 'zh')
    m.i18n.global.locale.value = 'zh'
    for (const scope of LOCALE_LOAD_SCOPES) {
      await m.ensureLocaleMessagesForPath('/admin/settings', 'zh', [scope])
      for (const k of tabs)
        expect(
          m.i18n.global.t(`admin.settings.tabs.${k}`),
          scope + ':' + k
        ).toMatch(/[\u4e00-\u9fff]/)
    }
  })

  for (const locale of ['zh', 'en'] as const)
    it(`all dynamic PlayOps tabs and video units resolve in ${locale}`, async () => {
      vi.resetModules()
      const m = await import('../index')
      await m.ensureLocaleMessagesForRoute('AdminPlayOps', locale)
      await m.ensureLocaleMessagesForRoute('Models', locale)
      m.i18n.global.locale.value = locale
      const source = readFileSync(
        resolve(process.cwd(), 'src/views/admin/PlayOpsView.vue'),
        'utf8'
      )
      const block = source.match(
        /const tabKeys: PlayOpsTab\[\] = \[([\s\S]*?)\];/
      )![1]
      const keys = [...block.matchAll(/"([a-z-]+)"/g)].map(
        (match) => `admin.playOps.tabs.${match[1]}`
      )
      expect(keys.length).toBeGreaterThan(10)
      for (const key of keys) {
        expect(m.i18n.global.te(key, locale), key).toBe(true)
        expect(m.i18n.global.t(key)).not.toBe(key)
      }
      expect(m.i18n.global.t('modelPlaza.table.perSecond')).toBe(
        locale === 'zh' ? '按秒计费' : 'Per second'
      )
      expect(m.i18n.global.t('modelPlaza.table.perUnitSecond')).toBe(
        locale === 'zh' ? '/ 秒' : '/ second'
      )
    })
})
