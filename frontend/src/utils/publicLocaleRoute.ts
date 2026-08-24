import type { LocationQueryRaw, RouteLocationRaw } from 'vue-router'

type PublicLocaleCode = 'en' | 'zh'

function normalizePublicPath(path: string): string {
  const clean = path.trim().split('?')[0]?.split('#')[0] ?? '/'
  if (clean === '/') return '/'
  return clean.replace(/\/+$/, '') || '/'
}

function withQuery(path: string, query: LocationQueryRaw): RouteLocationRaw {
  return Object.keys(query).length > 0 ? { path, query } : { path }
}

export function resolvePublicLocaleRoute(
  targetLocale: PublicLocaleCode,
  currentPath: string,
  query: LocationQueryRaw = {},
): RouteLocationRaw | null {
  const path = normalizePublicPath(currentPath)

  if (targetLocale === 'en') {
    if (path === '/en' || path.startsWith('/en/')) return null
    if (path === '/about') return { path: '/en/about' }
    if (path === '/contact') return { path: '/en/contact' }
    if (path === '/pricing' || path.startsWith('/pricing/')) {
      return withQuery(path === '/pricing' ? '/en/models' : `/en/models${path.slice('/pricing'.length)}`, query)
    }
    if (path === '/docs') return withQuery('/en/docs', query)
    return { path: '/en' }
  }

  if (path === '/en/models' || path.startsWith('/en/models/')) {
    return withQuery(path === '/en/models' ? '/models' : `/models${path.slice('/en/models'.length)}`, query)
  }
  if (path === '/en/docs') return withQuery('/docs', query)
  if (path === '/en/about') return { path: '/about' }
  if (path === '/en/contact') return { path: '/contact' }
  if (path === '/en') return { path: '/' }
  return null
}
