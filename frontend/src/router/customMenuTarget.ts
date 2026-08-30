type CustomMenuTargetInput = { url?: string }
type CustomMenuTargetItem = CustomMenuTargetInput & { id?: string }
type WorkspaceLocale = 'zh' | 'en'

// This ID was published in production bookmarks before docs became a native
// route. Keep this one exact compatibility mapping; arbitrary custom pages
// must continue through the explicit iframe flow.
export const LEGACY_DOC_CUSTOM_PAGE_ID = '095790f89fc04920'
export const CANONICAL_APPLICATION_ORIGIN = 'https://www.jisudeng.com'

function normalizeOrigin(origin: string): string | null {
  try {
    const parsed = new URL(origin)
    if ((parsed.protocol !== 'http:' && parsed.protocol !== 'https:') || parsed.username || parsed.password) {
      return null
    }
    return parsed.origin
  } catch {
    return null
  }
}

export function canonicalApplicationOrigin(): string | null {
  return normalizeOrigin(CANONICAL_APPLICATION_ORIGIN)
}

/** Convert only canonical same-origin docs targets into first-party routes. */
export function resolveCustomMenuRoute(
  item: CustomMenuTargetInput,
  locale: WorkspaceLocale | string,
): '/docs' | '/en/docs' | null {
  const canonicalOrigin = canonicalApplicationOrigin()
  if (!canonicalOrigin || !item.url?.trim()) return null

  try {
    const target = new URL(item.url.trim())
    if (
      target.origin !== canonicalOrigin ||
      target.username ||
      target.password ||
      (target.pathname !== '/docs' && target.pathname !== '/en/docs')
    ) {
      return null
    }
  } catch {
    return null
  }

  return locale.toLowerCase().startsWith('en') ? '/en/docs' : '/docs'
}

export function isNativeDocsMenuTarget(
  item: CustomMenuTargetInput,
): boolean {
  return resolveCustomMenuRoute(item, 'zh') !== null
}

/** Resolve a menu entry by its exact id before applying the URL policy. */
export function resolveCustomMenuRouteById(
  items: readonly CustomMenuTargetItem[],
  id: string,
  locale: WorkspaceLocale | string,
): '/docs' | '/en/docs' | null {
  const item = items.find((candidate) => candidate.id === id)
  return item ? resolveCustomMenuRoute(item, locale) : null
}

/**
 * Detects first-party docs aliases only for the iframe security boundary.
 * Native navigation stays stricter: it accepts the exact canonical origin
 * above, while an alias can never receive panel context through its URL.
 */
export function isFirstPartyDocsTarget(item: CustomMenuTargetInput): boolean {
  if (!item.url?.trim()) return false

  try {
    const target = new URL(item.url.trim())
    return (
      (target.protocol === 'http:' || target.protocol === 'https:') &&
      !target.username &&
      !target.password &&
      (target.hostname === 'jisudeng.com' || target.hostname === 'www.jisudeng.com') &&
      (target.pathname === '/docs' || target.pathname === '/en/docs')
    )
  } catch {
    return false
  }
}

export function resolveLegacyDocsCustomPageRoute(
  id: string,
  locale: WorkspaceLocale | string,
): '/docs' | '/en/docs' | null {
  if (id !== LEGACY_DOC_CUSTOM_PAGE_ID) return null
  return locale.toLowerCase().startsWith('en') ? '/en/docs' : '/docs'
}
