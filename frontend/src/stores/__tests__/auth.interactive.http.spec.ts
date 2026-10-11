// Real Pinia, API helpers, Axios interceptors and Node HTTP. Only transport location/config is substituted.
import { createServer, type Server, type ServerResponse } from 'node:http'
import axios from 'axios'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import apiClient from '@/api/client'
import { getAuthSessionIdentity, refreshAuthTokens } from '@/api/tokenRefresh'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/i18n', () => ({ getLocale: () => 'zh-CN' }))
const kinds = ['login', 'login2FA', 'register'] as const
type Kind = typeof kinds[number]
type Outcome = 200 | 401 | 503 | 'timeout' | 'cancel'
const outcomes: Outcome[] = [200, 401, 503, 'timeout', 'cancel']
let server: Server
let pinia: ReturnType<typeof createPinia>
let auth: ReturnType<typeof useAuthStore>
let held: Map<string, ServerResponse>
let heldLabels: Set<string>
let transportOutcome: Outcome
let controller: AbortController
let requests: string[]
let interceptor: number
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawOriginal = { baseURL: axios.defaults.baseURL, adapter: axios.defaults.adapter }
const labelOf = (body: Record<string, unknown>) => String(body.email || body.temp_token || '').split('@')[0]
const user = (id: number) => ({ id, email: `fixture-${id}@example.test`, role: 'user', status: 'active', balance: id, concurrency: 5 })
const pair = (label: string) => ({ access_token: `fixture-access-${label}`, refresh_token: `fixture-refresh-${label}`, expires_in: 3600, token_type: 'Bearer' })
const payload = (label: string) => ({ ...pair(label), user: user(Number(label.split('-')[0])) })
function reply(res: ServerResponse, label: string, code = 200, data: unknown = payload(label)) {
  if (res.destroyed || res.writableEnded) return
  res.writeHead(code, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify({ code: code === 200 ? 0 : code, message: code === 200 ? undefined : `fixture-error-${code}`, reason: `FIXTURE_${code}`, data }))
}
function release(label: string, code = 200, data?: unknown) {
  const res = held.get(label)
  if (!res) throw new Error(`Missing held HTTP response ${label}`)
  reply(res, label, code, data)
}
function call(kind: Kind, label: string) {
  if (kind === 'login2FA') return auth.login2FA(label, '123456')
  const body = { email: `${label}@example.test`, password: 'synthetic-fixture-only' }
  return kind === 'register' ? auth.register(body) : auth.login(body)
}
const settled = (promise: Promise<unknown>) => promise.then(value => ({ value, error: null }), error => ({ value: null, error }))
const state = () => ({ token: auth.token, userID: auth.user?.id ?? null, authenticated: auth.isAuthenticated, identity: getAuthSessionIdentity(),
  stored: Object.fromEntries(['auth_token', 'refresh_token', 'token_expires_at', 'auth_user', 'auth_session_id'].map(k => [k, localStorage.getItem(k)])) })
async function completeOld(outcome: Outcome) {
  if (outcome === 'cancel') controller.abort()
  else if (outcome !== 'timeout') release('1-old', outcome)
}

beforeAll(async () => {
  server = createServer(async (req, res) => {
    const path = req.url?.split('?')[0] || ''
    requests.push(path)
    let raw = ''; for await (const chunk of req) raw += chunk
    const body = raw ? JSON.parse(raw) : {}
    if (['/api/v1/auth/login', '/api/v1/auth/login/2fa', '/api/v1/auth/register'].includes(path)) {
      const label = labelOf(body)
      if (heldLabels.has(label)) { held.set(label, res); return }
      reply(res, label)
    } else if (path === '/api/v1/auth/logout') reply(res, '1-logout', 200, {})
    else if (path === '/api/v1/auth/refresh') reply(res, '1-refresh', 200, pair('1-refresh'))
    else if (path === '/api/v1/auth/me') reply(res, '1-profile', 200, user(1))
    else reply(res, '1-missing', 404, {})
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing HTTP fixture port')
  const origin = `http://127.0.0.1:${address.port}`
  Object.assign(apiClient.defaults, { baseURL: origin + '/api/v1', adapter: 'http' })
  Object.assign(axios.defaults, { baseURL: origin, adapter: 'http' })
  interceptor = apiClient.interceptors.request.use(config => {
    const data = typeof config.data === 'string' ? JSON.parse(config.data) : config.data || {}
    if (labelOf(data) === '1-old') {
      if (transportOutcome === 'timeout') config.timeout = 200
      if (transportOutcome === 'cancel') config.signal = controller.signal
    }
    return config
  })
})
beforeEach(() => {
  localStorage.clear(); sessionStorage.clear(); window.history.replaceState({}, '', '/login')
  held = new Map(); heldLabels = new Set(); requests = []; transportOutcome = 200; controller = new AbortController()
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore()
})
afterEach(async () => { await auth.logout(); disposePinia(pinia); server.closeAllConnections() })
afterAll(async () => {
  apiClient.interceptors.request.eject(interceptor)
  Object.assign(apiClient.defaults, original); Object.assign(axios.defaults, rawOriginal)
  await new Promise<void>(resolve => server.close(() => resolve()))
})

describe.each(kinds)('interactive %s through actual HTTP', kind => {
  it.each(['account', 'same-account', 'logout'].flatMap(transition => outcomes.map(outcome => ({ transition, outcome }))))('preserves $transition after old $outcome', async ({ transition, outcome }) => {
    // Each outcome uses a complete real session so successful old responses cannot hide behind user-ID equality.
    await call('login', '1-initial')
    heldLabels.add('1-old'); transportOutcome = outcome
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    if (transition === 'logout') await auth.logout()
    else await call('login', transition === 'account' ? '2-current' : '1-current')
    const current = state()
    await completeOld(outcome)
    const result = await old
    expect(state(), `old ${outcome}`).toEqual(current)
    expect(result.error, `old ${outcome} must reject`).toBeTruthy()
    if (outcome === 'cancel') expect(axios.isCancel(result.error)).toBe(true)
    if (outcome === 'timeout') expect(result.error.status).toBe(0)
    expect(requests.filter(path => path === '/api/v1/auth/refresh')).toHaveLength(0)
  })

  it('rejects an old success while the latest login is still pending', async () => {
    await call('login', '1-initial')
    const initial = state()
    heldLabels.add('1-old'); heldLabels.add('2-current')
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    const current = settled(call('login', '2-current'))
    await vi.waitFor(() => expect(held.has('2-current')).toBe(true))
    release('1-old')
    const oldResult = await old
    expect(state()).toEqual(initial)
    expect(oldResult.error).toBeTruthy()
    release('2-current')
    expect((await current).error).toBeNull()
    expect(auth.user?.id).toBe(2)
  })

  it.each([401, 503])('does not clear a session when old HTTP %s arrives before the newer success', async code => {
    await call('login', '1-initial')
    const initial = state()
    heldLabels.add('1-old'); heldLabels.add('2-current')
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    const current = settled(call('login', '2-current'))
    await vi.waitFor(() => expect(held.has('2-current')).toBe(true))
    release('1-old', code)
    expect((await old).error.status).toBe(code)
    expect(state()).toEqual(initial)
    release('2-current')
    expect((await current).error).toBeNull()
    expect(auth.user?.id).toBe(2)
  })

  it.each(['login2FA', 'register'] as const)('cannot overwrite a later successful %s', async newerKind => {
    heldLabels.add('1-old')
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    await call(newerKind, '2-current')
    const current = state()
    release('1-old')
    expect((await old).error).toBeTruthy()
    expect(state()).toEqual(current)
  })

  it('still completes a current authentication after normal token rotation', async () => {
    await call('login', '1-initial')
    heldLabels.add('1-old')
    const pending = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    const identity = getAuthSessionIdentity()
    await refreshAuthTokens()
    expect(getAuthSessionIdentity()).toEqual(identity)
    release('1-old')
    expect((await pending).error).toBeNull()
    expect(auth.token).toBe('fixture-access-1-old')
    expect(auth.isAuthenticated).toBe(true)
  })

  it.each([401, 503, 'timeout', 'cancel'] as const)('preserves current %s error behavior', async outcome => {
    heldLabels.add('1-old'); transportOutcome = outcome
    const pending = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    await completeOld(outcome)
    const result = await pending
    expect(result.error).toBeTruthy()
    expect(auth.isAuthenticated).toBe(false)
    expect(localStorage.getItem('auth_token')).toBeNull()
    if (typeof outcome === 'number') {
      expect(result.error.status).toBe(outcome)
      expect(result.error.message).toBe(`fixture-error-${outcome}`)
      expect(result.error.reason).toBe(`FIXTURE_${outcome}`)
    } else if (outcome === 'cancel') expect(axios.isCancel(result.error)).toBe(true)
    else expect(result.error.status).toBe(0)
  })
})

it('returns a current 2FA challenge without publishing credentials', async () => {
  heldLabels.add('1-old')
  const pending = settled(call('login', '1-old'))
  await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
  release('1-old', 200, { requires_2fa: true, temp_token: 'fixture-only-challenge' })
  expect((await pending).value).toMatchObject({ requires_2fa: true })
  expect(auth.isAuthenticated).toBe(false)
  expect(localStorage.getItem('auth_token')).toBeNull()
})
it('rejects an obsolete 2FA challenge after another login succeeds', async () => {
  heldLabels.add('1-old')
  const pending = settled(call('login', '1-old'))
  await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
  await call('login', '2-current')
  const current = state()
  release('1-old', 200, { requires_2fa: true, temp_token: 'fixture-only-challenge' })
  expect((await pending).error).toBeTruthy()
  expect(state()).toEqual(current)
})

// These exercise lifecycle identity independently of token text and response order.
describe.each(kinds)('additional %s ownership boundaries', kind => {
  it.each([200, 401])('preserves an identical-token relogin after old HTTP %s', async outcome => {
    await call('login', '1-initial')
    const initial = state()
    heldLabels.add('1-old'); heldLabels.add('1-current')
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    const current = settled(call('login', '1-current'))
    await vi.waitFor(() => expect(held.has('1-current')).toBe(true))
    release('1-current', 200, payload('1-initial'))
    expect((await current).error).toBeNull()
    expect(auth.token).toBe(initial.token)
    expect(getAuthSessionIdentity()).not.toEqual(initial.identity)
    const latest = state()
    release('1-old', outcome)
    expect((await old).error).toBeTruthy()
    expect(state()).toEqual(latest)
  })

  it.each([401, 503])('does not revive an old success after the latest HTTP %s failure', async outcome => {
    heldLabels.add('1-old'); heldLabels.add('2-current')
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    const latest = settled(call('login', '2-current'))
    await vi.waitFor(() => expect(held.has('2-current')).toBe(true))
    release('2-current', outcome)
    expect((await latest).error.status).toBe(outcome)
    const signedOut = state()
    expect(signedOut.authenticated).toBe(false)
    release('1-old')
    expect((await old).error).toBeTruthy()
    expect(state()).toEqual(signedOut)
  })

  it('cannot replace a later setToken session', async () => {
    heldLabels.add('1-old')
    const old = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    await auth.setToken('fixture-access-1-set-token')
    const latest = state()
    release('1-old')
    expect((await old).error).toBeTruthy()
    expect(state()).toEqual(latest)
  })

  it.each([401, 503])('keeps current HTTP %s cleanup and subsequent login recovery', async outcome => {
    await call('login', '1-initial')
    heldLabels.add('1-old')
    const current = settled(call(kind, '1-old'))
    await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
    release('1-old', outcome)
    expect((await current).error.status).toBe(outcome)
    expect(auth.isAuthenticated).toBe(false)
    expect(localStorage.getItem('auth_token')).toBeNull()
    await call('login', '2-current')
    expect(auth.user?.id).toBe(2)
    expect(auth.token).toBe('fixture-access-2-current')
  })
})

it('does not publish an old success after a newer login returns a 2FA challenge', async () => {
  heldLabels.add('1-old'); heldLabels.add('2-current')
  const old = settled(call('login', '1-old'))
  await vi.waitFor(() => expect(held.has('1-old')).toBe(true))
  const current = settled(call('login', '2-current'))
  await vi.waitFor(() => expect(held.has('2-current')).toBe(true))
  release('2-current', 200, { requires_2fa: true, temp_token: 'new-fixture-challenge' })
  expect((await current).value).toMatchObject({ requires_2fa: true })
  const waitingFor2FA = state()
  release('1-old')
  expect((await old).error).toBeTruthy()
  expect(state()).toEqual(waitingFor2FA)
})

it('recovers an abandoned peer publication with an already initialized store', async () => {
  await call('login', '1-initial')
  localStorage.setItem('auth_session_id', 'pending:abandoned-fixture|unfinished')
  localStorage.setItem('refresh_token', 'fixture-unfinished-peer')
  await call('login', '2-current')
  expect(auth.user?.id).toBe(2)
  expect(auth.token).toBe('fixture-access-2-current')
  expect(localStorage.getItem('auth_session_id')).not.toMatch(/^pending:/)
})
