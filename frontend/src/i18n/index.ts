import { createI18n } from 'vue-i18n'
import type { LocationQuery } from 'vue-router'

type LocaleCode = 'en' | 'zh'

type LocaleMessages = Record<string, unknown>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'zh'

const localeLoaders: Record<LocaleCode, () => Promise<{ default: LocaleMessages }>> = {
  en: () => import('./locales/en'),
  zh: () => import('./locales/zh'),
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

function localeFromURL(): LocaleCode | null {
  if (typeof window === 'undefined') return null
  const params = new URLSearchParams(window.location.search)
  const fromQuery = params.get('lang') ?? params.get('locale')
  return normalizeStoredLocale(fromQuery)
}

function getDefaultLocale(): LocaleCode {
  const fromURL = localeFromURL()
  if (fromURL) {
    return fromURL
  }

  const saved = normalizeStoredLocale(localStorage.getItem(LOCALE_KEY))
  if (saved) {
    return saved
  }

  const browserLang = navigator.language.toLowerCase()
  if (browserLang.startsWith('zh')) {
    return 'zh'
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

const loadedLocales = new Set<LocaleCode>()

export async function loadLocaleMessages(locale: LocaleCode): Promise<void> {
  if (loadedLocales.has(locale)) {
    return
  }

  const loader = localeLoaders[locale]
  const module = await loader()
  i18n.global.setLocaleMessage(locale, module.default)
  loadedLocales.add(locale)
}

export async function initI18n(): Promise<void> {
  const fromURL = localeFromURL()
  if (fromURL) {
    await loadLocaleMessages(fromURL)
    i18n.global.locale.value = fromURL
    localStorage.setItem(LOCALE_KEY, fromURL)
  } else {
    const current = getLocale()
    await loadLocaleMessages(current)
  }
  document.documentElement.setAttribute('lang', documentLanguage(getLocale()))
}

export async function applyLocaleFromRouteQuery(query: LocationQuery): Promise<void> {
  const raw = query.lang ?? query.locale
  const value = Array.isArray(raw) ? raw[0] : raw
  if (typeof value !== 'string' || !value.trim()) return
  await setLocale(value.trim())
}

export async function setLocale(locale: string): Promise<void> {
  const normalized = normalizeStoredLocale(locale) ?? (isLocaleCode(locale) ? locale : null)
  if (!normalized) {
    return
  }

  await loadLocaleMessages(normalized)
  i18n.global.locale.value = normalized
  localStorage.setItem(LOCALE_KEY, normalized)
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
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
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
