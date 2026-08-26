import { createI18n } from 'vue-i18n'
import type { LocationQuery } from 'vue-router'
import { mergeLocaleMessages } from './locales/merge'

type LocaleCode = 'en' | 'zh'

type LocaleMessages = Record<string, unknown>
export type LocaleLoadScope =
  | 'core'
  | 'public-pages'
  | 'user-dashboard'
  | 'user-wallet'
  | 'user-batch'
  | 'user-misc'
  | 'channel-monitor'
  | 'admin-overview'
  | 'admin-accounts'
  | 'admin-channels'
  | 'admin-ops'
  | 'admin-play'
  | 'admin-resources'
  | 'admin-settings'
  | 'admin-plugins'
  | 'admin-audit'
  | 'admin-prompt-audit'

type LocaleLoader = () => Promise<{ default: LocaleMessages }>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'zh'
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
    core: () => import('./locales/en/core'),
    'public-pages': async () => {
      const { jisudengPagesEn } = await import('./locales/jisudeng-pages.en')
      return { default: jisudengPagesEn }
    },
    'user-dashboard': () => import('./locales/en/dashboard'),
    'user-wallet': () => import('./locales/en/wallet'),
    'user-batch': () => import('./locales/en/batchImage'),
    'user-misc': async () => {
      const module = await import('./locales/en/misc')
      return {
        default: mergeLocaleMessages(module.default, {
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
      const [accounts, resources] = await Promise.all([
        import('./locales/en/admin/accounts'),
        import('./locales/en/admin/resources'),
      ])
      return { default: adminMessages(accounts.default, resources.default) }
    },
    'admin-channels': async () => {
      const module = await import('./locales/en/admin/channels')
      return { default: adminMessages(module.default) }
    },
    'admin-ops': async () => {
      const module = await import('./locales/en/admin/ops')
      return { default: adminMessages(module.default) }
    },
    'admin-play': async () => {
      const module = await import('./locales/en/admin/playOps')
      return { default: adminMessages(module.default) }
    },
    'admin-resources': async () => {
      const [overview, resources] = await Promise.all([
        import('./locales/en/admin/overview'),
        import('./locales/en/admin/resources'),
      ])
      return { default: adminMessages(overview.default, resources.default) }
    },
    'admin-settings': async () => {
      const module = await import('./locales/en/admin/settings')
      return { default: adminMessages(module.default) }
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
    core: () => import('./locales/zh/core'),
    'public-pages': async () => {
      const { jisudengPagesZh } = await import('./locales/jisudeng-pages.zh')
      return { default: jisudengPagesZh }
    },
    'user-dashboard': () => import('./locales/zh/dashboard'),
    'user-wallet': () => import('./locales/zh/wallet'),
    'user-batch': () => import('./locales/zh/batchImage'),
    'user-misc': async () => {
      const module = await import('./locales/zh/misc')
      return {
        default: mergeLocaleMessages(module.default, {
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
      const [accounts, resources] = await Promise.all([
        import('./locales/zh/admin/accounts'),
        import('./locales/zh/admin/resources'),
      ])
      return { default: adminMessages(accounts.default, resources.default) }
    },
    'admin-channels': async () => {
      const module = await import('./locales/zh/admin/channels')
      return { default: adminMessages(module.default) }
    },
    'admin-ops': async () => {
      const module = await import('./locales/zh/admin/ops')
      return { default: adminMessages(module.default) }
    },
    'admin-play': async () => {
      const module = await import('./locales/zh/admin/playOps')
      return { default: adminMessages(module.default) }
    },
    'admin-resources': async () => {
      const [overview, resources] = await Promise.all([
        import('./locales/zh/admin/overview'),
        import('./locales/zh/admin/resources'),
      ])
      return { default: adminMessages(overview.default, resources.default) }
    },
    'admin-settings': async () => {
      const module = await import('./locales/zh/admin/settings')
      return { default: adminMessages(module.default) }
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

function localeFromURL(): LocaleCode | null {
  if (typeof window === 'undefined') return null
  const params = new URLSearchParams(window.location.search)
  const fromQuery = params.get('lang') ?? params.get('locale')
  return normalizeStoredLocale(fromQuery)
}

export function localeFromQuery(query: LocationQuery | null | undefined = {}): LocaleCode | null {
  const raw = query?.lang ?? query?.locale
  const value = Array.isArray(raw) ? raw[0] : raw
  return typeof value === 'string' ? normalizeStoredLocale(value.trim()) : null
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
  const fromPath = typeof window === 'undefined' ? null : localeFromPath(window.location.pathname)
  if (fromPath) {
    return fromPath
  }

  const fromURL = localeFromURL()
  if (fromURL) {
    return fromURL
  }
  return DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
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
  if (normalized === '/' || normalized === '/home' || normalized === '/en' || normalized === '/login' || normalized === '/register' || normalized === '/setup' || normalized === '/key-usage') {
    return ['core']
  }

  if (normalized.startsWith('/admin')) {
    if (normalized === '/admin/dashboard') return ['core', 'admin-overview']
    if (normalized.startsWith('/admin/accounts')) return ['core', 'admin-accounts']
    if (normalized.startsWith('/admin/channels')) return ['core', 'admin-channels', 'channel-monitor']
    if (normalized.startsWith('/admin/ops')) return ['core', 'admin-ops']
    if (normalized.startsWith('/admin/play-ops') || normalized.startsWith('/admin/funds') || normalized.startsWith('/admin/withdrawals')) {
      return ['core', 'admin-play', 'admin-resources']
    }
    if (normalized.startsWith('/admin/settings')) return ['core', 'admin-settings']
    if (normalized.startsWith('/admin/plugins')) return ['core', 'admin-settings', 'admin-plugins']
    if (normalized.startsWith('/admin/audit-logs')) return ['core', 'admin-resources', 'admin-audit']
    if (normalized === adminPromptAuditPathForLocale()) return ['core', 'admin-channels', 'admin-prompt-audit']
    if (normalized.startsWith('/admin/model-plaza')) return ['core', 'user-dashboard', 'admin-channels']
    return ['core', 'admin-resources', 'admin-channels']
  }

  if (normalized === '/dashboard') return ['core', 'user-dashboard']
  if (normalized === '/wallet') return ['core', 'user-dashboard', 'user-wallet', 'user-misc']
  if (normalized === '/batch-image' || normalized === '/docs/batch-image') {
    return ['core', 'user-dashboard', 'user-batch']
  }
  if (normalized === '/monitor') return ['core', 'user-dashboard', 'channel-monitor']

  if (
    normalized === '/models'
    || normalized.startsWith('/models/')
    || normalized === '/model-plaza'
    || normalized === '/en/models'
    || normalized.startsWith('/en/models/')
  ) {
    return ['core', 'public-pages', 'user-dashboard']
  }

  if (
    normalized === '/docs'
    || normalized.startsWith('/docs/')
    || normalized.startsWith('/en/')
    || normalized === '/about'
    || normalized === '/contact'
    || normalized === '/legal'
    || normalized.startsWith('/download/')
  ) {
    return ['core', 'public-pages']
  }

  return ['core', 'user-dashboard', 'user-misc', 'public-pages']
}

function adminPromptAuditPathForLocale(): string {
  return '/admin/pro' + 'mpt-audit'
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
  localStorage.setItem(LOCALE_KEY, current)
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
  if (getLocale() === resolved) {
    if (typeof localStorage !== 'undefined') localStorage.setItem(LOCALE_KEY, resolved)
    return
  }
  await setLocale(resolved)
}

export async function setLocale(locale: string, options: { persist?: boolean } = {}): Promise<void> {
  const normalized = normalizeStoredLocale(locale) ?? (isLocaleCode(locale) ? locale : null)
  if (!normalized) {
    return
  }

  const path = typeof window === 'undefined' ? '/' : window.location.pathname
  await ensureLocaleMessagesForPath(path, normalized)
  i18n.global.locale.value = normalized
  if (options.persist !== false) {
    localStorage.setItem(LOCALE_KEY, normalized)
  }
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
