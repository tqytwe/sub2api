import type { RouteLocationRaw } from 'vue-router'

export const PUBLIC_ROUTE_NAMES = {
  aiCreationSpace: 'AICreationSpace',
  about: 'About',
  englishAbout: 'EnglishAbout',
  adminDashboard: 'AdminDashboard',
  androidDownload: 'AndroidDownload',
  contact: 'Contact',
  englishContact: 'EnglishContact',
  dashboard: 'Dashboard',
  docs: 'Docs',
  englishDocs: 'EnglishDocs',
  englishHome: 'EnglishHome',
  englishModels: 'EnglishModels',
  englishStatus: 'EnglishStatus',
  keyUsage: 'KeyUsage',
  login: 'Login',
  models: 'Models',
  status: 'Status',
  register: 'Register',
} as const

export type HomePrimaryNavKey =
  | 'models'
  | 'docs'
  | 'creation'
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

export const MODELS_ROUTE: RouteLocationRaw = { name: PUBLIC_ROUTE_NAMES.models }

export function dashboardEntryRoute(isAdmin: boolean): RouteLocationRaw {
  return { name: isAdmin ? PUBLIC_ROUTE_NAMES.adminDashboard : PUBLIC_ROUTE_NAMES.dashboard }
}

export function authEntryRoute(preferRegister: boolean): RouteLocationRaw {
  return { name: preferRegister ? PUBLIC_ROUTE_NAMES.register : PUBLIC_ROUTE_NAMES.login }
}

export function aiCreationSpaceEntryRoute(isAuthenticated: boolean, locale: 'zh' | 'en' = 'zh'): RouteLocationRaw {
  const workspacePath = locale === 'en' ? '/ai-creation-space?lang=en' : '/ai-creation-space'
  if (isAuthenticated) {
    return locale === 'en'
      ? { name: PUBLIC_ROUTE_NAMES.aiCreationSpace, query: { lang: 'en' } }
      : { name: PUBLIC_ROUTE_NAMES.aiCreationSpace }
  }
  return {
    name: PUBLIC_ROUTE_NAMES.register,
    query: { redirect: workspacePath },
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
    { key: 'models', labelKey: 'home.jisudeng.nav.models', to: locale === 'en' ? { name: PUBLIC_ROUTE_NAMES.englishModels } : MODELS_ROUTE },
    { key: 'docs', labelKey: 'home.jisudeng.nav.docs', to: locale === 'en' ? { name: PUBLIC_ROUTE_NAMES.englishDocs } : { name: PUBLIC_ROUTE_NAMES.docs } },
    { key: 'creation', labelKey: 'home.jisudeng.nav.creation', to: aiCreationSpaceEntryRoute(isAuthenticated, locale) },
    { key: 'keyUsage', labelKey: 'home.jisudeng.nav.keyUsage', to: sharedRoute(PUBLIC_ROUTE_NAMES.keyUsage) },
    { key: 'about', labelKey: 'home.jisudeng.nav.about', to: locale === 'en' ? { name: PUBLIC_ROUTE_NAMES.englishAbout } : { name: PUBLIC_ROUTE_NAMES.about } },
    { key: 'contact', labelKey: 'home.jisudeng.nav.contact', to: locale === 'en' ? { name: PUBLIC_ROUTE_NAMES.englishContact } : CONTACT_ROUTE, requiresSupportContact: true },
  ]
}
