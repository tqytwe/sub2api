/**
 * Shared URL builder for iframe-embedded pages.
 * Used by PurchaseSubscriptionView and CustomPageView to build consistent URLs
 * with user_id, token, theme, lang, ui_mode, src_host, and src parameters.
 */

const EMBEDDED_USER_ID_QUERY_KEY = 'user_id'
const EMBEDDED_AUTH_TOKEN_QUERY_KEY = 'token'
const EMBEDDED_THEME_QUERY_KEY = 'theme'
const EMBEDDED_LANG_QUERY_KEY = 'lang'
const EMBEDDED_UI_MODE_QUERY_KEY = 'ui_mode'
const EMBEDDED_UI_MODE_VALUE = 'embedded'
const EMBEDDED_SRC_HOST_QUERY_KEY = 'src_host'
const EMBEDDED_SRC_QUERY_KEY = 'src_url'

export interface EmbeddedUrlOptions {
  /** Omit all panel-derived query values for first-party documents and aliases. */
  includePanelContext?: boolean
}

export function buildEmbeddedUrl(
  baseUrl: string,
  userId?: number,
  authToken?: string | null,
  theme: 'light' | 'dark' = 'light',
  lang?: string,
  options: EmbeddedUrlOptions = {},
): string {
  if (!baseUrl) return baseUrl
  try {
    const url = new URL(baseUrl)
    if (options.includePanelContext === false) {
      // A historic first-party docs configuration can already contain these
      // names. Remove them as well as declining new context, otherwise an old
      // HTTP/alias URL could still leak a panel credential through the iframe.
		for (const key of [
        EMBEDDED_USER_ID_QUERY_KEY,
        EMBEDDED_AUTH_TOKEN_QUERY_KEY,
        EMBEDDED_THEME_QUERY_KEY,
        EMBEDDED_LANG_QUERY_KEY,
        EMBEDDED_UI_MODE_QUERY_KEY,
        EMBEDDED_SRC_HOST_QUERY_KEY,
        EMBEDDED_SRC_QUERY_KEY,
		]) {
			url.searchParams.delete(key)
		}
		url.hash = ''
		return url.toString()
    }
    if (userId) {
      url.searchParams.set(EMBEDDED_USER_ID_QUERY_KEY, String(userId))
    }
    if (authToken) {
      url.searchParams.set(EMBEDDED_AUTH_TOKEN_QUERY_KEY, authToken)
    }
    url.searchParams.set(EMBEDDED_THEME_QUERY_KEY, theme)
    if (lang) {
      url.searchParams.set(EMBEDDED_LANG_QUERY_KEY, lang)
    }
    url.searchParams.set(EMBEDDED_UI_MODE_QUERY_KEY, EMBEDDED_UI_MODE_VALUE)
    // Source tracking: let the embedded page know where it's being loaded from
    if (typeof window !== 'undefined') {
      url.searchParams.set(EMBEDDED_SRC_HOST_QUERY_KEY, window.location.origin)
      url.searchParams.set(EMBEDDED_SRC_QUERY_KEY, window.location.href)
    }
    return url.toString()
  } catch {
    return baseUrl
  }
}

export function detectTheme(): 'light' | 'dark' {
  if (typeof document === 'undefined') return 'light'
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}
