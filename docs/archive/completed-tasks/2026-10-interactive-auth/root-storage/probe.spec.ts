import { createServer, type Server } from 'node:http'
import { writeFileSync } from 'node:fs'
import axios from 'axios'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import apiClient from '@/api/client'
import { authAPI } from '@/api/auth'
import { getAuthSessionIdentity, isAuthSessionPublishing, isOwnAuthSessionPublication, isAuthSessionBlocked } from '@/api/tokenRefresh'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/i18n', () => ({ getLocale: () => 'zh-CN' }))
const kinds = ['login', 'login2FA', 'register'] as const
type Kind = typeof kinds[number]
const keys = ['auth_token', 'refresh_token', 'token_expires_at', 'auth_user', 'auth_session_id'] as const
type Key = typeof keys[number]
let server: Server
let pinia: ReturnType<typeof createPinia>
let auth: ReturnType<typeof useAuthStore>
let requests: string[]
const records: unknown[] = []
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawOriginal = { baseURL: axios.defaults.baseURL, adapter: axios.defaults.adapter }
const user = (id: number) => ({ id, email: `fixture-${id}@example.test`, role: 'user', status: 'active', balance: id, concurrency: 5 })
const payload = (label: string) => ({ access_token: `fixture-access-${label}`, refresh_token: `fixture-refresh-${label}`, expires_in: 3600, token_type: 'Bearer', user: user(Number(label.split('-')[0])) })
const labelOf = (body: Record<string, unknown>) => String(body.email || body.temp_token || '').split('@')[0]
function call(kind: Kind, label: string) {
  if (kind === 'login2FA') return auth.login2FA(label, '123456')
  const body = { email: `${label}@example.test`, password: 'synthetic-fixture-only' }
  return kind === 'register' ? auth.register(body) : auth.login(body)
}
const settled = (promise: Promise<unknown>) => promise.then(value => ({ value, error: null }), error => ({ value: null, error }))
const state = () => ({ token: auth.token, userID: auth.user?.id ?? null, authenticated: auth.isAuthenticated,
  identity: getAuthSessionIdentity(), publishing: isAuthSessionPublishing(), ownsPublication: isOwnAuthSessionPublication(), blocked: isAuthSessionBlocked(),
  stored: Object.fromEntries(keys.map(k => [k, localStorage.getItem(k)])) })
const errorInfo = (e: any) => e ? { name: e.name, code: e.code, status: e.status, message: e.message } : null
const clean = (s: ReturnType<typeof state>) => !s.authenticated && s.token === null && s.userID === null &&
  !s.publishing && !s.ownsPublication && keys.filter(k => k !== 'auth_session_id').every(k => s.stored[k] === null)
const recovered = (s: ReturnType<typeof state>) => s.authenticated && s.userID === 3 && s.token === 'fixture-access-3-recovery' &&
  s.stored.auth_token === s.token && s.stored.refresh_token === 'fixture-refresh-3-recovery' &&
  JSON.parse(s.stored.auth_user || '{}').id === 3 && !s.publishing && !s.ownsPublication && !s.blocked

// Throw exactly once before one target write. Every subsequent write, including
// cleanup and recovery, delegates to the actual jsdom Storage implementation.
function injectOnce(key: Key, phase: 'api' | 'store') {
  const target = typeof Object.getPrototypeOf(localStorage).setItem === 'function' ? Object.getPrototypeOf(localStorage) : localStorage
  const originalSet = target.setItem
  let seen = 0, fired = 0
  vi.spyOn(target, 'setItem').mockImplementation(function (this: Storage, k: string, value: string) {
    const matches = this === localStorage && k === key && (key !== 'auth_session_id' || !String(value).startsWith('pending:'))
    if (matches && ++seen === (phase === 'api' ? 1 : 2)) {
      fired++
      throw new DOMException(`one-time fixture failure: ${phase}/${key}`, 'QuotaExceededError')
    }
    return originalSet.call(this, k, value)
  })
  return () => fired
}

beforeAll(async () => {
  server = createServer(async (req, res) => {
    const path = req.url?.split('?')[0] || ''
    let raw = ''; for await (const chunk of req) raw += chunk
    const body = raw ? JSON.parse(raw) : {}
    requests.push(path)
    let data: unknown = {}
    if (['/api/v1/auth/login', '/api/v1/auth/login/2fa', '/api/v1/auth/register'].includes(path)) data = payload(labelOf(body))
    else if (path === '/api/v1/auth/me') data = user(1)
    res.writeHead(200, { 'Content-Type': 'application/json', Connection: 'close' })
    res.end(JSON.stringify({ code: 0, data }))
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing HTTP fixture port')
  const origin = `http://127.0.0.1:${address.port}`
  Object.assign(apiClient.defaults, { baseURL: origin + '/api/v1', adapter: 'http' })
  Object.assign(axios.defaults, { baseURL: origin, adapter: 'http' })
})
beforeEach(() => {
  localStorage.clear(); sessionStorage.clear(); window.history.replaceState({}, '', '/login'); requests = []
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore()
})
afterEach(async () => { vi.restoreAllMocks(); await auth.logout(); disposePinia(pinia); server.closeAllConnections() })
afterAll(async () => {
  Object.assign(apiClient.defaults, original); Object.assign(axios.defaults, rawOriginal)
  await new Promise<void>(resolve => server.close(() => resolve()))
  writeFileSync(`/tmp/p1-interactive-storage-probe/${process.env.PROBE_LABEL}-states.json`, JSON.stringify(records, null, 2) + '\n')
})

describe.each(kinds)('%s one-time Storage failure through actual HTTP', kind => {
  it.each(['api', 'store'].flatMap(phase => keys.map(key => ({ phase: phase as 'api' | 'store', key }))))('cleans own $phase/$key failure and recovers', async ({ phase, key }) => {
    await call('login', '1-initial')
    const initial = state()
    const fired = injectOnce(key, phase)
    const failed = await settled(call(kind, '2-failed'))
    const afterFailure = state()
    // Recovery executes even when the cleanup assertion is RED.
    const recovery = await settled(call('login', '3-recovery'))
    const afterRecovery = state()
    records.push({ case: 'own-failure', kind, phase, key, initial, afterFailure, afterRecovery,
      error: errorInfo(failed.error), recoveryError: errorInfo(recovery.error), fired: fired(),
      cleanupCorrect: clean(afterFailure), recoveryCorrect: recovered(afterRecovery), requests: [...requests] })
    expect.soft(fired()).toBe(1)
    expect.soft(failed.error?.name).toBe('QuotaExceededError')
    expect.soft(clean(afterFailure), JSON.stringify(afterFailure)).toBe(true)
    expect.soft(recovery.error).toBeNull()
    expect.soft(recovered(afterRecovery), JSON.stringify(afterRecovery)).toBe(true)
  })

  it('cleans a fresh-session API auth_user failure and recovers', async () => {
    const fired = injectOnce('auth_user', 'api')
    const failed = await settled(call(kind, '2-failed'))
    const afterFailure = state()
    const recovery = await settled(call('login', '3-recovery'))
    const afterRecovery = state()
    records.push({ case: 'fresh-failure', kind, phase: 'api', key: 'auth_user', afterFailure, afterRecovery,
      error: errorInfo(failed.error), recoveryError: errorInfo(recovery.error), fired: fired(),
      cleanupCorrect: clean(afterFailure), recoveryCorrect: recovered(afterRecovery), requests: [...requests] })
    expect.soft(fired()).toBe(1)
    expect.soft(failed.error?.name).toBe('QuotaExceededError')
    expect.soft(clean(afterFailure), JSON.stringify(afterFailure)).toBe(true)
    expect.soft(recovery.error).toBeNull()
    expect.soft(recovered(afterRecovery), JSON.stringify(afterRecovery)).toBe(true)
  })

  it('does not clear a genuinely newer HTTP session when the original Storage rejection is delivered late', async () => {
    await call('login', '1-initial')
    const fired = injectOnce('auth_user', 'api')
    const real = authAPI[kind]
    let observed = false
    let release!: () => void
    const barrier = new Promise<void>(resolve => { release = resolve })
    // Timing barrier only: delegate every HTTP/API call to its real helper and
    // propagate the original thrown DOMException. This deliberately widens the
    // API->store rejection handoff, not a claim about native event-loop timing.
    vi.spyOn(authAPI, kind).mockImplementation((async (...args: unknown[]) => {
      try { return await (real as (...a: unknown[]) => Promise<unknown>)(...args) }
      catch (error) {
        if (labelOf(args[0] as Record<string, unknown>) === '2-failed') { observed = true; await barrier }
        throw error
      }
    }) as any)
    const old = settled(call(kind, '2-failed'))
    await vi.waitFor(() => expect(observed).toBe(true))
    const newer = await settled(call('login', '3-recovery'))
    const winner = state()
    release()
    const oldResult = await old
    const afterOld = state()
    records.push({ case: 'newer-session-before-rejection-handoff', kind, fired: fired(), winner, afterOld,
      newerError: errorInfo(newer.error), oldError: errorInfo(oldResult.error), newerPreserved: JSON.stringify(winner) === JSON.stringify(afterOld),
      scheduling: 'Artificial API rejection barrier; original real HTTP helper/error retained', requests: [...requests] })
    expect.soft(fired()).toBe(1)
    expect.soft(newer.error).toBeNull()
    expect.soft(recovered(winner), JSON.stringify(winner)).toBe(true)
    expect.soft(oldResult.error?.name).toBe('QuotaExceededError')
    expect.soft(afterOld).toEqual(winner)
  })
})
