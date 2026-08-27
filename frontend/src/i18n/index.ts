import { createI18n } from 'vue-i18n'
import type { LocationQuery } from 'vue-router'
import { mergeLocaleMessages } from './locales/merge'
import {
  localeScopesForRouteName,
  type LocaleLoadScope,
  type LocaleRouteName,
} from './routeScopes'

export {
  LOCALE_LOAD_SCOPES,
  localeScopesForRouteName,
  ROUTE_LOCALE_SCOPES,
} from './routeScopes'
export type { LocaleLoadScope, LocaleRouteName } from './routeScopes'

type LocaleCode = 'en' | 'zh'

type LocaleMessages = Record<string, unknown>

type LocaleLoader = () => Promise<{ default: LocaleMessages }>

const DEFAULT_LOCALE: LocaleCode = 'zh'
const LEGACY_LOCALE_STORAGE_KEY = 'sub2api_locale'
const CHINESE_PUBLIC_LOCALE_PATHS = new Set([
  '/',
  '/home',
  '/models',
  '/docs',
  '/login',
  '/register',
  '/about',
  '/contact',
  '/setup',
  '/key-usage',
])

function adminMessages(...fragments: LocaleMessages[]): LocaleMessages {
  return {
    admin: fragments.reduce<LocaleMessages>(
      (messages, fragment) => mergeLocaleMessages(messages, fragment),
      {},
    ),
  }
}

const localeLoaders: Record<LocaleCode, Record<LocaleLoadScope, LocaleLoader>> = {
  en: {
    core: async () => {
      const [core, legacy] = await Promise.all([
        import('./locales/en/core'),
        import('./locales/en/legacy/core'),
      ])
      return { default: mergeLocaleMessages(legacy.default, core.default) }
    },
    'public-pages': async () => {
      const { jisudengPagesEn } = await import('./locales/jisudeng-pages.en')
      return { default: jisudengPagesEn }
    },
    'workspace-shell': () => import('./locales/en/workspaceShell'),
    'admin-shell': () => import('./locales/en/adminShell'),
    'user-dashboard': async () => {
      const [dashboard, legacy] = await Promise.all([
        import('./locales/en/dashboard'),
        import('./locales/en/legacy/user-dashboard'),
      ])
      return { default: mergeLocaleMessages(legacy.default, dashboard.default) }
    },
    'user-usage': () => import('./locales/en/userUsage'),
    'user-wallet': () => import('./locales/en/wallet'),
    'user-batch': () => import('./locales/en/batchImage'),
    'user-misc': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/en/misc'),
        import('./locales/en/legacy/user-misc'),
      ])
      return {
        default: mergeLocaleMessages(mergeLocaleMessages(legacy.default, module.default), {
          marketplace: { title: 'AI Model Marketplace', status: { unavailable: 'The marketplace is not available yet' } },
          nextChatLaunch: {
            title: 'Opening AI Creation Space',
            loading: 'Creating a secure session for your account...',
            failed: 'AI Creation Space is unavailable right now. Please try again later.',
          },
        }),
      }
    },
    'channel-monitor': () => import('./locales/en/channelMonitorV2'),
    'admin-overview': async () => {
      const module = await import('./locales/en/admin/overview')
      return { default: adminMessages(module.default) }
    },
    'admin-accounts': async () => {
      const [accounts, resources, legacy] = await Promise.all([
        import('./locales/en/admin/accounts'),
        import('./locales/en/admin/resources'),
        import('./locales/en/legacy/admin-accounts'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(accounts.default, resources.default)) }
    },
    'admin-channels': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/en/admin/channels'),
        import('./locales/en/legacy/admin-channels'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-ops': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/en/admin/ops'),
        import('./locales/en/legacy/admin-ops'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-play': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/en/admin/playOps'),
        import('./locales/en/legacy/admin-play'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-resources': async () => {
      const [overview, resources, legacy] = await Promise.all([
        import('./locales/en/admin/overview'),
        import('./locales/en/admin/resources'),
        import('./locales/en/legacy/admin-resources'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(overview.default, resources.default)) }
    },
    'admin-settings': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/en/admin/settings'),
        import('./locales/en/legacy/admin-settings'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-plugins': async () => {
      const module = await import('./locales/en/admin/plugins')
      return { default: adminMessages(module.default) }
    },
    'admin-audit': async () => {
      const module = await import('./locales/en/admin/audit')
      return { default: adminMessages(module.default) }
    },
    'admin-prompt-audit': async () => {
      const module = await import('./locales/en/admin/promptAudit')
      return { default: adminMessages(module.default) }
    },
  },
  zh: {
    core: async () => {
      const [core, legacy] = await Promise.all([
        import('./locales/zh/core'),
        import('./locales/zh/legacy/core'),
      ])
      return { default: mergeLocaleMessages(legacy.default, core.default) }
    },
    'public-pages': async () => {
      const { jisudengPagesZh } = await import('./locales/jisudeng-pages.zh')
      return { default: jisudengPagesZh }
    },
    'workspace-shell': () => import('./locales/zh/workspaceShell'),
    'admin-shell': () => import('./locales/zh/adminShell'),
    'user-dashboard': async () => {
      const [dashboard, legacy] = await Promise.all([
        import('./locales/zh/dashboard'),
        import('./locales/zh/legacy/user-dashboard'),
      ])
      return { default: mergeLocaleMessages(legacy.default, dashboard.default) }
    },
    'user-usage': () => import('./locales/zh/userUsage'),
    'user-wallet': () => import('./locales/zh/wallet'),
    'user-batch': () => import('./locales/zh/batchImage'),
    'user-misc': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/zh/misc'),
        import('./locales/zh/legacy/user-misc'),
      ])
      return {
        default: mergeLocaleMessages(mergeLocaleMessages(legacy.default, module.default), {
          marketplace: { title: 'AI 模型商城', status: { unavailable: '商城功能暂未开放' } },
          nextChatLaunch: {
            title: '正在进入 AI创作空间',
            loading: '正在为当前账号创建安全会话...',
            failed: 'AI创作空间暂时无法打开，请稍后重试。',
          },
        }),
      }
    },
    'channel-monitor': () => import('./locales/zh/channelMonitorV2'),
    'admin-overview': async () => {
      const module = await import('./locales/zh/admin/overview')
      return { default: adminMessages(module.default) }
    },
    'admin-accounts': async () => {
      const [accounts, resources, legacy] = await Promise.all([
        import('./locales/zh/admin/accounts'),
        import('./locales/zh/admin/resources'),
        import('./locales/zh/legacy/admin-accounts'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(accounts.default, resources.default)) }
    },
    'admin-channels': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/zh/admin/channels'),
        import('./locales/zh/legacy/admin-channels'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-ops': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/zh/admin/ops'),
        import('./locales/zh/legacy/admin-ops'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-play': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/zh/admin/playOps'),
        import('./locales/zh/legacy/admin-play'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-resources': async () => {
      const [overview, resources, legacy] = await Promise.all([
        import('./locales/zh/admin/overview'),
        import('./locales/zh/admin/resources'),
        import('./locales/zh/legacy/admin-resources'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(overview.default, resources.default)) }
    },
    'admin-settings': async () => {
      const [module, legacy] = await Promise.all([
        import('./locales/zh/admin/settings'),
        import('./locales/zh/legacy/admin-settings'),
      ])
      return { default: mergeLocaleMessages(legacy.default, adminMessages(module.default)) }
    },
    'admin-plugins': async () => {
      const module = await import('./locales/zh/admin/plugins')
      return { default: adminMessages(module.default) }
    },
    'admin-audit': async () => {
      const module = await import('./locales/zh/admin/audit')
      return { default: adminMessages(module.default) }
    },
    'admin-prompt-audit': async () => {
      const module = await import('./locales/zh/admin/promptAudit')
      return { default: adminMessages(module.default) }
    },
  },
}

function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh'
}

function normalizeStoredLocale(saved: string | null): LocaleCode | null {
  if (!saved) return null
  if (saved === 'zh-MY' || saved === 'zh-CN' || saved === 'zh') return 'zh'
  if (saved === 'en' || saved === 'en-US') return 'en'
  return null
}

function documentLanguage(locale: LocaleCode): string {
  return locale === 'en' ? 'en' : 'zh-CN'
}

function normalizeRoutePath(path: string): string {
  const clean = path.trim().split('?')[0]?.split('#')[0] ?? '/'
  if (clean === '/') return '/'
  return clean.replace(/\/+$/, '') || '/'
}

export function localeFromPath(path: string): LocaleCode | null {
  const normalized = normalizeRoutePath(path)
  if (normalized === '/en' || normalized.startsWith('/en/')) {
    return 'en'
  }
  if (normalized.startsWith('/models/') || normalized === '/models') {
    return 'zh'
  }
  if (CHINESE_PUBLIC_LOCALE_PATHS.has(normalized)) {
    return 'zh'
  }
  return null
}

export function localeFromQuery(query: LocationQuery | null | undefined = {}): LocaleCode | null {
  const raw = query?.lang ?? query?.locale
  const value = Array.isArray(raw) ? raw[0] : raw
  return typeof value === 'string' ? normalizeStoredLocale(value.trim()) : null
}

/**
 * Workspace English is URL-scoped. Preserve that explicit choice across an
 * internal navigation, while a no-query navigation continues to mean Chinese.
 */
export function inheritedEnglishLocaleQuery(
  fromQuery: LocationQuery,
  toQuery: LocationQuery,
  destinationPath: string,
): LocationQuery | null {
  if (
    localeFromQuery(fromQuery) === 'en'
    && localeFromQuery(toQuery) === null
    && !destinationPath.startsWith('/en')
  ) {
    return { ...toQuery, lang: 'en' }
  }
  return null
}

/**
 * Resolve the language for a route without consulting a persisted browser preference.
 * English is opt-in through the explicit /en route layer or a lang query parameter;
 * every other route is Chinese so an old English preference cannot leak into the app.
 */
export function localeForRoute(path: string, query: LocationQuery = {}): LocaleCode {
  const fromPath = localeFromPath(path)
  if (fromPath === 'en') return 'en'
  return localeFromQuery(query) ?? 'zh'
}

function getDefaultLocale(): LocaleCode {
  if (typeof window === 'undefined') return DEFAULT_LOCALE
  // Use the same precedence as router navigation. In particular, a cold
  // `/models?lang=en` visit must not render Chinese before the first guard.
  return localeForRoute(
    window.location.pathname,
    Object.fromEntries(new URLSearchParams(window.location.search).entries()),
  )
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  // Each route explicitly loads its zh/en union. Falling back across languages
  // masks missing fragments and produces mixed-language pages.
  fallbackLocale: false,
  fallbackWarn: false,
  missingWarn: false,
  messages: {},
  warnHtmlMessage: false,
})

const loadedLocaleScopes = new Map<LocaleCode, Set<LocaleLoadScope>>()
const pendingLocaleScopes = new Map<string, Promise<void>>()

function loadedScopesFor(locale: LocaleCode): Set<LocaleLoadScope> {
  const existing = loadedLocaleScopes.get(locale)
  if (existing) return existing
  const next = new Set<LocaleLoadScope>()
  loadedLocaleScopes.set(locale, next)
  return next
}

export function localeScopesForPath(path: string): LocaleLoadScope[] {
  const normalized = normalizeRoutePath(path)
  const routeName = routeNameForPath(normalized)
  return routeName ? localeScopesForRouteName(routeName) : ['core']
}

/**
 * Bootstrap has a path but not a resolved router record. The guard below uses
 * route names at runtime; this only keeps the initial render on the same
 * declared fragment union until the router becomes ready.
 */
function routeNameForPath(path: string): LocaleRouteName | null {
  if (path === '/' || path === '/home') return 'Home'
  if (path === '/en') return 'EnglishHome'
  if (path === '/en/models' || path.startsWith('/en/models/')) return 'EnglishModels'
  if (path === '/en/docs') return 'EnglishDocs'
  if (path === '/en/about') return 'EnglishAbout'
  if (path === '/en/contact') return 'EnglishContact'
  if (path === '/models' || path.startsWith('/models/')) return 'Models'
  if (path === '/model-plaza' || path === '/pricing' || path.startsWith('/pricing/')) return 'Pricing'
  if (path === '/docs') return 'Docs'
  if (path.startsWith('/docs/batch-image')) return 'BatchImageGuide'
  if (path.startsWith('/download/android')) return 'AndroidDownload'
  if (path === '/login') return 'Login'
  if (path === '/register') return 'Register'
  if (path === '/email-verify') return 'EmailVerify'
  if (path === '/setup') return 'Setup'
  if (path === '/key-usage') return 'KeyUsage'
  if (path.startsWith('/legal/')) return 'LegalDocument'
  if (path === '/about') return 'About'
  if (path === '/contact' || path === '/contact/qq') return 'Contact'
  if (path === '/blindbox') return 'Blindbox'
  if (path === '/arena') return 'Arena'
  if (path === '/quiz-quest') return 'QuizQuest'
  if (path === '/agent-team') return 'AgentTeam'
  if (path === '/dashboard') return 'Dashboard'
  if (path === '/keys') return 'Keys'
  if (path === '/keys/speed-test') return 'KeySpeedTest'
  if (path === '/batch-image') return 'BatchImageGuide'
  if (path === '/usage') return 'Usage'
  if (path === '/wallet') return 'Wallet'
  if (path === '/redeem') return 'Redeem'
  if (path === '/ai' || path === '/image-studio' || path === '/ai-creation-space') return 'AICreationSpace'
  if (path === '/play') return 'PlayHub'
  if (path === '/check-in') return 'CheckIn'
  if (path === '/affiliate') return 'Affiliate'
  if (path === '/available-channels') return 'UserAvailableChannels'
  if (path === '/profile') return 'Profile'
  if (path === '/subscriptions') return 'Subscriptions'
  if (path === '/purchase') return 'PurchaseSubscription'
  if (path === '/orders') return 'OrderList'
  if (path === '/payment/qrcode') return 'PaymentQRCode'
  if (path === '/payment/result') return 'PaymentResult'
  if (path === '/payment/stripe') return 'StripePayment'
  if (path === '/payment/airwallex') return 'AirwallexPayment'
  if (path === '/payment/stripe-popup') return 'StripePopup'
  if (path.startsWith('/custom/')) return 'CustomPage'
  if (path === '/admin/dashboard' || path === '/admin') return 'AdminDashboard'
  if (path === '/admin/ops') return 'AdminOps'
  if (path === '/admin/play-ops') return 'AdminPlayOps'
  if (path.startsWith('/admin/funds')) return 'AdminFunds'
  if (path === '/admin/withdrawals') return 'AdminWithdrawals'
  if (path === '/admin/audit-logs') return 'AdminAuditLogs'
  if (path === '/admin/users') return 'AdminUsers'
  if (path === '/admin/groups') return 'AdminGroups'
  if (path.startsWith('/admin/channels/monitor')) return 'AdminChannelMonitor'
  if (path.startsWith('/admin/channels')) return 'AdminChannels'
  if (path === '/admin/model-plaza') return 'AdminModelPlaza'
  if (path === '/monitor') return 'ChannelStatus'
  if (path === '/admin/subscriptions') return 'AdminSubscriptions'
  if (path === '/admin/accounts') return 'AdminAccounts'
  if (path === '/admin/plugins') return 'AdminPlugins'
  if (path === '/admin/announcements') return 'AdminAnnouncements'
  if (path === '/admin/proxies') return 'AdminProxies'
  if (path === '/admin/proxies/risk') return 'AdminIPRisk'
  if (path === '/admin/proxies/actions') return 'AdminIPRiskActions'
  if (path === '/admin/redeem') return 'AdminRedeem'
  if (path === '/admin/promo-codes') return 'AdminPromoCodes'
  if (path === '/admin/settings') return 'AdminSettings'
  if (path === '/admin/risk-control') return 'AdminRiskControl'
  if (path === '/admin/prompt-audit') return 'AdminPromptAudit'
  if (path === '/admin/usage') return 'AdminUsage'
  if (path === '/admin/affiliates/invites') return 'AdminAffiliateInvites'
  if (path === '/admin/affiliates/rebates') return 'AdminAffiliateRebates'
  if (path === '/admin/affiliates/transfers') return 'AdminAffiliateTransfers'
  if (path === '/admin/orders/dashboard') return 'AdminPaymentDashboard'
  if (path === '/admin/orders/plans') return 'AdminPaymentPlans'
  if (path === '/admin/orders/play-billing') return 'AdminPlayBillingConfig'
  if (path === '/admin/orders') return 'AdminOrders'
  return null
}

export async function loadLocaleMessages(locale: LocaleCode, scope: LocaleLoadScope = 'core'): Promise<void> {
  const loadedScopes = loadedScopesFor(locale)
  if (loadedScopes.has(scope)) return

  const pendingKey = `${locale}:${scope}`
  const existing = pendingLocaleScopes.get(pendingKey)
  if (existing) return existing

  const pending = localeLoaders[locale][scope]().then((module) => {
    i18n.global.mergeLocaleMessage(locale, module.default)
    loadedScopes.add(scope)
  }).finally(() => {
    pendingLocaleScopes.delete(pendingKey)
  })
  pendingLocaleScopes.set(pendingKey, pending)
  return pending
}

export async function ensureLocaleMessagesForPath(
  path: string,
  locale: LocaleCode = getLocale(),
  routeScopes?: readonly LocaleLoadScope[],
): Promise<void> {
  const scopes = routeScopes?.length
    ? Array.from(new Set<LocaleLoadScope>(['core', ...routeScopes]))
    : localeScopesForPath(path)
  await Promise.all(scopes.map((scope) => loadLocaleMessages(locale, scope)))
}

export async function ensureLocaleMessagesForRoute(
  routeName: unknown,
  locale: LocaleCode = getLocale(),
  routeScopes?: readonly LocaleLoadScope[],
): Promise<void> {
  const scopes = Array.from(new Set<LocaleLoadScope>([
    ...localeScopesForRouteName(routeName),
    ...(routeScopes ?? []),
  ]))
  await Promise.all(scopes.map((scope) => loadLocaleMessages(locale, scope)))
}

export async function initI18n(): Promise<void> {
  const path = typeof window === 'undefined' ? '/' : window.location.pathname
  const current = localeForRoute(
    path,
    typeof window === 'undefined'
      ? {}
      : Object.fromEntries(new URLSearchParams(window.location.search).entries()),
  )
  await ensureLocaleMessagesForPath(path, current)
  i18n.global.locale.value = current
  document.documentElement.setAttribute('lang', documentLanguage(getLocale()))
}

export async function applyLocaleFromRouteQuery(query: LocationQuery): Promise<void> {
  const raw = query.lang ?? query.locale
  const value = Array.isArray(raw) ? raw[0] : raw
  if (typeof value !== 'string' || !value.trim()) return
  await setLocale(value.trim())
}

export async function applyLocaleFromRoute(path: string, query: LocationQuery): Promise<void> {
  const resolved = localeForRoute(path, query)
  // Versions before URL-scoped locale selection persisted an English preference.
  // Clear it on Chinese routes so a rollback or an older cached bundle cannot
  // reintroduce English into a no-query workspace route.
  if (resolved === 'zh' && typeof localStorage !== 'undefined') {
    localStorage.removeItem(LEGACY_LOCALE_STORAGE_KEY)
  }
  if (getLocale() === resolved) {
    return
  }
  await setLocale(resolved)
}

export async function setLocale(locale: string): Promise<void> {
  const normalized = normalizeStoredLocale(locale) ?? (isLocaleCode(locale) ? locale : null)
  if (!normalized) {
    return
  }

  const path = typeof window === 'undefined' ? '/' : window.location.pathname
  await ensureLocaleMessagesForPath(path, normalized)
  i18n.global.locale.value = normalized
  document.documentElement.setAttribute('lang', documentLanguage(normalized))

  const { resolveRouteDocumentTitle } = await import('@/router/title')
  const { default: router } = await import('@/router')
  const { useAppStore } = await import('@/stores/app')
  const { useAuthStore } = await import('@/stores/auth')
  const { useAdminSettingsStore } = await import('@/stores/adminSettings')
  const route = router.currentRoute.value
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  const { applyPublicRouteSeo } = await import('@/utils/routeSeo')
  if (!applyPublicRouteSeo(route.path)) {
    document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
  }
}

export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  if (current === 'zh-MY') return 'zh'
  return isLocaleCode(current) ? current : DEFAULT_LOCALE
}

export const availableLocales = [
  { code: 'zh', name: '简体中文', flag: '中' },
  { code: 'en', name: 'English', flag: 'EN' },
] as const

export default i18n
