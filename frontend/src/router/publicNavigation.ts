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
  models: 'Models',
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
  | 'androidApp'

export interface HomePrimaryNavItem {
  key: HomePrimaryNavKey
  labelKey: string
  to: RouteLocationRaw
  requiresSupportContact?: boolean
}

export const CONTACT_ROUTE: RouteLocationRaw = { name: PUBLIC_ROUTE_NAMES.contact }

export function dashboardEntryRoute(isAdmin: boolean): RouteLocationRaw {
  return { name: isAdmin ? PUBLIC_ROUTE_NAMES.adminDashboard : PUBLIC_ROUTE_NAMES.dashboard }
}

export function authEntryRoute(preferRegister: boolean): RouteLocationRaw {
  return { name: preferRegister ? PUBLIC_ROUTE_NAMES.register : PUBLIC_ROUTE_NAMES.login }
}

export function imageStudioEntryRoute(isAuthenticated: boolean): RouteLocationRaw {
  if (isAuthenticated) return { name: PUBLIC_ROUTE_NAMES.imageStudio }
  return {
    name: PUBLIC_ROUTE_NAMES.register,
    query: { redirect: '/image-studio' },
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

export function buildHomePrimaryNav(isAuthenticated: boolean): HomePrimaryNavItem[] {
  return [
    { key: 'models', labelKey: 'home.jisudeng.nav.models', to: { name: PUBLIC_ROUTE_NAMES.models } },
    { key: 'docs', labelKey: 'home.jisudeng.nav.docs', to: { name: PUBLIC_ROUTE_NAMES.docs } },
    { key: 'creation', labelKey: 'home.jisudeng.nav.creation', to: imageStudioEntryRoute(isAuthenticated) },
    { key: 'prompts', labelKey: 'home.jisudeng.nav.prompts', to: { name: PUBLIC_ROUTE_NAMES.promptSquare } },
    { key: 'keyUsage', labelKey: 'home.jisudeng.nav.keyUsage', to: { name: PUBLIC_ROUTE_NAMES.keyUsage } },
    { key: 'about', labelKey: 'home.jisudeng.nav.about', to: { name: PUBLIC_ROUTE_NAMES.about } },
    { key: 'contact', labelKey: 'home.jisudeng.nav.contact', to: CONTACT_ROUTE, requiresSupportContact: true },
    { key: 'androidApp', labelKey: 'home.jisudeng.nav.androidApp', to: { name: PUBLIC_ROUTE_NAMES.androidDownload } },
  ]
}
