import { existsSync, readFileSync, statSync } from 'node:fs'
import { dirname, resolve } from 'node:path'

import { describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import {
  ROUTE_LOCALE_SCOPES,
  applyLocaleFromRoute,
  ensureLocaleMessagesForRoute,
  i18n,
  inheritedEnglishLocaleQuery,
  localeForRoute,
  localeScopesForRouteName,
  setLocale,
} from '../index'

function readPath(messages: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (!node || typeof node !== 'object') return undefined
    return (node as Record<string, unknown>)[key]
  }, messages)
}

const FRONTEND_SRC = resolve(process.cwd(), 'src')
const SOURCE_EXTENSIONS = ['', '.ts', '.vue', '.js', '/index.ts', '/index.vue', '/index.js']
const I18N_LITERAL_CALL = /(?:\bt|\$t)\s*\(\s*(['"])([A-Za-z][A-Za-z0-9_-]*(?:\.[A-Za-z0-9_-]+)+)\1/g
const I18N_KEYPATH_ATTRIBUTE = /\bkeypath\s*=\s*(['"])([A-Za-z][A-Za-z0-9_-]*(?:\.[A-Za-z0-9_-]+)+)\1/g
// Dynamic imports are lazy by definition and must not expand the initial
// cold-visit dependency contract. They are covered when their own route is
// visited; only statically imported Vue children belong to this scan.
const LOCAL_SOURCE_IMPORT = /from\s*(['"])(@\/[^'"]+|\.{1,2}\/[^'"]+)\1/g

function routeComponentFiles(): Map<string, string> {
  const routerSource = readFileSync(resolve(FRONTEND_SRC, 'router/index.ts'), 'utf8')
  const files = new Map<string, string>()
  const routeWithComponent = /name:\s*'([^']+)'[\s\S]{0,1200}?component:\s*\(\)\s*=>\s*import\('(@\/[^']+)'\)/g

  for (const match of routerSource.matchAll(routeWithComponent)) {
    files.set(match[1], resolve(FRONTEND_SRC, match[2].slice(2)))
  }
  return files
}

function resolveSourceImport(sourceFile: string, specifier: string): string | null {
  const unresolved = specifier.startsWith('@/')
    ? resolve(FRONTEND_SRC, specifier.slice(2))
    : resolve(dirname(sourceFile), specifier)

  for (const extension of SOURCE_EXTENSIONS) {
    const candidate = `${unresolved}${extension}`
    if (existsSync(candidate) && statSync(candidate).isFile()) return candidate
  }
  return null
}

function componentLocaleKeys(entryFile: string): string[] {
  const pending = [entryFile]
  const visited = new Set<string>()
  const keys = new Set<string>()

  while (pending.length > 0) {
    const sourceFile = pending.pop()
    if (!sourceFile || visited.has(sourceFile) || !existsSync(sourceFile)) continue
    visited.add(sourceFile)

    const source = readFileSync(sourceFile, 'utf8')
    for (const expression of [I18N_LITERAL_CALL, I18N_KEYPATH_ATTRIBUTE]) {
      expression.lastIndex = 0
      for (const match of source.matchAll(expression)) keys.add(match[2])
    }

    LOCAL_SOURCE_IMPORT.lastIndex = 0
    for (const match of source.matchAll(LOCAL_SOURCE_IMPORT)) {
      const dependency = resolveSourceImport(sourceFile, match[2])
      // Only follow renderable Vue dependencies. Traversing API/store imports
      // reaches the router and every unrelated route, which would turn a
      // route-level cold-start test into a meaningless whole-app union test.
      if (dependency?.startsWith(FRONTEND_SRC) && dependency.endsWith('.vue')) {
        pending.push(dependency)
      }
    }
  }

  return [...keys].sort()
}

function translated(locale: 'zh' | 'en', path: string): string {
  const messages = i18n.global.getLocaleMessage(locale) as Record<string, unknown>
  const value = readPath(messages, path)
  expect(typeof value, `${locale}:${path}`).toBe('string')
  expect((value as string).trim(), `${locale}:${path}`).not.toBe('')
  return value as string
}

async function loadFreshI18n() {
  // The production loader caches already loaded scopes. A fresh module proves
  // the first browser visit receives its full URL-derived fragment union.
  vi.resetModules()
  return import('../index')
}

const ROUTE_RUNTIME_KEYS = {
  Dashboard: ['dashboard.title'],
  Keys: ['keys.title'],
  BatchImageGuide: ['batchImageGuide.title', 'batchImage.create.apiKey'],
  Usage: [
    'usage.title',
    'usage.tabs.errors',
    'usage.time',
    'usage.reasoningEffort',
    'usage.inboundEndpoint',
    'usage.rate',
    'usage.userBilled',
    'usage.original',
    'usage.firstToken',
    'usage.duration',
    'admin.usage.ipAddress',
    'admin.usage.inputTokens',
    'admin.usage.outputTokens',
    'admin.usage.cacheReadTokens',
    'admin.usage.cacheCreationTokens',
  ],
  Wallet: ['wallet.title'],
  AICreationSpace: ['imageStudio.title', 'promptLibrary.panel.title'],
  PlayHub: ['playHub.title'],
  CheckIn: ['checkin.title'],
  Affiliate: ['affiliate.title'],
  UserAvailableChannels: ['availableChannels.title'],
  Profile: ['profile.title'],
  Subscriptions: ['userSubscriptions.title', 'userSubscriptions.status.suspended'],
  PurchaseSubscription: ['payment.title'],
  OrderList: ['payment.orders.title'],
  AdminDashboard: ['admin.dashboard.title'],
  AdminOps: ['admin.ops.title'],
  AdminPlayOps: ['admin.playOps.title'],
  AdminGroups: ['admin.groups.title', 'admin.accounts.status.active'],
  AdminChannels: ['admin.channels.title'],
  AdminRiskControl: ['admin.riskControl.proxy', 'admin.riskControl.proxyHint'],
  AdminAccounts: ['admin.accounts.title', 'admin.accounts.vertexDesc'],
  AdminProxies: ['admin.proxies.title', 'admin.accounts.status.active'],
  AdminIPRisk: ['admin.ipRisk.title', 'admin.accounts.status.active'],
  AdminIPRiskActions: ['admin.ipRisk.actionsView.title', 'admin.accounts.status.active'],
  AdminPlugins: ['admin.plugins.title'],
  AdminSettings: ['admin.settings.title'],
  AdminAuditLogs: ['admin.audit.title'],
  AdminSubscriptions: ['admin.subscriptions.title', 'admin.subscriptions.description'],
  AdminPromptAudit: ['admin.promptAudit.title'],
  AdminFunds: [
    'admin.funds.title',
    'admin.funds.grants.offlineTitle',
    'admin.funds.status.pending_review',
  ],
  AdminPromoCodes: [
    'coupon.admin.checkinPoolTitle',
    'coupon.admin.checkinSplit',
    'coupon.admin.poolStatus.published',
    'coupon.admin.couponStatus.available',
  ],
  AdminUsage: ['admin.usage.title', 'usage.totalRequests', 'usage.tabs.usage'],
  AdminPlayBillingConfig: [
    'payment.admin.playBilling.eyebrow',
    'payment.admin.playBilling.title',
    'payment.admin.playBilling.saved',
  ],
} as const

const MODEL_CATALOG_MEDIA_CAPABILITY_KEYS = [
  'admin.modelCatalog.mediaCapabilities.undeclared',
  'admin.modelCatalog.mediaCapabilities.chat',
  'admin.modelCatalog.mediaCapabilities.image',
  'admin.modelCatalog.mediaCapabilities.video',
  'admin.modelCatalog.mediaCapabilities.audio',
  'admin.modelCatalog.mediaCapabilities.validation.modalities_required',
  'admin.modelCatalog.mediaCapabilities.validation.version_required',
  'admin.modelCatalog.mediaCapabilities.validation.adapter_required',
  'admin.modelCatalog.mediaCapabilities.validation.image_operations_required',
  'admin.modelCatalog.mediaCapabilities.validation.video_operations_required',
  'admin.modelCatalog.mediaCapabilities.validation.image_operations_invalid',
  'admin.modelCatalog.mediaCapabilities.validation.video_operations_invalid',
  'admin.modelCatalog.mediaCapabilities.validation.image_limits_invalid',
  'admin.modelCatalog.mediaCapabilities.validation.video_limits_invalid',
] as const

describe('route locale runtime scopes', () => {
  it('keeps every route component static key available on a cold visit', async () => {
    const routes = routeComponentFiles()
    expect(routes.size).toBeGreaterThan(0)

    for (const [routeName, componentFile] of routes) {
      const keys = componentLocaleKeys(componentFile)
      if (keys.length === 0) continue

      for (const locale of ['zh', 'en'] as const) {
        const fresh = await loadFreshI18n()
        await fresh.ensureLocaleMessagesForRoute(routeName, locale)
        const messages = fresh.i18n.global.getLocaleMessage(locale) as Record<string, unknown>

        for (const key of keys) {
          const value = readPath(messages, key)
          expect(typeof value, `${locale}:${routeName}:${key}`).toBe('string')
          expect((value as string).trim(), `${locale}:${routeName}:${key}`).not.toBe('')
          expect(value, `${locale}:${routeName}:${key}`).not.toBe(key)
        }
      }
    }
  }, 60_000)

  it.each([
    {
      path: '/dashboard',
      keys: ['nav.aiCreationSpace', 'nav.fundManagement', 'dashboard.title'],
    },
    {
      path: '/admin/usage',
      keys: [
        'nav.aiCreationSpace',
        'nav.fundManagement',
        'admin.accounts.status.active',
        'usage.totalRequests',
        'usage.apiKeyFilter',
        'usage.endpointDistribution',
        'usage.tabs.usage',
      ],
    },
    {
      path: '/usage',
      keys: [
        'nav.aiCreationSpace',
        'usage.title',
        'admin.dashboard.timeRange',
        'admin.usage.billingMode',
      ],
    },
    {
      path: '/admin/funds',
      keys: ['admin.funds.title', 'admin.funds.grants.offlineTitle', 'admin.funds.status.pending_review'],
    },
    {
      path: '/admin/promo-codes',
      keys: [
        'coupon.admin.checkinPoolTitle',
        'coupon.admin.checkinSplit',
        'coupon.admin.poolStatus.published',
        'coupon.admin.couponStatus.available',
      ],
    },
  ])('loads every shell and page fragment for a cold $path visit', async ({ path, keys }) => {
    for (const locale of ['zh', 'en'] as const) {
      const fresh = await loadFreshI18n()
      await fresh.ensureLocaleMessagesForPath(path, locale)

      const messages = fresh.i18n.global.getLocaleMessage(locale) as Record<string, unknown>
      for (const key of keys) {
        const value = readPath(messages, key)
        expect(typeof value, `${locale}:${path}:${key}`).toBe('string')
        expect((value as string).trim(), `${locale}:${path}:${key}`).not.toBe('')
        expect(value, `${locale}:${path}:${key}`).not.toBe(key)
      }
    }
  }, 30_000)

  it.each(['zh', 'en'] as const)('loads the AdminUsage shell keys without relying on a prior route for %s', async (locale) => {
    expect(localeScopesForRouteName('AdminUsage')).toEqual(expect.arrayContaining([
      'workspace-shell',
      'admin-shell',
      'user-dashboard',
      'admin-ops',
    ]))

    await ensureLocaleMessagesForRoute('AdminUsage', locale)

    for (const key of [
      'nav.fundManagement',
      'admin.accounts.status.active',
      'usage.totalRequests',
      'admin.ops.errorLog.type',
    ]) {
      expect(translated(locale, key)).not.toBe(key)
    }
  })

  it('declares one typed locale-scope union for every named route', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const routeNames = [...routerSource.matchAll(/name: '([^']+)'/g)].map((match) => match[1]).sort()

    expect(Object.keys(ROUTE_LOCALE_SCOPES).sort()).toEqual(routeNames)
    for (const routeName of routeNames) {
      expect(localeScopesForRouteName(routeName)).toContain('core')
      expect(localeScopesForRouteName(routeName)).not.toContain('full')
    }
  })

  it.each(['zh', 'en'] as const)('loads and merges every declared route union for %s', async (locale) => {
    for (const [routeName, scopes] of Object.entries(ROUTE_LOCALE_SCOPES)) {
      await ensureLocaleMessagesForRoute(routeName, locale)

      expect(translated(locale, 'common.loading')).not.toBe('common.loading')
      expect(translated(locale, 'nav.dashboard')).not.toBe('nav.dashboard')

      if (scopes.includes('workspace-shell')) {
        expect(translated(locale, 'nav.aiCreationSpace')).not.toBe('nav.aiCreationSpace')
        expect(translated(locale, 'nav.fundManagement')).not.toBe('nav.fundManagement')
      }
      if (scopes.includes('admin-shell')) {
        expect(translated(locale, 'admin.accounts.status.active')).not.toBe('admin.accounts.status.active')
        expect(translated(locale, 'status.unknown')).not.toBe('status.unknown')
      }

      for (const key of ROUTE_RUNTIME_KEYS[routeName as keyof typeof ROUTE_RUNTIME_KEYS] ?? []) {
        expect(translated(locale, key)).not.toBe(key)
      }
    }
  })

  it.each(['zh', 'en'] as const)('loads the workspace shell instead of raw sidebar keys for %s', async (locale) => {
    await ensureLocaleMessagesForRoute('Dashboard', locale)

    expect(translated(locale, 'nav.aiCreationSpace')).not.toBe('nav.aiCreationSpace')
    expect(translated(locale, 'nav.fundManagement')).not.toBe('nav.fundManagement')
    expect(translated(locale, 'nav.playBilling')).not.toBe('nav.playBilling')
  })

  it.each(['zh', 'en'] as const)('loads reused admin dependencies for %s', async (locale) => {
    await ensureLocaleMessagesForRoute('AdminUsage', locale)
    await ensureLocaleMessagesForRoute('AdminGroups', locale)

    expect(translated(locale, 'usage.totalRequests')).not.toBe('usage.totalRequests')
    expect(translated(locale, 'usage.tabs.usage')).not.toBe('usage.tabs.usage')
    expect(translated(locale, 'admin.accounts.status.active')).not.toBe('admin.accounts.status.active')
  })

  it.each(['zh', 'en'] as const)('loads AI Creation Space copy from its explicit public-page dependency for %s', async (locale) => {
    await ensureLocaleMessagesForRoute('AICreationSpace', locale)

    expect(localeScopesForRouteName('AICreationSpace')).toContain('public-pages')
    expect(translated(locale, 'imageStudio.title')).not.toBe('imageStudio.title')
    expect(translated(locale, 'imageStudio.customDimensions')).not.toBe('imageStudio.customDimensions')
    expect(translated(locale, 'imageStudio.sizeConstraint')).not.toBe('imageStudio.sizeConstraint')
    expect(translated(locale, 'promptLibrary.panel.title')).not.toBe('promptLibrary.panel.title')
  })

  it.each(['zh', 'en'] as const)('loads every dynamic media-capability label for the model catalog in %s', async (locale) => {
    await ensureLocaleMessagesForRoute('AdminModelPlaza', locale)

    for (const key of MODEL_CATALOG_MEDIA_CAPABILITY_KEYS) {
      expect(translated(locale, key)).not.toBe(key)
    }
  })

  it('keeps the Vertex service-account and batch submission labels in the active language', async () => {
    await Promise.all([
      ensureLocaleMessagesForRoute('AdminAccounts', 'zh'),
      ensureLocaleMessagesForRoute('BatchImageGuide', 'zh'),
      ensureLocaleMessagesForRoute('AdminAccounts', 'en'),
      ensureLocaleMessagesForRoute('BatchImageGuide', 'en'),
    ])

    expect(translated('zh', 'admin.accounts.vertexDesc')).toBe('服务账号')
    expect(translated('en', 'admin.accounts.vertexDesc')).toBe('Service Account')
    expect(translated('zh', 'batchImage.create.apiKey')).toBe('提交密钥')
    expect(translated('en', 'batchImage.create.apiKey')).toBe('API key')
  })

  it('loads the suspended subscription status from the active route fragment', async () => {
    await Promise.all([
      ensureLocaleMessagesForRoute('Subscriptions', 'zh'),
      ensureLocaleMessagesForRoute('Subscriptions', 'en'),
    ])

    expect(translated('zh', 'userSubscriptions.status.suspended')).toBe('已暂停')
    expect(translated('en', 'userSubscriptions.status.suspended')).toBe('Suspended')
  })

  it.each([
    'Dashboard',
    'Keys',
    'KeySpeedTest',
    'BatchImageGuide',
    'Usage',
    'Wallet',
    'Redeem',
    'AICreationSpace',
    'PlayHub',
    'CheckIn',
    'Affiliate',
    'UserAvailableChannels',
    'Profile',
    'Subscriptions',
    'PurchaseSubscription',
    'OrderList',
    'PaymentQRCode',
    'StripePayment',
    'AirwallexPayment',
    'CustomPage',
  ] as const)('%s loads the workspace shell before rendering AppLayout', (routeName) => {
    expect(localeScopesForRouteName(routeName)).toContain('workspace-shell')
  })

  it('uses URL state, not old local storage state, to resolve the workspace language', () => {
    localStorage.setItem('sub2api_locale', 'en')

    expect(localeForRoute('/dashboard')).toBe('zh')
    expect(localeForRoute('/dashboard', { lang: 'en' })).toBe('en')
    expect(localeForRoute('/dashboard', { lang: 'zh' })).toBe('zh')
    expect(localeForRoute('/catalog', { lang: 'en' })).toBe('en')
    expect(localeForRoute('/en/catalog', { lang: 'zh' })).toBe('en')
  })

  it('clears the legacy locale preference on a Chinese workspace route', async () => {
    localStorage.setItem('sub2api_locale', 'en')
    i18n.global.locale.value = 'zh'

    await applyLocaleFromRoute('/dashboard', {})

    expect(localStorage.getItem('sub2api_locale')).toBeNull()
    expect(i18n.global.locale.value).toBe('zh')
  })

  it('keeps the last requested locale when lazy locale work resolves out of order', async () => {
    setActivePinia(createPinia())
    i18n.global.locale.value = 'zh'
    document.documentElement.setAttribute('lang', 'zh-CN')

    const olderEnglishTransition = setLocale('en')
    const newerChineseTransition = setLocale('zh')

    await Promise.all([olderEnglishTransition, newerChineseTransition])

    expect(i18n.global.locale.value).toBe('zh')
    expect(document.documentElement.lang).toBe('zh-CN')
  })

  it('marks a superseded route locale transition stale so its router guard cannot update the title', async () => {
    setActivePinia(createPinia())
    i18n.global.locale.value = 'zh'
    document.documentElement.setAttribute('lang', 'zh-CN')

    const olderEnglishNavigation = applyLocaleFromRoute('/en/docs', {})
    const newerChineseNavigation = applyLocaleFromRoute('/docs', {})

    await expect(olderEnglishNavigation).resolves.toBe(false)
    await expect(newerChineseNavigation).resolves.toBe(true)
    expect(i18n.global.locale.value).toBe('zh')
    expect(document.documentElement.lang).toBe('zh-CN')
  })

  it('preserves only an explicit English workspace query during internal navigation', () => {
    expect(inheritedEnglishLocaleQuery({ lang: 'en' }, { tab: 'usage' }, '/wallet')).toEqual({
      lang: 'en',
      tab: 'usage',
    })
    expect(inheritedEnglishLocaleQuery({ lang: 'en' }, {}, '/en/catalog')).toBeNull()
    expect(inheritedEnglishLocaleQuery({ lang: 'zh' }, {}, '/wallet')).toBeNull()
    expect(inheritedEnglishLocaleQuery({}, {}, '/wallet')).toBeNull()
  })

  it('does not fall back to Chinese when English has no message', async () => {
    await ensureLocaleMessagesForRoute('Dashboard', 'en')
    i18n.global.locale.value = 'en'

    expect(i18n.global.t('__missing_runtime_translation__')).toBe('__missing_runtime_translation__')
  })
})
