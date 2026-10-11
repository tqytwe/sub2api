// Real Axios transport, token coordinator and Pinia. All identities are local fixtures.
import { createServer, type Server, type ServerResponse } from 'node:http'
import axios from 'axios'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import apiClient from '@/api/client'
import { persistOAuthTokenContext } from '@/api/auth'
import { refreshAuthTokens, AuthRefreshPendingError, startAuthSession, finishAuthSession } from '@/api/tokenRefresh'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/i18n', () => ({ getLocale: () => 'zh-CN' }))

const user = (id = 1, balance = 1) => ({ id, email: `fixture-${id}@example.test`, role: 'user', status: 'active', balance, concurrency: 5 })
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawOriginal = { baseURL: axios.defaults.baseURL, adapter: axios.defaults.adapter }
const originalLocks = Object.getOwnPropertyDescriptor(navigator, 'locks')
let server: Server
let auth: ReturnType<typeof useAuthStore>
let pinia: ReturnType<typeof createPinia>
let loginNumber = 0
let refreshCount = 0
let expiresIn = 3600
let refreshedExpiresIn = 3600
let holdRefresh = false
let refreshStatus = 200
let refreshResponse: ServerResponse | undefined
let refreshOwner = 1
let lateResponse: ServerResponse | undefined
let holdProfile = false
let profileResponse: ServerResponse | undefined
let profileBalance = 1
let accessOwner: Map<string, number>
let refreshOwners: Map<string, number>
let rejectedAccess: Set<string>
let requests: Array<{ path: string, token: string }>
const reply = (res: ServerResponse, data: unknown, status = 200) => {
  res.writeHead(status, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify({ code: status === 200 ? 0 : status, data }))
}
function tokenPair(id: number, suffix: string, lifetime = 3600) {
  const access = `fixture-${id}-access-${suffix}`
  const refresh = `fixture-${id}-refresh-${suffix}`
  accessOwner.set(access, id); refreshOwners.set(refresh, id)
  return { access_token: access, refresh_token: refresh, expires_in: lifetime, token_type: 'Bearer' }
}
function finishRefresh(status = refreshStatus) {
  if (!refreshResponse) throw new Error('No HTTP refresh response to release')
  reply(refreshResponse, tokenPair(refreshOwner, `renewed-${refreshCount}`, refreshedExpiresIn), status)
  refreshResponse = undefined
}
beforeAll(async () => {
  server = createServer(async (req, res) => {
    const path = req.url?.split('?')[0] || ''
    const token = req.headers.authorization?.replace(/^Bearer /, '') || ''
    requests.push({ path, token })
    let raw = ''; for await (const chunk of req) raw += chunk
    const body = raw ? JSON.parse(raw) : {}
    if (path === '/api/v1/auth/login') {
      const id = body.email === 'fixture-2@example.test' ? 2 : 1
      reply(res, { ...tokenPair(id, `login-${++loginNumber}`, expiresIn), user: user(id) })
    } else if (path === '/api/v1/auth/refresh') {
      refreshCount++; refreshOwner = refreshOwners.get(body.refresh_token) || 0
      refreshResponse = res
      if (!holdRefresh) finishRefresh()
    } else if (path === '/api/v1/auth/logout') reply(res, {})
    else if (path === '/api/v1/fixture/late' && requests.filter(r => r.path === path).length === 1) lateResponse = res
    else if (rejectedAccess.has(token)) reply(res, {}, 401)
    else if (path === '/api/v1/auth/me') {
      if (holdProfile) profileResponse = res
      else reply(res, user(accessOwner.get(token), profileBalance))
    } else if (path.startsWith('/api/v1/fixture/')) reply(res, { owner: accessOwner.get(token) })
    else reply(res, {}, 404)
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing HTTP fixture port')
  const origin = `http://127.0.0.1:${address.port}`
  Object.assign(apiClient.defaults, { baseURL: origin + '/api/v1', adapter: 'http' })
  Object.assign(axios.defaults, { baseURL: origin, adapter: 'http' })
})
beforeEach(() => {
  localStorage.clear(); sessionStorage.clear(); window.history.replaceState({}, '', '/login')
  loginNumber = 0; refreshCount = 0; expiresIn = 3600; refreshedExpiresIn = 3600
  holdRefresh = false; refreshStatus = 200; refreshResponse = undefined; lateResponse = undefined
  holdProfile = false; profileResponse = undefined; profileBalance = 1
  accessOwner = new Map(); refreshOwners = new Map(); rejectedAccess = new Set(); requests = []
  // Model the supported browser Web Locks path; the refresh itself uses actual HTTP.
  Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: (_name: string, _options: unknown, callback: (lock: object) => unknown) => callback({ name: _name }) } })
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore()
})
afterEach(async () => {
  await auth.logout(); disposePinia(pinia); server.closeAllConnections(); vi.restoreAllMocks()
})
afterAll(async () => {
  Object.assign(apiClient.defaults, original); Object.assign(axios.defaults, rawOriginal)
  if (originalLocks) Object.defineProperty(navigator, 'locks', originalLocks)
  else Reflect.deleteProperty(navigator, 'locks')
  await new Promise<void>(resolve => server.close(() => resolve()))
})
async function login(id = 1) {
  await auth.login({ email: `fixture-${id}@example.test`, password: 'synthetic-fixture-only' })
}
function rejectCurrentAccess() { rejectedAccess.add(auth.token!) }
function protectedRequests() { return requests.filter(r => r.path.startsWith('/api/v1/fixture/')) }

describe('native authentication refresh through actual HTTP', () => {
  it('waits for a contended peer commit even when every cached storage field is still old', async () => {
    await login()
    Object.defineProperty(navigator, 'locks', { configurable: true, value: {
      request: async (_name: string, optionsOrCallback: unknown, available?: (lock: null) => unknown) => {
        if (available) return available(null)
        const { data } = await axios.post('/api/v1/auth/refresh', { refresh_token: localStorage.getItem('refresh_token') })
        window.setTimeout(() => {
          localStorage.setItem('auth_token', data.data.access_token)
          localStorage.setItem('token_expires_at', String(Date.now() + 3600_000))
          localStorage.setItem('refresh_token', data.data.refresh_token)
        }, 40)
        return (optionsOrCallback as () => unknown)()
      }
    } })
    const result = await refreshAuthTokens()
    expect(result.access_token).toBe('fixture-1-access-renewed-1')
    expect(refreshCount).toBe(1)
    expect(auth.token).toBe(result.access_token)
  })

  it('invalidates an old HTTP response before OAuth refresh credentials become visible', async () => {
    await login()
    const old = apiClient.get('/fixture/late').catch(error => error)
    await vi.waitFor(() => expect(lateResponse).toBeDefined())
    const incoming = tokenPair(2, 'oauth')
    persistOAuthTokenContext(incoming)
    expect.soft(localStorage.getItem('auth_session_id')).toMatch(/^pending:/)
    reply(lateResponse!, { owner: 1 })
    expect.soft(await old).toMatchObject({ code: 'AUTH_SESSION_CHANGED' })
    await auth.setToken(incoming.access_token)
    expect(auth.user?.id).toBe(2)
    expect(localStorage.getItem('auth_session_id')).not.toMatch(/^pending:/)
  })

  it('does not open a new session for an OAuth bind completion without login credentials', async () => {
    await login()
    const marker = localStorage.getItem('auth_session_id')
    persistOAuthTokenContext({})
    expect(localStorage.getItem('auth_session_id')).toBe(marker)
  })

  it('does not treat protected auth endpoints as an interactive login during peer publication', async () => {
    await login()
    const marker = localStorage.getItem('auth_session_id')!
    localStorage.setItem('auth_session_id', 'pending:' + marker + '|peer')
    await expect(apiClient.post('/auth/revoke-all-sessions')).rejects.toMatchObject({ code: 'TOKEN_REFRESH_UNAVAILABLE' })
    expect(requests.filter(r => r.path.endsWith('/auth/revoke-all-sessions'))).toHaveLength(0)
    localStorage.setItem('auth_session_id', marker)
  })

  it.each([false, true])('adopts a publication before the refresh snapshot and blocks mixed-pair requests (commit while queued=%s)', async queued => {
    await login()
    const { data } = await axios.post('/api/v1/auth/refresh', { refresh_token: localStorage.getItem('refresh_token') })
    const marker = localStorage.getItem('auth_session_id')!
    localStorage.setItem('auth_session_id', 'pending:' + marker + '|peer')
    localStorage.setItem('auth_token', data.data.access_token)
    await expect(apiClient.get('/fixture/protected')).rejects.toMatchObject({ code: 'TOKEN_REFRESH_UNAVAILABLE' })
    expect(auth.token).toBe('fixture-1-access-login-1')
    expect(protectedRequests()).toHaveLength(0)
    const commit = () => {
      localStorage.setItem('token_expires_at', String(Date.now() + 3600_000))
      localStorage.setItem('refresh_token', data.data.refresh_token)
      localStorage.setItem('auth_session_id', marker)
    }
    if (queued) {
      localStorage.setItem('refresh_token', data.data.refresh_token)
      Object.defineProperty(navigator, 'locks', { configurable: true, value: {
        request: (_name: string, _options: unknown, callback: (lock: object) => unknown) => { commit(); return callback({ name: _name }) }
      } })
    }
    const pending = refreshAuthTokens()
    if (!queued) commit()
    expect((await pending).access_token).toBe(data.data.access_token)
    expect(refreshCount).toBe(1)
    expect(auth.token).toBe(data.data.access_token)
  })

  it('retains the current session after publication timeout and accepts its eventual commit', async () => {
    await login()
    const marker = localStorage.getItem('auth_session_id')!
    localStorage.setItem('auth_session_id', 'pending:' + marker + '|peer')
    await expect(refreshAuthTokens()).rejects.toBeInstanceOf(AuthRefreshPendingError)
    expect(auth.isAuthenticated).toBe(true)
    expect(sessionStorage.getItem('auth_expired')).toBeNull()
    expect(refreshCount).toBe(0)
    localStorage.setItem('auth_session_id', marker)
    await expect(apiClient.get('/fixture/protected')).resolves.toMatchObject({ data: { owner: 1 } })
  })

  it('allows interactive login to recover an abandoned peer publication', async () => {
    await login()
    localStorage.setItem('auth_session_id', 'pending:abandoned-peer|publication')
    auth.checkAuth()
    await login(2)
    expect(auth.user?.id).toBe(2)
    expect(localStorage.getItem('auth_session_id')).not.toMatch(/^pending:/)
    await expect(apiClient.get('/fixture/protected')).resolves.toMatchObject({ data: { owner: 2 } })
  })

  it('does not finish a publication replaced by a newer owner', async () => {
    await login()
    const first = startAuthSession()
    const second = startAuthSession()
    expect(finishAuthSession(first)).toBe(false)
    expect(localStorage.getItem('auth_session_id')).toBe(second)
    expect(finishAuthSession(second)).toBe(true)
  })

  it.each([200, 503])('does not clear or commit over a newer login after an old setToken profile completes (%s)', async status => {
    await login()
    holdProfile = true
    const oldToken = tokenPair(1, 'oauth').access_token
    const pending = auth.setToken(oldToken).catch(error => error)
    await vi.waitFor(() => expect(profileResponse).toBeDefined())
    const oldResponse = profileResponse!
    holdProfile = false
    await login(2)
    const marker = localStorage.getItem('auth_session_id')
    const access = auth.token
    reply(oldResponse, user(1), status)
    await pending
    expect(auth.user?.id).toBe(2)
    expect(auth.token).toBe(access)
    expect(localStorage.getItem('auth_session_id')).toBe(marker)
    expect(localStorage.getItem('auth_token')).toBe(access)
  })

  it('waits for the peer refresh commit when storage exposes a partial token pair after the lock', async () => {
    await login()
    Object.defineProperty(navigator, 'locks', { configurable: true, value: {
      request: async (_name: string, _options: unknown, callback: (lock: object) => unknown) => {
        const { data } = await axios.post('/api/v1/auth/refresh', { refresh_token: localStorage.getItem('refresh_token') })
        const marker = localStorage.getItem('auth_session_id')!
        localStorage.setItem('auth_session_id', 'pending:' + marker)
        localStorage.setItem('auth_token', data.data.access_token)
        // Reproduce the native two-renderer trace: access arrives before the refresh commit.
        window.setTimeout(() => {
          localStorage.setItem('token_expires_at', String(Date.now() + 3600_000))
          localStorage.setItem('refresh_token', data.data.refresh_token)
          localStorage.setItem('auth_session_id', marker)
        }, 40)
        return callback({ name: _name })
      }
    } })
    const result = await refreshAuthTokens()
    expect(result.access_token).toBe('fixture-1-access-renewed-1')
    expect(refreshCount).toBe(1)
    expect(auth.token).toBe(result.access_token)
  })

  it('updates Pinia automatically after proactive rotation and schedules the next rotation', async () => {
    expiresIn = 121; refreshedExpiresIn = 121
    await login()
    await vi.waitFor(() => expect(refreshCount).toBe(1), { timeout: 2500 })
    await vi.waitFor(() => expect(localStorage.getItem('auth_token')).toBe('fixture-1-access-renewed-1'))
    expect(auth.token).toBe('fixture-1-access-renewed-1')
    expect(auth.user?.id).toBe(1)
    await vi.waitFor(() => expect(refreshCount).toBe(2), { timeout: 2500 })
    await vi.waitFor(() => expect(auth.token).toBe('fixture-1-access-renewed-2'))
  })

  it.each([false, true])('coalesces concurrent 401s and retries each original request once (proactive=%s)', async proactive => {
    if (proactive) expiresIn = 121
    holdRefresh = true; await login(); rejectCurrentAccess()
    if (proactive) await vi.waitFor(() => expect(refreshResponse).toBeDefined(), { timeout: 2500 })
    const calls = [1, 2, 3].map(n => apiClient.get(`/fixture/protected-${n}`))
    const results = Promise.all(calls)
    await vi.waitFor(() => { expect(protectedRequests()).toHaveLength(3); expect(refreshResponse).toBeDefined() })
    expect(refreshCount).toBe(1); finishRefresh()
    expect((await results).map(r => r.data.owner)).toEqual([1, 1, 1])
    expect(auth.token).toBe('fixture-1-access-renewed-1')
    expect(protectedRequests().map(r => r.token)).toEqual([
      'fixture-1-access-login-1', 'fixture-1-access-login-1', 'fixture-1-access-login-1',
      'fixture-1-access-renewed-1', 'fixture-1-access-renewed-1', 'fixture-1-access-renewed-1'
    ])
    expect(refreshCount).toBe(1)
  })

  it('applies the current user response after its own 401 refresh without manual session restoration', async () => {
    await login(); rejectCurrentAccess(); profileBalance = 2
    await expect(auth.refreshUser()).resolves.toMatchObject({ id: 1, balance: 2 })
    expect(auth.token).toBe('fixture-1-access-renewed-1')
    expect(auth.user?.balance).toBe(2); expect(refreshCount).toBe(1)
  })

  it('does not attach a new account to a retry queued after rotation', async () => {
    await login(); rejectCurrentAccess()
    let releaseRetry!: () => void
    let retryQueued = false
    const gate = new Promise<void>(resolve => { releaseRetry = resolve })
    const interceptor = apiClient.interceptors.request.use(async config => {
      if ((config as typeof config & { _retry?: boolean })._retry) { retryQueued = true; await gate }
      return config
    })
    const pending = apiClient.get('/fixture/protected').catch(error => error)
    try {
      await vi.waitFor(() => expect(retryQueued).toBe(true))
      await auth.logout(); await login(2); const token = auth.token
      releaseRetry()
      expect(await pending).toMatchObject({ status: 401, code: 'AUTH_SESSION_CHANGED' })
      expect(auth.token).toBe(token); expect(auth.user?.id).toBe(2)
      expect(protectedRequests()).toHaveLength(1); expect(refreshCount).toBe(1)
    } finally { releaseRetry(); apiClient.interceptors.request.eject(interceptor); await pending }
  })

  it('allows the new session to refresh while an old session refresh is still in flight', async () => {
    await login(); rejectCurrentAccess(); holdRefresh = true
    const oldPending = apiClient.get('/fixture/old').catch(error => error)
    await vi.waitFor(() => expect(refreshResponse).toBeDefined())
    const oldResponse = refreshResponse!
    await auth.logout(); await login(2); rejectCurrentAccess()
    const newPending = apiClient.get('/fixture/new').catch(error => error)
    try {
      await vi.waitFor(() => expect(refreshCount).toBe(2))
      finishRefresh()
      expect((await newPending).data).toMatchObject({ owner: 2 })
      reply(oldResponse, tokenPair(1, 'obsolete'))
      expect(await oldPending).toMatchObject({ code: 'AUTH_SESSION_CHANGED' })
      expect(auth.token).toBe('fixture-2-access-renewed-2'); expect(auth.user?.id).toBe(2)
      expect(requests.filter(r => r.path === '/api/v1/fixture/old')).toHaveLength(1)
    } finally {
      if (!oldResponse.writableEnded) reply(oldResponse, {}, 503)
      if (refreshResponse && !refreshResponse.writableEnded) finishRefresh(503)
      await Promise.all([oldPending, newPending])
    }
  })

  it.each(['logout', 'account', 'same-account-login'] as const)('ignores a late successful refresh after %s', async change => {
    await login(); rejectCurrentAccess(); holdRefresh = true
    const pending = apiClient.get('/fixture/protected').catch(error => error)
    await vi.waitFor(() => expect(refreshResponse).toBeDefined())
    await auth.logout()
    if (change !== 'logout') await login(change === 'account' ? 2 : 1)
    const expectedToken = auth.token; const expectedID = auth.user?.id
    finishRefresh()
    expect(await pending).toMatchObject({ status: 401, code: 'AUTH_SESSION_CHANGED' })
    expect(auth.token).toBe(expectedToken); expect(auth.user?.id).toBe(expectedID)
    expect(localStorage.getItem('auth_token')).toBe(expectedToken)
    expect(protectedRequests()).toHaveLength(1)
    expect(sessionStorage.getItem('auth_expired')).toBeNull()
  })

  it('does not clear a new account when an old profile refresh is rejected', async () => {
    await login(); rejectCurrentAccess(); holdRefresh = true
    const pending = auth.refreshUser().catch(error => error)
    await vi.waitFor(() => expect(refreshResponse).toBeDefined())
    await auth.logout(); await login(2); const expectedToken = auth.token
    finishRefresh(401)
    expect(await pending).toMatchObject({ status: 401, code: 'AUTH_SESSION_CHANGED' })
    expect(auth.user?.id).toBe(2); expect(auth.token).toBe(expectedToken)
    expect(localStorage.getItem('auth_token')).toBe(expectedToken)
  })

  it.each(['account', 'logout', 'same-account-login'] as const)('does not retry a delayed old 401 under the session after %s', async change => {
    await login()
    const pending = apiClient.get('/fixture/late').catch(error => error)
    await vi.waitFor(() => expect(lateResponse).toBeDefined())
    await auth.logout()
    if (change !== 'logout') await login(change === 'account' ? 2 : 1)
    const expectedToken = auth.token; const expectedID = auth.user?.id
    reply(lateResponse!, {}, 401)
    expect(await pending).toMatchObject({ status: 401, code: 'AUTH_SESSION_CHANGED' })
    expect(auth.token).toBe(expectedToken); expect(auth.user?.id).toBe(expectedID)
    expect(localStorage.getItem('auth_token')).toBe(expectedToken)
    expect(refreshCount).toBe(0); expect(protectedRequests()).toHaveLength(1)
    expect(sessionStorage.getItem('auth_expired')).toBeNull()
  })

  it.each([401, 403])('clears the rejected current session in storage and Pinia (%s)', async status => {
    await login(); rejectCurrentAccess(); refreshStatus = status
    await expect(apiClient.get('/fixture/protected')).rejects.toMatchObject({ status: 401, code: 'TOKEN_REFRESH_FAILED' })
    expect(auth.isAuthenticated).toBe(false); expect(auth.user).toBeNull(); expect(auth.token).toBeNull()
    for (const key of ['auth_token', 'refresh_token', 'auth_user', 'token_expires_at']) expect(localStorage.getItem(key)).toBeNull()
    expect(sessionStorage.getItem('auth_expired')).toBe('1'); expect(protectedRequests()).toHaveLength(1)
  })

  it.each([429, 503])('retains the session when refresh is temporarily unavailable (%s)', async status => {
    await login(); rejectCurrentAccess(); refreshStatus = status; const token = auth.token
    await expect(apiClient.get('/fixture/protected')).rejects.toMatchObject({ status, code: 'TOKEN_REFRESH_UNAVAILABLE' })
    expect(auth.token).toBe(token); expect(auth.isAuthenticated).toBe(true)
    expect(localStorage.getItem('auth_token')).toBe(token); expect(sessionStorage.getItem('auth_expired')).toBeNull()
    expect(protectedRequests()).toHaveLength(1)
  })

  it('does not overwrite the new identity with a delayed profile response', async () => {
    await login(); holdProfile = true
    const pending = auth.refreshUser().catch(error => error)
    await vi.waitFor(() => expect(profileResponse).toBeDefined())
    await auth.logout(); await login(2); const token = auth.token
    reply(profileResponse!, user(1, 99))
    expect((await pending).message).toContain('Authentication session changed')
    expect(auth.user?.id).toBe(2); expect(auth.user?.balance).toBe(1); expect(auth.token).toBe(token)
  })
  it.each([401, 403])('expires a proactively rejected current session (%s)', async status => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {})
    expiresIn = 121; refreshStatus = status
    await login()
    await vi.waitFor(() => expect(errors).toHaveBeenCalled(), { timeout: 3000 })
    expect(auth.isAuthenticated).toBe(false); expect(auth.user).toBeNull(); expect(auth.token).toBeNull()
    for (const key of ['auth_token', 'refresh_token', 'auth_user', 'token_expires_at']) expect(localStorage.getItem(key)).toBeNull()
    expect(protectedRequests()).toHaveLength(0); expect(sessionStorage.getItem('auth_expired')).toBe('1')
  })

  it.each([429, 503, 0])('retains the session after proactive temporary failure (%s)', async status => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {})
    expiresIn = 121; holdRefresh = true
    await login(); const token = auth.token
    await vi.waitFor(() => expect(refreshResponse).toBeDefined(), { timeout: 2500 })
    if (status === 0) { refreshResponse!.destroy(); refreshResponse = undefined }
    else finishRefresh(status)
    await vi.waitFor(() => expect(errors).toHaveBeenCalled(), { timeout: 2500 })
    expect(auth.token).toBe(token); expect(auth.isAuthenticated).toBe(true)
    expect(localStorage.getItem('auth_token')).toBe(token); expect(sessionStorage.getItem('auth_expired')).toBeNull()
  })

  it.each(['logout', 'account', 'same-account-login'] as const)('ignores proactive hard rejection after %s', async change => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {})
    expiresIn = 121; holdRefresh = true
    await login()
    await vi.waitFor(() => expect(refreshResponse).toBeDefined(), { timeout: 2500 })
    await auth.logout(); expiresIn = 3600
    if (change !== 'logout') await login(change === 'account' ? 2 : 1)
    const token = auth.token; const id = auth.user?.id
    finishRefresh(401)
    await vi.waitFor(() => expect(errors).toHaveBeenCalled(), { timeout: 2500 })
    expect(auth.token).toBe(token); expect(auth.user?.id).toBe(id)
    expect(localStorage.getItem('auth_token')).toBe(token); expect(sessionStorage.getItem('auth_expired')).toBeNull()
  })

  it('preserves a same-session profile update when another profile refresh receives 503', async () => {
    await login(); holdProfile = true; const token = auth.token
    const priorProfile = auth.refreshUser()
    await vi.waitFor(() => expect(profileResponse).toBeDefined())
    rejectCurrentAccess(); holdRefresh = true
    const pendingProfile = auth.refreshUser().catch(error => error)
    await vi.waitFor(() => expect(refreshResponse).toBeDefined())
    reply(profileResponse!, user(1, 2))
    await expect(priorProfile).resolves.toMatchObject({ id: 1, balance: 2 })
    finishRefresh(503)
    expect.soft(await pendingProfile).toMatchObject({ status: 503, code: 'TOKEN_REFRESH_UNAVAILABLE' })
    expect.soft(auth.isAuthenticated).toBe(true); expect.soft(auth.user?.balance).toBe(2)
    expect.soft(auth.token).toBe(token); expect.soft(localStorage.getItem('auth_token')).toBe(token)
    expect.soft(sessionStorage.getItem('auth_expired')).toBeNull()
  })

})
