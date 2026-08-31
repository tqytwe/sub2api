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

function withoutLocaleQuery(query: LocationQueryRaw): LocationQueryRaw {
  const next = { ...query }
  delete next.lang
  delete next.locale
  return next
}

function hasLocaleQuery(query: LocationQueryRaw): boolean {
  return Object.prototype.hasOwnProperty.call(query, 'lang')
    || Object.prototype.hasOwnProperty.call(query, 'locale')
}

export function resolvePublicLocaleRoute(
  targetLocale: PublicLocaleCode,
  currentPath: string,
  query: LocationQueryRaw = {},
): RouteLocationRaw | null {
  const path = normalizePublicPath(currentPath)
  // A public locale path is canonical. Keep business filters, but never carry
  // a prior `lang`/`locale` override across the language transition or a
  // refresh can undo the user's selection.
  const normalizedQuery = withoutLocaleQuery(query)

  if (targetLocale === 'en') {
    if (path === '/en' || path.startsWith('/en/')) {
      return hasLocaleQuery(query) ? withQuery(path, normalizedQuery) : null
    }
    if (path === '/about') return withQuery('/en/about', normalizedQuery)
    if (path === '/contact') return withQuery('/en/contact', normalizedQuery)
    if (path === '/pricing' || path.startsWith('/pricing/')) {
      return withQuery(path === '/pricing' ? '/en/catalog' : `/en/catalog${path.slice('/pricing'.length)}`, normalizedQuery)
    }
    if (path === '/catalog' || path.startsWith('/catalog/')) {
      return withQuery(path === '/catalog' ? '/en/catalog' : `/en/catalog${path.slice('/catalog'.length)}`, normalizedQuery)
    }
    if (path === '/docs') return withQuery('/en/docs', normalizedQuery)
    if (path === '/status') return withQuery('/en/status', normalizedQuery)
    return withQuery('/en', normalizedQuery)
  }

  if (path === '/en/catalog' || path.startsWith('/en/catalog/')) {
    return withQuery(path === '/en/catalog' ? '/catalog' : `/catalog${path.slice('/en/catalog'.length)}`, normalizedQuery)
  }
  if (path === '/en/docs') return withQuery('/docs', normalizedQuery)
  if (path === '/en/status') return withQuery('/status', normalizedQuery)
  if (path === '/en/about') return withQuery('/about', normalizedQuery)
  if (path === '/en/contact') return withQuery('/contact', normalizedQuery)
  if (path === '/en') return withQuery('/', normalizedQuery)
  return hasLocaleQuery(query) ? withQuery(path, normalizedQuery) : null
}
