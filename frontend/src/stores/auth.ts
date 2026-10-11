/**
 * Authentication Store
 * Manages user authentication state, login/logout, token refresh, and token persistence
 */

import { defineStore } from 'pinia'
import { isAxiosError } from 'axios'
import { ref, computed, readonly, onScopeDispose } from 'vue'
import { authAPI, isTotp2FARequired, passkeyAPI, type LoginResponse } from '@/api'
import {
  getAuthSessionIdentity, isAuthSessionCurrent, invalidateAuthSession, subscribeAuthSession,
  AUTH_SESSION_KEY, startAuthSession, finishAuthSession, isAuthSessionPublishing,
  blockAuthSession, isAuthSessionBlocked, AuthRefreshPendingError, consumeOAuthPublication,
  beginInteractiveAuthAttempt, isInteractiveAuthAttemptCurrent, commitInteractiveAuth
} from '@/api/tokenRefresh'
import type {
  User,
  LoginRequest,
  RegisterRequest,
  AuthResponse,
  ActionCaptchaRequestProof
} from '@/types'

const AUTH_TOKEN_KEY = 'auth_token'
const AUTH_USER_KEY = 'auth_user'
const REFRESH_TOKEN_KEY = 'refresh_token'
const TOKEN_EXPIRES_AT_KEY = 'token_expires_at' // 存储过期时间戳而非有效期
const PENDING_AUTH_SESSION_KEY = 'pending_auth_session'
const AUTO_REFRESH_INTERVAL = 60 * 1000 // 60 seconds for user data refresh
const TOKEN_REFRESH_BUFFER = 120 * 1000 // 120 seconds before expiry to refresh token

type PendingAuthTokenField = 'pending_auth_token' | 'pending_oauth_token'

interface PendingAuthSessionSummary {
  token: string
  token_field: PendingAuthTokenField
  provider: string
  redirect?: string
  adoption_required?: boolean
  suggested_display_name?: string
  suggested_avatar_url?: string
}

function normalizePendingAuthTokenField(value: unknown): PendingAuthTokenField {
  return value === 'pending_oauth_token' ? 'pending_oauth_token' : 'pending_auth_token'
}

function getPersistedPendingAuthSession(): PendingAuthSessionSummary | null {
  const raw = localStorage.getItem(PENDING_AUTH_SESSION_KEY)
  if (!raw) {
    return null
  }

  try {
    const parsed = JSON.parse(raw) as Partial<PendingAuthSessionSummary> | null
    const provider = typeof parsed?.provider === 'string' ? parsed.provider.trim() : ''
    if (!provider) {
      localStorage.removeItem(PENDING_AUTH_SESSION_KEY)
      return null
    }
    return {
      token: typeof parsed?.token === 'string' ? parsed.token : '',
      token_field: normalizePendingAuthTokenField(parsed?.token_field),
      provider,
      redirect: typeof parsed?.redirect === 'string' ? parsed.redirect : undefined,
      adoption_required: typeof parsed?.adoption_required === 'boolean' ? parsed.adoption_required : undefined,
      suggested_display_name: typeof parsed?.suggested_display_name === 'string' ? parsed.suggested_display_name : undefined,
      suggested_avatar_url: typeof parsed?.suggested_avatar_url === 'string' ? parsed.suggested_avatar_url : undefined
    }
  } catch {
    localStorage.removeItem(PENDING_AUTH_SESSION_KEY)
    return null
  }
}

function persistPendingAuthSession(session: PendingAuthSessionSummary): void {
  localStorage.setItem(PENDING_AUTH_SESSION_KEY, JSON.stringify(session))
}

function clearPendingAuthSessionStorage(): void {
  localStorage.removeItem(PENDING_AUTH_SESSION_KEY)
}

export const useAuthStore = defineStore('auth', () => {
  // ==================== State ====================

  const user = ref<User | null>(null)
  const token = ref<string | null>(null)
  const refreshTokenValue = ref<string | null>(null)
  const tokenExpiresAt = ref<number | null>(null) // 过期时间戳（毫秒）
  const runMode = ref<'standard' | 'simple'>('standard')
  const pendingAuthSession = ref<PendingAuthSessionSummary | null>(null)
  let refreshIntervalId: ReturnType<typeof setInterval> | null = null
  let tokenRefreshTimeoutId: ReturnType<typeof setTimeout> | null = null
  let sessionGeneration = 0
  let storedSessionID = getAuthSessionIdentity().sessionID
  let initialized = false
  let reloadRequested = false

  function reloadWhenSessionReady(): void {
    if (!isAuthSessionPublishing() && !reloadRequested) {
      reloadRequested = true
      window.location.reload()
    }
  }

  const unsubscribeSession = subscribeAuthSession(() => {
    if (initialized) syncStoredTokensBeforeRefresh()
  })
  const onStorage = (event: StorageEvent) => {
    if (event.storageArea === localStorage && (event.key === null ||
      [AUTH_SESSION_KEY, AUTH_TOKEN_KEY, AUTH_USER_KEY, REFRESH_TOKEN_KEY, TOKEN_EXPIRES_AT_KEY].includes(event.key))) {
      if (initialized) syncStoredTokensBeforeRefresh()
    }
  }
  window.addEventListener('storage', onStorage)
  onScopeDispose(() => {
    unsubscribeSession()
    window.removeEventListener('storage', onStorage)
    stopAutoRefresh()
    stopTokenRefresh()
  })

  // ==================== Computed ====================

  const isAuthenticated = computed(() => {
    return !!token.value && !!user.value
  })

  const isAdmin = computed(() => {
    return user.value?.role === 'admin'
  })

  const isSimpleMode = computed(() => runMode.value === 'simple')
  const hasPendingAuthSession = computed(() => pendingAuthSession.value !== null)

  // ==================== Actions ====================

  /**
   * Initialize auth state from localStorage
   * Call this on app startup to restore session
   * Also starts auto-refresh and immediately fetches latest user data
   */
  function checkAuth(): void {
    initialized = true
    storedSessionID = getAuthSessionIdentity().sessionID
    if (isAuthSessionPublishing()) {
      blockAuthSession()
      return
    }
    const savedToken = localStorage.getItem(AUTH_TOKEN_KEY)
    const savedUser = localStorage.getItem(AUTH_USER_KEY)
    const savedRefreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
    const savedExpiresAt = localStorage.getItem(TOKEN_EXPIRES_AT_KEY)
    pendingAuthSession.value = getPersistedPendingAuthSession()

    if (savedToken && savedUser) {
      try {
        const savedUserData = JSON.parse(savedUser)
        if (user.value?.id !== savedUserData.id) {
          sessionGeneration += 1
          invalidateAuthSession()
        }
        token.value = savedToken
        user.value = savedUserData
        refreshTokenValue.value = savedRefreshToken
        tokenExpiresAt.value = savedExpiresAt ? parseInt(savedExpiresAt, 10) : null

        // Immediately refresh user data from backend (async, don't block)
        refreshUser().catch((error) => {
          console.error('Failed to refresh user on init:', error)
        })

        // Start auto-refresh interval for user data
        startAutoRefresh()

        // Start proactive token refresh if we have refresh token and expiry info
        // Note: use !== null to handle case when tokenExpiresAt.value is 0 (expired)
        if (savedRefreshToken && tokenExpiresAt.value !== null) {
          scheduleTokenRefreshAt(tokenExpiresAt.value)
        }
      } catch (error) {
        console.error('Failed to parse saved user data:', error)
        clearAuth({ preservePendingAuthSession: true })
      }
    }
  }

  /**
   * Start auto-refresh interval for user data
   * Refreshes user data every 60 seconds
   */
  function startAutoRefresh(): void {
    // Clear existing interval if any
    stopAutoRefresh()

    refreshIntervalId = setInterval(() => {
      if (token.value) {
        refreshUser().catch((error) => {
          console.error('Auto-refresh user failed:', error)
        })
      }
    }, AUTO_REFRESH_INTERVAL)
  }

  /**
   * Stop auto-refresh interval
   */
  function stopAutoRefresh(): void {
    if (refreshIntervalId) {
      clearInterval(refreshIntervalId)
      refreshIntervalId = null
    }
  }

  /**
   * Schedule proactive token refresh before expiry (based on expiry timestamp)
   * @param expiresAtMs - Token expiry timestamp in milliseconds
   */
  function scheduleTokenRefreshAt(expiresAtMs: number): void {
    // Clear any existing timeout
    if (tokenRefreshTimeoutId) {
      clearTimeout(tokenRefreshTimeoutId)
      tokenRefreshTimeoutId = null
    }

    // Calculate remaining time until refresh (buffer time before expiry)
    const now = Date.now()
    const refreshInMs = Math.max(0, expiresAtMs - now - TOKEN_REFRESH_BUFFER)

    if (refreshInMs <= 0) {
      // Token is about to expire or already expired, refresh immediately
      performTokenRefresh()
      return
    }

    tokenRefreshTimeoutId = setTimeout(() => {
      performTokenRefresh()
    }, refreshInMs)
  }

  /**
   * Schedule proactive token refresh before expiry (based on expires_in seconds)
   * @param expiresInSeconds - Token expiry time in seconds from now
   */
  function scheduleTokenRefresh(expiresInSeconds: number): void {
    const expiresAtMs = Date.now() + expiresInSeconds * 1000
    tokenExpiresAt.value = expiresAtMs
    localStorage.setItem(TOKEN_EXPIRES_AT_KEY, String(expiresAtMs))
    scheduleTokenRefreshAt(expiresAtMs)
  }

  function syncStoredTokensBeforeRefresh(): boolean {
    if (isAuthSessionBlocked()) {
      reloadWhenSessionReady()
      return true
    }
    const storedIdentity = getAuthSessionIdentity()
    if (storedIdentity.sessionID !== storedSessionID ||
      (user.value && storedIdentity.userID !== user.value.id)) {
      // Retire this document before adopting a peer session. Do not erase the peer's storage
      // or reuse old page state / idempotency ownership under its credentials.
      clearAuth({ preservePendingAuthSession: true, preserveStoredSession: true })
      blockAuthSession()
      reloadWhenSessionReady()
      return true
    }
    // A peer has not committed all credential fields yet. Keep the last complete
    // in-memory pair; the ready storage event will synchronize it.
    if (isAuthSessionPublishing()) return true
    const storedAccessToken = localStorage.getItem(AUTH_TOKEN_KEY)
    const storedRefreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
    const storedExpiresAt = localStorage.getItem(TOKEN_EXPIRES_AT_KEY)
    const expiresAtMs = storedExpiresAt ? Number.parseInt(storedExpiresAt, 10) : Number.NaN
    if (storedAccessToken === token.value && storedRefreshToken === refreshTokenValue.value) {
      // A peer may have updated the store before an already-queued timer runs.
      if (storedAccessToken && storedRefreshToken && Number.isFinite(expiresAtMs) &&
        expiresAtMs > Date.now() + TOKEN_REFRESH_BUFFER) {
        tokenExpiresAt.value = expiresAtMs
        scheduleTokenRefreshAt(expiresAtMs)
        return true
      }
      return false
    }

    if (!storedAccessToken || !storedRefreshToken) {
      clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      return true
    }

    token.value = storedAccessToken
    refreshTokenValue.value = storedRefreshToken

    if (Number.isFinite(expiresAtMs)) {
      tokenExpiresAt.value = expiresAtMs
      if (expiresAtMs > Date.now() + TOKEN_REFRESH_BUFFER) {
        scheduleTokenRefreshAt(expiresAtMs)
        return true
      }
    }

    return false
  }

  /**
   * Perform the actual token refresh
   */
  async function performTokenRefresh(): Promise<void> {
    if (syncStoredTokensBeforeRefresh()) {
      return
    }

    const refreshTokenAtStart = refreshTokenValue.value
    const accessTokenAtStart = token.value
    const generationAtStart = sessionGeneration
    const identityAtStart = getAuthSessionIdentity()
    if (!refreshTokenAtStart || !accessTokenAtStart) {
      return
    }

    try {
      const response = await authAPI.refreshToken(refreshTokenAtStart)

      // A response from a previous account must never replace the current session.
      if (
        generationAtStart !== sessionGeneration ||
        token.value !== accessTokenAtStart ||
        refreshTokenValue.value !== refreshTokenAtStart ||
        localStorage.getItem(REFRESH_TOKEN_KEY) !== refreshTokenAtStart
      ) {
        return
      }

      // Update state
      token.value = response.access_token
      refreshTokenValue.value = response.refresh_token
      localStorage.setItem(AUTH_TOKEN_KEY, response.access_token)
      localStorage.setItem(REFRESH_TOKEN_KEY, response.refresh_token)

      // Schedule next refresh (this also updates tokenExpiresAt and localStorage)
      scheduleTokenRefresh(response.expires_in)
    } catch (error) {
      console.error('Token refresh failed:', error)
      if (
        generationAtStart !== sessionGeneration || !isAuthSessionCurrent(identityAtStart) ||
        localStorage.getItem(REFRESH_TOKEN_KEY) !== refreshTokenAtStart
      ) {
        return
      }
      // Raw refresh HTTP bypasses the API interceptor. Apply the same expiry policy here.
      if (error instanceof AuthRefreshPendingError) return
      if (isAxiosError(error)) {
        const status = error.response?.status ?? 0
        if (status === 0 || status === 429 || status >= 500) return
      }
      clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      sessionStorage.setItem('auth_expired', '1')
      if (!window.location.pathname.includes('/login')) window.location.href = '/login'
    }
  }

  /**
   * Stop token refresh timeout
   */
  function stopTokenRefresh(): void {
    if (tokenRefreshTimeoutId) {
      clearTimeout(tokenRefreshTimeoutId)
      tokenRefreshTimeoutId = null
    }
  }

  /**
   * User login
   * @param credentials - Login credentials (email and password)
   * @returns Promise resolving to the login response (may require 2FA)
   * @throws Error if login fails
   */
  async function login(credentials: LoginRequest): Promise<LoginResponse> {
    const attempt = beginInteractiveAuthAttempt()
    try {
      const response = await authAPI.login(credentials, attempt)
      return commitInteractiveAuth(attempt, () => {
        // A current 2FA challenge is returned without publishing credentials.
        if (!isTotp2FARequired(response)) setAuthFromResponse(response)
        return response
      })
    } catch (error) {
      if (isInteractiveAuthAttemptCurrent(attempt)) {
        clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      }
      throw error
    }
  }

  /**
   * Complete login with 2FA code
   * @param tempToken - Temporary token from initial login
   * @param totpCode - 6-digit TOTP code
   * @returns Promise resolving to the authenticated user
   * @throws Error if 2FA verification fails
   */
  async function login2FA(tempToken: string, totpCode: string): Promise<User> {
    const attempt = beginInteractiveAuthAttempt()
    try {
      const response = await authAPI.login2FA({ temp_token: tempToken, totp_code: totpCode }, attempt)
      return commitInteractiveAuth(attempt, () => {
        setAuthFromResponse(response)
        return user.value!
      })
    } catch (error) {
      if (isInteractiveAuthAttemptCurrent(attempt)) {
        clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      }
      throw error
    }
  }

  async function loginWithPasskey(proof?: ActionCaptchaRequestProof): Promise<User> {
    try {
      const response = await passkeyAPI.login(proof)
      setAuthFromResponse(response)
      return user.value!
    } catch (error) {
      clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      throw error
    }
  }

  /**
   * Set auth state from an AuthResponse
   * Internal helper function
   */
  function setAuthFromResponse(response: AuthResponse): void {
    stopAutoRefresh()
    stopTokenRefresh()
    sessionGeneration += 1
    const publication = startAuthSession()
    initialized = true
    storedSessionID = getAuthSessionIdentity().sessionID

    // Store token and user
    token.value = response.access_token

    // Store refresh token if present
    if (response.refresh_token) {
      refreshTokenValue.value = response.refresh_token
      localStorage.setItem(REFRESH_TOKEN_KEY, response.refresh_token)
    }

    // Extract run_mode if present
    if (response.user.run_mode) {
      runMode.value = response.user.run_mode
    }
    const { run_mode: _run_mode, ...userData } = response.user
    user.value = userData

    // Persist to localStorage
    localStorage.setItem(AUTH_TOKEN_KEY, response.access_token)
    localStorage.setItem(AUTH_USER_KEY, JSON.stringify(userData))
    if (response.refresh_token && response.expires_in) {
      tokenExpiresAt.value = Date.now() + response.expires_in * 1000
      localStorage.setItem(TOKEN_EXPIRES_AT_KEY, String(tokenExpiresAt.value))
    }
    finishAuthSession(publication)
    clearPendingAuthSession()

    // Start auto-refresh interval for user data
    startAutoRefresh()

    // Start proactive token refresh if we have refresh token and expiry info
    // scheduleTokenRefresh will also store the expiry timestamp
    if (response.refresh_token && response.expires_in) {
      scheduleTokenRefreshAt(tokenExpiresAt.value!)
    }
  }

  /**
   * User registration
   * @param userData - Registration data (username, email, password)
   * @returns Promise resolving to the newly registered and authenticated user
   * @throws Error if registration fails
   */
  async function register(userData: RegisterRequest): Promise<User> {
    const attempt = beginInteractiveAuthAttempt()
    try {
      const response = await authAPI.register(userData, attempt)
      return commitInteractiveAuth(attempt, () => {
        setAuthFromResponse(response)
        return user.value!
      })
    } catch (error) {
      if (isInteractiveAuthAttemptCurrent(attempt)) {
        clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      }
      throw error
    }
  }

  /**
   * 直接设置 token（用于 OAuth/SSO 回调），并加载当前用户信息。
   * 会自动读取 localStorage 中已设置的 refresh_token 和 token_expires_in
   * @param newToken - 后端签发的 JWT access token
   */
  async function setToken(newToken: string): Promise<User> {
    // Clear any previous state first (avoid mixing sessions)
    // Note: Don't clear localStorage here as OAuth callback may have set refresh_token
    stopAutoRefresh()
    stopTokenRefresh()
    sessionGeneration += 1
    const publication = consumeOAuthPublication() ?? startAuthSession()
    initialized = true
    storedSessionID = getAuthSessionIdentity().sessionID
    token.value = null
    user.value = null

    token.value = newToken
    localStorage.setItem(AUTH_TOKEN_KEY, newToken)

    // Read refresh token and expires_at from localStorage if set by OAuth callback
    const savedRefreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
    const savedExpiresAt = localStorage.getItem(TOKEN_EXPIRES_AT_KEY)

    if (savedRefreshToken) {
      refreshTokenValue.value = savedRefreshToken
    }
    if (savedExpiresAt) {
      tokenExpiresAt.value = parseInt(savedExpiresAt, 10)
    }

    try {
      const userData = await refreshUser()
      if (!finishAuthSession(publication)) throw new Error('Authentication session changed while publishing')
      startAutoRefresh()

      // Start proactive token refresh if we have refresh token and expiry info
      // Note: use !== null to handle case when tokenExpiresAt.value is 0 (expired)
      if (savedRefreshToken && tokenExpiresAt.value !== null) {
        scheduleTokenRefreshAt(tokenExpiresAt.value)
      }

      clearPendingAuthSession()
      return userData
    } catch (error) {
      if (localStorage.getItem(AUTH_SESSION_KEY) === publication) {
        clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      }
      throw error
    }
  }

  function setPendingAuthSession(session: PendingAuthSessionSummary | null): void {
    pendingAuthSession.value = session

    if (session) {
      persistPendingAuthSession(session)
      return
    }

    clearPendingAuthSessionStorage()
  }

  function clearPendingAuthSession(): void {
    setPendingAuthSession(null)
  }

  /**
   * User logout
   * Clears all authentication state and persisted data
   */
  async function logout(): Promise<void> {
    const refreshTokenToRevoke = refreshTokenValue.value
    // Invalidate the local session before awaiting the network. This prevents an
    // in-flight refresh for the old account from being committed meanwhile.
    clearAuth()
    try {
      // Call API logout (revokes refresh token on server)
      await authAPI.logout(refreshTokenToRevoke)
    } catch (err) {
      // 服务端吊销失败（网络/5xx/超时）不应阻止本地登出，否则用户点了退出仍处于登录态。
      console.warn('Logout API call failed; local session was already cleared', err)
    }
  }

  /**
   * Refresh current user data
   * Fetches latest user info from the server
   * @returns Promise resolving to the updated user
   * @throws Error if not authenticated or request fails
   */
  async function refreshUser(): Promise<User> {
    const accessTokenAtStart = token.value
    const generationAtStart = sessionGeneration
    const identityAtStart = getAuthSessionIdentity()
    const userIDAtStart = user.value?.id
    if (!accessTokenAtStart) {
      throw new Error('Not authenticated')
    }

    try {
      const response = await authAPI.getCurrentUser()
      if (
        generationAtStart !== sessionGeneration || !isAuthSessionCurrent(identityAtStart) ||
        user.value?.id !== userIDAtStart ||
        (userIDAtStart !== undefined && response.data.id !== userIDAtStart)
      ) {
        throw new Error('Authentication session changed while refreshing user')
      }
      if (response.data.run_mode) {
        runMode.value = response.data.run_mode
      }
      const { run_mode: _run_mode, ...userData } = response.data
      user.value = userData

      // Update localStorage
      localStorage.setItem(AUTH_USER_KEY, JSON.stringify(userData))

      return userData
    } catch (error) {
      // If refresh fails with 401, clear auth state
      if (
        (error as { status?: number }).status === 401 &&
        generationAtStart === sessionGeneration && isAuthSessionCurrent(identityAtStart)
      ) {
        clearAuth({ preservePendingAuthSession: pendingAuthSession.value !== null })
      }
      throw error
    }
  }

  /**
   * Clear all authentication state
   * Internal helper function
   */
  function clearAuth(options?: { preservePendingAuthSession?: boolean, preserveStoredSession?: boolean }): void {
    sessionGeneration += 1
    if (options?.preserveStoredSession) invalidateAuthSession()
    const publication = options?.preserveStoredSession ? null : startAuthSession()
    initialized = true
    storedSessionID = getAuthSessionIdentity().sessionID
    // Stop auto-refresh
    stopAutoRefresh()
    // Stop token refresh
    stopTokenRefresh()

    token.value = null
    refreshTokenValue.value = null
    tokenExpiresAt.value = null
    user.value = null
    if (!options?.preserveStoredSession) {
      localStorage.removeItem(AUTH_TOKEN_KEY)
      localStorage.removeItem(AUTH_USER_KEY)
      localStorage.removeItem(REFRESH_TOKEN_KEY)
      localStorage.removeItem(TOKEN_EXPIRES_AT_KEY)
      finishAuthSession(publication!)
    }

    if (options?.preservePendingAuthSession) {
      pendingAuthSession.value = getPersistedPendingAuthSession()
      return
    }

    pendingAuthSession.value = null
    clearPendingAuthSessionStorage()
  }

  // ==================== Return Store API ====================

  return {
    // State
    user,
    token,
    runMode: readonly(runMode),
    pendingAuthSession: readonly(pendingAuthSession),

    // Computed
    isAuthenticated,
    isAdmin,
    isSimpleMode,
    hasPendingAuthSession,

    // Actions
    login,
    loginWithPasskey,
    login2FA,
    register,
    setToken,
    logout,
    checkAuth,
    refreshUser,
    setPendingAuthSession,
    clearPendingAuthSession
  }
})
