import { createI18n } from 'vue-i18n'

type LocaleCode = 'en' | 'zh'

type LocaleMessages = Record<string, unknown>

const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'

const localeLoaders: Record<LocaleCode, () => Promise<{ default: LocaleMessages }>> = {
  en: () => import('./locales/en'),
  zh: () => import('./locales/zh'),
}

function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh'
}

function normalizeStoredLocale(saved: string | null): LocaleCode | null {
  if (!saved) return null
  // Legacy: zh-MY was a mistaken third locale — treat as zh.
  if (saved === 'zh-MY') return 'zh'
  return isLocaleCode(saved) ? saved : null
}

function getDefaultLocale(): LocaleCode {
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
  const current = getLocale()
  await loadLocaleMessages(current)
  document.documentElement.setAttribute('lang', current === 'zh' ? 'zh-MY' : current)
}

export async function setLocale(locale: string): Promise<void> {
  const normalized = normalizeStoredLocale(locale) ?? (isLocaleCode(locale) ? locale : null)
  if (!normalized) {
    return
  }

  await loadLocaleMessages(normalized)
  i18n.global.locale.value = normalized
  localStorage.setItem(LOCALE_KEY, normalized)
  document.documentElement.setAttribute('lang', normalized === 'zh' ? 'zh-MY' : normalized)

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
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'zh', name: '马来西亚中文', flag: '🇲🇾' },
] as const

export default i18n
