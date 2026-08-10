/**
 * Resumes a forum SSO authorize request that was interrupted by login.
 *
 * FLOW
 * ----
 * 1. The forum sends the browser to /api/v1/sso/oauth/authorize?...
 * 2. The user has no panel session, so the backend redirects to
 *    /login?forum_sso_resume=<the original authorize request URI>
 * 3. The user logs in by any means (password, passkey, 2FA, external OAuth).
 * 4. This composable replays the request and navigates to the forum callback.
 *
 * WHY A COMPOSABLE PLUS A ROUTE WATCH, NOT A CALL IN THE LOGIN HANDLER
 * --------------------------------------------------------------------
 * LoginView finishes login in three separate places (password, passkey, 2FA),
 * and external OAuth logins leave the SPA entirely and come back through a
 * different route. Hooking each exit would be easy to get subtly wrong and easy
 * to forget when a new login method is added. Instead this resolves whenever an
 * authenticated session and a pending resume parameter coexist, so every login
 * method is covered by construction.
 *
 * SECURITY
 * --------
 * `forum_sso_resume` arrives in a URL, so it is attacker-supplied: anyone can
 * send a victim to /login?forum_sso_resume=<anything>. Two rules contain that.
 *
 * First, only the query string is ever used. The value is parsed and the path is
 * discarded, so it cannot be turned into a navigation target. The only thing the
 * SPA navigates to is `redirect_uri` as returned by the backend, and the backend
 * only returns a redirect_uri that exact-matches its configured allowlist. A
 * forged parameter therefore cannot redirect a user anywhere the operator has
 * not explicitly allowed, which is what keeps this from becoming an open
 * redirector.
 *
 * Second, resumption requires an authenticated session, and the backend binds
 * the authorization code to the user behind the JWT. A forged parameter cannot
 * mint a code for anybody else.
 */

import { ref, watch, type Ref } from 'vue'
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

import { completeForumSsoAuthorize } from '@/api/forumSso'

/** Query parameter the backend uses to hand a pending request to the SPA. */
export const FORUM_SSO_RESUME_PARAM = 'forum_sso_resume'

export interface ParsedForumSsoResume {
  clientId: string
  redirectUri: string
  state: string
  scope: string
}

/**
 * Extracts the OAuth parameters from a `forum_sso_resume` value.
 *
 * Returns null when the value is absent or does not carry the two parameters
 * the authorize call requires. Everything else about the value, including its
 * path, is deliberately ignored.
 */
export function parseForumSsoResume(raw: unknown): ParsedForumSsoResume | null {
  const value = Array.isArray(raw) ? raw[0] : raw
  if (typeof value !== 'string' || value.trim() === '') {
    return null
  }

  let params: URLSearchParams
  try {
    // Resolved against an opaque base purely so relative and absolute values
    // parse the same way. The base, and the parsed origin and path, are then
    // discarded: only the query is used.
    params = new URL(value, 'https://forum-sso.invalid').searchParams
  } catch {
    return null
  }

  const clientId = (params.get('client_id') || '').trim()
  const redirectUri = (params.get('redirect_uri') || '').trim()
  if (!clientId || !redirectUri) {
    return null
  }

  return {
    clientId,
    redirectUri,
    state: params.get('state') || '',
    scope: params.get('scope') || ''
  }
}

export interface UseForumSsoResumeOptions {
  router: Router
  route: RouteLocationNormalizedLoaded
  /** Whether a panel session exists. Resumption waits for this to be true. */
  isAuthenticated: Ref<boolean>
  /** Reports a failed resume attempt to the user. */
  onError?: (message: string) => void
}

export interface UseForumSsoResume {
  /** True while the authorize call is in flight or navigation is starting. */
  isResuming: Ref<boolean>
  /** True when the current route carries a usable resume parameter. */
  hasPendingResume: Ref<boolean>
  /** Attempts resumption immediately; safe to call more than once. */
  resume: () => Promise<void>
}

export function useForumSsoResume(options: UseForumSsoResumeOptions): UseForumSsoResume {
  const { router, route, isAuthenticated, onError } = options

  const isResuming = ref(false)
  const hasPendingResume = ref(parseForumSsoResume(route.query[FORUM_SSO_RESUME_PARAM]) !== null)

  // Guards against a double navigation if both the watcher and an explicit
  // resume() call fire for the same parameter.
  let consumed = false

  async function resume(): Promise<void> {
    if (consumed || isResuming.value) return

    const parsed = parseForumSsoResume(route.query[FORUM_SSO_RESUME_PARAM])
    if (!parsed || !isAuthenticated.value) return

    consumed = true
    isResuming.value = true
    try {
      const { redirect_uri: target } = await completeForumSsoAuthorize({
        client_id: parsed.clientId,
        redirect_uri: parsed.redirectUri,
        state: parsed.state,
        scope: parsed.scope
      })

      if (!target) {
        throw new Error('authorize response did not include a redirect target')
      }

      // A full page load, not router.push: the forum is a separate origin and
      // the SPA router cannot navigate there.
      window.location.assign(target)
    } catch (error: unknown) {
      // Allow a retry, and drop the parameter so a stuck request cannot trap the
      // user in a resume loop on every subsequent navigation.
      consumed = false
      hasPendingResume.value = false
      isResuming.value = false

      const query = { ...route.query }
      delete query[FORUM_SSO_RESUME_PARAM]
      void router.replace({ path: route.path, query })

      onError?.(error instanceof Error ? error.message : String(error))
    }
  }

  watch(
    [isAuthenticated, () => route.query[FORUM_SSO_RESUME_PARAM]],
    ([authed, rawResume]) => {
      hasPendingResume.value = parseForumSsoResume(rawResume) !== null
      if (authed && hasPendingResume.value) {
        void resume()
      }
    },
    { immediate: true }
  )

  return { isResuming, hasPendingResume, resume }
}
