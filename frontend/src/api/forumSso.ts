/**
 * Forum SSO API endpoints
 *
 * The community forum (NodeBB) authenticates against this platform using the
 * OAuth2 authorization code flow. When the forum sends a user to
 * GET /api/v1/sso/oauth/authorize and that user has no panel session, the
 * backend bounces them to the login page with a `forum_sso_resume` query
 * parameter holding the original authorize request URI.
 *
 * After the user logs in, the SPA replays that request through this endpoint,
 * which mints an authorization code and returns where to navigate next. Unlike
 * the protocol endpoints consumed by the forum itself, this one speaks the
 * platform's standard {code,message,data} envelope because our own frontend is
 * the only caller.
 */

import { apiClient } from './client'

export interface ForumSsoAuthorizeRequest {
  client_id: string
  redirect_uri: string
  state?: string
  scope?: string
}

export interface ForumSsoAuthorizeResponse {
  /** Absolute forum callback URL, already carrying `code` and `state`. */
  redirect_uri: string
}

/**
 * Completes a pending forum authorize request for the logged-in user.
 *
 * Requires a valid panel session; the backend derives the user from the JWT
 * rather than trusting anything in the payload.
 */
export async function completeForumSsoAuthorize(
  request: ForumSsoAuthorizeRequest
): Promise<ForumSsoAuthorizeResponse> {
  const { data } = await apiClient.post<ForumSsoAuthorizeResponse>(
    '/sso/oauth/authorize',
    request
  )
  return data
}
