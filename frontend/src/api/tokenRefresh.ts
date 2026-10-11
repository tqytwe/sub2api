import axios from 'axios'
import type { ApiResponse } from '@/types'
import { getAPIBaseURL } from './url'

const AUTH_TOKEN_KEY = 'auth_token'
const AUTH_USER_KEY = 'auth_user'
const REFRESH_TOKEN_KEY = 'refresh_token'
const TOKEN_EXPIRES_AT_KEY = 'token_expires_at'
export const AUTH_SESSION_KEY = 'auth_session_id'
const SESSION_PENDING_PREFIX = 'pending:'
const TOKEN_REFRESH_LOCK_NAME = 'sub2api-auth-token-refresh'
const TOKEN_REFRESH_TIMEOUT_MS = 30_000
const PEER_REFRESH_WAIT_MS = 1_000
const PEER_REFRESH_GRACE_MS = 1_000
const PEER_REFRESH_POLL_MS = 25

export interface RefreshTokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
}

export interface RefreshAuthTokensOptions {
  /** Access token attached to the request that received a 401 response. */
  failedAccessToken?: string | null
}

export interface AuthSessionIdentity {
  generation: number
  userID: number | null
  sessionID: string | null
}

interface AuthSnapshot extends AuthSessionIdentity {
  accessToken: string | null
  refreshToken: string
  expiresAt: number
  publishing: boolean
}

let inFlightRefresh: Promise<RefreshTokenResponse> | null = null
let sessionGeneration = 0
let sessionBlocked = false
let ownedPublication: string | null = null
let pendingOAuthPublication: string | null = null
const sessionListeners = new Set<() => void>()

export function getAuthSessionIdentity(): AuthSessionIdentity {
  const marker = localStorage.getItem(AUTH_SESSION_KEY)
  const sessionID = marker?.startsWith(SESSION_PENDING_PREFIX) ? marker.slice(SESSION_PENDING_PREFIX.length).split('|')[0] || null : marker
  return { generation: sessionGeneration, userID: getStoredUserID(), sessionID }
}

export function isAuthSessionCurrent(identity: AuthSessionIdentity): boolean {
  return identity.generation === sessionGeneration && identity.userID === getStoredUserID() &&
    identity.sessionID === getAuthSessionIdentity().sessionID
}

export function isAuthSessionPublishing(): boolean {
  return localStorage.getItem(AUTH_SESSION_KEY)?.startsWith(SESSION_PENDING_PREFIX) ?? false
}

export class AuthRefreshPendingError extends Error {
  constructor() { super('Authentication update is still being published') }
}

export function isOwnAuthSessionPublication(): boolean {
  return ownedPublication !== null && localStorage.getItem(AUTH_SESSION_KEY) === ownedPublication
}

function newSessionID(): string {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`
}

function beginAuthPublication(sessionID: string | null): string {
  const marker = SESSION_PENDING_PREFIX + (sessionID ?? '') + '|' + newSessionID()
  ownedPublication = marker
  localStorage.setItem(AUTH_SESSION_KEY, marker)
  return marker
}

export function finishAuthSession(marker: string): boolean {
  if (localStorage.getItem(AUTH_SESSION_KEY) !== marker) return false
  const sessionID = getAuthSessionIdentity().sessionID
  if (sessionID) localStorage.setItem(AUTH_SESSION_KEY, sessionID)
  else localStorage.removeItem(AUTH_SESSION_KEY)
  if (ownedPublication === marker) ownedPublication = null
  if (pendingOAuthPublication === marker) pendingOAuthPublication = null
  return true
}

// Non-credential lifecycle marker: a same-account login differs from token rotation.
export function startAuthSession(kind: 'login' | 'oauth' = 'login'): string {
  const marker = beginAuthPublication(newSessionID())
  pendingOAuthPublication = kind === 'oauth' ? marker : null
  sessionBlocked = false
  invalidateAuthSession()
  return marker
}

// OAuth callbacks persist refresh context immediately before setToken loads the profile.
// Transfer only this document's publication once; no credential is copied into metadata.
export function consumeOAuthPublication(): string | null {
  const marker = pendingOAuthPublication
  pendingOAuthPublication = null
  if (marker === null || marker !== localStorage.getItem(AUTH_SESSION_KEY)) return null
  sessionBlocked = false
  return marker
}

export function blockAuthSession(): void { sessionBlocked = true }
export function isAuthSessionBlocked(): boolean { return sessionBlocked }

// Login/logout starts a new session; token rotation within that session does not.
export function invalidateAuthSession(): void {
  sessionGeneration += 1
  inFlightRefresh = null
}

export function subscribeAuthSession(listener: () => void): () => void {
  sessionListeners.add(listener)
  return () => { sessionListeners.delete(listener) }
}

// Same-document writes do not emit the browser's storage event. Publish no credentials.
export function notifyAuthSessionChanged(): void {
  sessionListeners.forEach(listener => listener())
}

function getStoredUserID(): number | null {
  const rawUser = localStorage.getItem(AUTH_USER_KEY)
  if (!rawUser) {
    return null
  }

  try {
    const id = Number((JSON.parse(rawUser) as { id?: unknown }).id)
    return Number.isFinite(id) && id > 0 ? id : null
  } catch {
    return null
  }
}

function readAuthSnapshot(): AuthSnapshot {
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
  if (!refreshToken) {
    throw new Error('No refresh token available')
  }

  return {
    ...getAuthSessionIdentity(),
    accessToken: localStorage.getItem(AUTH_TOKEN_KEY),
    refreshToken,
    expiresAt: Number(localStorage.getItem(TOKEN_EXPIRES_AT_KEY)),
    publishing: isAuthSessionPublishing(),
  }
}

function readStoredTokenPair(snapshot: AuthSnapshot): RefreshTokenResponse | null {
  const accessToken = localStorage.getItem(AUTH_TOKEN_KEY)
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
  const expiresAt = Number(localStorage.getItem(TOKEN_EXPIRES_AT_KEY))

  if (
    !accessToken ||
    !refreshToken ||
    !Number.isFinite(expiresAt) ||
    expiresAt <= Date.now() ||
    isAuthSessionPublishing() ||
    !isAuthSessionCurrent(snapshot)
  ) {
    return null
  }

  return {
    access_token: accessToken,
    refresh_token: refreshToken,
    expires_in: Math.max(1, Math.ceil((expiresAt - Date.now()) / 1000)),
    token_type: 'Bearer'
  }
}

function readPeerRefreshResult(
  snapshot: AuthSnapshot,
  failedAccessToken?: string | null
): RefreshTokenResponse | null {
  const storedPair = readStoredTokenPair(snapshot)
  if (!storedPair) {
    return null
  }

  if (storedPair.refresh_token !== snapshot.refreshToken) {
    return storedPair
  }

  if (
    failedAccessToken &&
    snapshot.accessToken !== failedAccessToken &&
    storedPair.access_token === snapshot.accessToken
  ) {
    return storedPair
  }

  return null
}

async function waitForPeerRefresh(
  snapshot: AuthSnapshot,
  failedAccessToken?: string | null,
  deadline = Date.now() + PEER_REFRESH_WAIT_MS
): Promise<RefreshTokenResponse | null> {
  while (Date.now() < deadline) {
    const peerResult = readPeerRefreshResult(snapshot, failedAccessToken)
    if (peerResult) {
      return peerResult
    }
    await new Promise((resolve) => window.setTimeout(resolve, PEER_REFRESH_POLL_MS))
  }

  return readPeerRefreshResult(snapshot, failedAccessToken)
}

function persistTokenPair(tokens: RefreshTokenResponse): void {
  // The non-credential marker brackets the multi-key write. A peer renderer can
  // acquire the Web Lock before it has observed every localStorage write.
  const marker = beginAuthPublication(getAuthSessionIdentity().sessionID)
  localStorage.setItem(AUTH_TOKEN_KEY, tokens.access_token)
  localStorage.setItem(TOKEN_EXPIRES_AT_KEY, String(Date.now() + tokens.expires_in * 1000))
  // Publish the ready marker only after the complete pair and expiry have been written.
  localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token)
  finishAuthSession(marker)
}

async function requestTokenPair(
  snapshot: AuthSnapshot,
  failedAccessToken?: string | null,
  mayHaveUncoordinatedPeer = false
): Promise<RefreshTokenResponse> {
  // If this request loses a one-time-token race, the winning peer can legitimately take as long
  // as our own HTTP timeout to publish its replacement token. Keep the recovery window tied to
  // that timeout instead of an arbitrary short delay.
  const peerRefreshDeadline = Date.now() + TOKEN_REFRESH_TIMEOUT_MS + PEER_REFRESH_GRACE_MS

  try {
    const response = await axios.post<ApiResponse<RefreshTokenResponse>>(
      `${getAPIBaseURL()}/auth/refresh`,
      { refresh_token: snapshot.refreshToken },
      { headers: { 'Content-Type': 'application/json' }, timeout: TOKEN_REFRESH_TIMEOUT_MS }
    )
    const payload = response.data
    if (payload.code !== 0 || !payload.data) {
      throw new Error(payload.message || 'Token refresh failed')
    }

    if (
      localStorage.getItem(REFRESH_TOKEN_KEY) !== snapshot.refreshToken ||
      !isAuthSessionCurrent(snapshot)
    ) {
      const peerResult = readPeerRefreshResult(snapshot, failedAccessToken)
      if (peerResult) {
        return peerResult
      }
      throw new Error('Session changed during token refresh')
    }

    persistTokenPair(payload.data)
    return payload.data
  } catch (error) {
    // A peer tab may have rotated the one-time refresh token while this request was in flight.
    // A 4xx response can arrive quickly while the winning peer's response is still in flight, so
    // wait through the shared request deadline before treating the session as expired. Transient
    // non-4xx failures retain the short reconciliation window.
    const responseStatus = (error as { response?: { status?: unknown } }).response?.status
    const isTokenRejection =
      typeof responseStatus === 'number' && responseStatus >= 400 && responseStatus < 500
    const peerResult = await waitForPeerRefresh(
      snapshot,
      failedAccessToken,
      isTokenRejection && mayHaveUncoordinatedPeer
        ? peerRefreshDeadline
        : Date.now() + PEER_REFRESH_WAIT_MS
    )
    if (peerResult) {
      return peerResult
    }
    throw error
  }
}

async function runRefresh(options: RefreshAuthTokensOptions): Promise<RefreshTokenResponse> {
  const snapshot = readAuthSnapshot()
  const refresh = async (mayHaveUncoordinatedPeer = false): Promise<RefreshTokenResponse> => {
    if (!isAuthSessionCurrent(snapshot)) {
      throw new Error('Session changed during token refresh')
    }
    if (snapshot.publishing || isAuthSessionPublishing()) {
      const deadline = Date.now() + PEER_REFRESH_WAIT_MS
      while (isAuthSessionCurrent(snapshot) && isAuthSessionPublishing() && Date.now() < deadline) {
        await new Promise(resolve => window.setTimeout(resolve, PEER_REFRESH_POLL_MS))
      }
      const committed = readStoredTokenPair(snapshot)
      if (committed) return committed
      throw new AuthRefreshPendingError()
    }
    const peerResult = readPeerRefreshResult(snapshot, options.failedAccessToken)
    if (peerResult) {
      return peerResult
    }
    return requestTokenPair(snapshot, options.failedAccessToken, mayHaveUncoordinatedPeer)
  }

  if (typeof navigator !== 'undefined' && navigator.locks) {
    return navigator.locks.request(TOKEN_REFRESH_LOCK_NAME, { ifAvailable: true }, lock => {
      if (lock) return refresh(false)
      return navigator.locks.request(TOKEN_REFRESH_LOCK_NAME, async () => {
        // A queued lock can arrive before this renderer has received any of the
        // winner's storage updates. Wait for its commit, including the all-old cache case.
        const peer = await waitForPeerRefresh(snapshot, options.failedAccessToken)
        return peer ?? refresh(false)
      })
    })
  }

  return refresh(true)
}

/**
 * Refresh and persist the browser session.
 *
 * Calls in the same document share one promise. Web Locks serialize refreshes across tabs, while
 * the token snapshot check adopts a peer's newly rotated token instead of logging the user out.
 */
export function refreshAuthTokens(
  options: RefreshAuthTokensOptions = {}
): Promise<RefreshTokenResponse> {
  if (inFlightRefresh) {
    return inFlightRefresh
  }

  const pending = runRefresh(options).then(tokens => {
    notifyAuthSessionChanged()
    return tokens
  })
  inFlightRefresh = pending
  const clearPending = (): void => {
    if (inFlightRefresh === pending) {
      inFlightRefresh = null
    }
  }
  void pending.then(clearPending, clearPending)
  return pending
}
