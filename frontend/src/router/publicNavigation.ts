import type { RouteLocationRaw } from 'vue-router'

export const PUBLIC_ROUTE_NAMES = {
  about: 'About',
  adminDashboard: 'AdminDashboard',
  androidDownload: 'AndroidDownload',
  contact: 'Contact',
  dashboard: 'Dashboard',
  docs: 'Docs',
  englishDocs: 'EnglishDocs',
  englishHome: 'EnglishHome',
  englishModels: 'EnglishModels',
  imageStudio: 'ImageStudio',
  keyUsage: 'KeyUsage',
  login: 'Login',
  pricing: 'Pricing',
  promptSquare: 'PromptSquare',
  register: 'Register',
} as const

export type HomePrimaryNavKey =
  | 'models'
  | 'docs'
  | 'creation'
  | 'prompts'
  | 'keyUsage'
  | 'about'
  | 'contact'

export interface HomePrimaryNavItem {
  key: HomePrimaryNavKey
  labelKey: string
  to: RouteLocationRaw
  requiresSupportContact?: boolean
}

export const CONTACT_ROUTE: RouteLocationRaw = { name: PUBLIC_ROUTE_NAMES.contact }

export const HOME_ROUTE: RouteLocationRaw = { name: 'Home' }

export const PRICING_ROUTE: RouteLocationRaw = { name: PUBLIC_ROUTE_NAMES.pricing }

export function dashboardEntryRoute(isAdmin: boolean): RouteLocationRaw {
  return { name: isAdmin ? PUBLIC_ROUTE_NAMES.adminDashboard : PUBLIC_ROUTE_NAMES.dashboard }
}

export function authEntryRoute(preferRegister: boolean): RouteLocationRaw {
  return { name: preferRegister ? PUBLIC_ROUTE_NAMES.register : PUBLIC_ROUTE_NAMES.login }
}

export function imageStudioEntryRoute(isAuthenticated: boolean, locale: 'zh' | 'en' = 'zh'): RouteLocationRaw {
  const studioPath = locale === 'en' ? '/image-studio?lang=en' : '/image-studio'
  if (isAuthenticated) {
    return locale === 'en'
      ? { name: PUBLIC_ROUTE_NAMES.imageStudio, query: { lang: 'en' } }
      : { name: PUBLIC_ROUTE_NAMES.imageStudio }
  }
  return {
    name: PUBLIC_ROUTE_NAMES.register,
    query: { redirect: studioPath },
  }
}

export function docsTopicRoute(cat: string, page: string): RouteLocationRaw {
  return {
    name: PUBLIC_ROUTE_NAMES.docs,
    query: { cat, page },
  }
}

export function englishDocsTopicRoute(cat: string, page: string): RouteLocationRaw {
  return {
    name: PUBLIC_ROUTE_NAMES.englishDocs,
    query: { cat, page },
  }
}

export function buildHomePrimaryNav(isAuthenticated: boolean, locale: 'zh' | 'en' = 'zh'): HomePrimaryNavItem[] {
  const sharedRoute = (name: string): RouteLocationRaw =>
    locale === 'en' ? { name, query: { lang: 'en' } } : { name }

  return [
    { key: 'models', labelKey: 'home.jisudeng.nav.models', to: locale === 'en' ? { name: PUBLIC_ROUTE_NAMES.englishModels } : PRICING_ROUTE },
    { key: 'docs', labelKey: 'home.jisudeng.nav.docs', to: locale === 'en' ? { name: PUBLIC_ROUTE_NAMES.englishDocs } : { name: PUBLIC_ROUTE_NAMES.docs } },
    { key: 'creation', labelKey: 'home.jisudeng.nav.creation', to: imageStudioEntryRoute(isAuthenticated, locale) },
    { key: 'prompts', labelKey: 'home.jisudeng.nav.prompts', to: sharedRoute(PUBLIC_ROUTE_NAMES.promptSquare) },
    { key: 'keyUsage', labelKey: 'home.jisudeng.nav.keyUsage', to: sharedRoute(PUBLIC_ROUTE_NAMES.keyUsage) },
    { key: 'about', labelKey: 'home.jisudeng.nav.about', to: sharedRoute(PUBLIC_ROUTE_NAMES.about) },
    { key: 'contact', labelKey: 'home.jisudeng.nav.contact', to: locale === 'en' ? sharedRoute(PUBLIC_ROUTE_NAMES.contact) : CONTACT_ROUTE, requiresSupportContact: true },
  ]
}
