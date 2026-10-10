// Real Pinia, Axios interceptors and HTTP; only endpoint location/UI shells are substituted.
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http'
import axios from 'axios'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import PlayHubView from '@/views/user/PlayHubView.vue'
import zh from '@/i18n/locales/zh'

const endpoint = vi.hoisted(() => ({ base: '' }))
vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
vi.mock('@/api/url', () => ({ getAPIBaseURL: () => endpoint.base }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/utils/growthAnalytics', () => ({ trackGrowthEvent: vi.fn() }))
vi.mock('@/utils/featureFlags', async importOriginal => {
  const actual = await importOriginal<typeof import('@/utils/featureFlags')>()
  return { ...actual, isFeatureFlagEnabled: (flag: unknown) => flag === actual.FeatureFlags.playCheckin || flag === actual.FeatureFlags.affiliate }
})
enableAutoUnmount(afterEach)
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
let auth: ReturnType<typeof useAuthStore>
let pinia: ReturnType<typeof createPinia>
let server: Server
let expireSoon = false
let refreshes = 0
const reads: Array<{ token: string; path: string; res: ServerResponse }> = []
let respond: (req: IncomingMessage, res: ServerResponse) => void
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawAdapter = axios.defaults.adapter
const json = (res: ServerResponse, code: number, data: unknown) => {
  res.writeHead(code, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify({ code: code === 200 ? 0 : code, data }))
}
function replyRead(res: ServerResponse, path: string, label: string, balance: number) {
  json(res, 200, path.endsWith('/hub') ? {
    any_enabled: true, pending_actions: balance,
    growth: { balance }, checkin: { enabled: true, checked_in_today: false, streak_count: balance },
  } : [{ campaign: { id: balance, name: `${label}-private-campaign`, status: 'running' }, attention: `${label}-private-reward` }])
}
const waiting = (token: string) => reads.filter(r => r.token === token && !r.res.writableEnded)
const release = (token: string, label: string, balance: number) => {
  for (const r of waiting(token)) replyRead(r.res, r.path, label, balance)
}
const mountView = () => mount(PlayHubView, { global: { plugins: [pinia, i18n], stubs: {
  AppLayout: { template: '<div><slot /></div>' }, Icon: true, RouterLink: { template: '<a><slot /></a>' },
} } })
const login = (account: string) => auth.login({ email: `${account}@example.test`, password: 'fixture-only' })

beforeAll(async () => {
  server = createServer(async (req, res) => {
    const path = new URL(req.url || '/', 'http://fixture.test').pathname
    if (path === '/api/v1/auth/login') {
      let body = ''
      for await (const chunk of req) body += chunk
      const account = JSON.parse(body).email.startsWith('b@') ? 'b' : 'a'
      json(res, 200, { access_token: `fixture-${account}`, refresh_token: `refresh-${account}`,
        ...(expireSoon ? { expires_in: 121 } : {}),
        user: { id: account === 'a' ? 1 : 2, balance: account === 'a' ? 1 : 2, email: `${account}@example.test`, role: 'user' } })
    } else if (path === '/api/v1/auth/refresh') {
      refreshes++
      json(res, 200, { access_token: 'fixture-renewed', refresh_token: 'refresh-renewed', expires_in: 3600, token_type: 'Bearer' })
    } else if (path === '/api/v1/auth/logout') json(res, 200, {})
    else if (path === '/api/v1/auth/me') json(res, 200, { id: 1, balance: 1, email: 'a@example.test', role: 'user' })
    else {
      reads.push({ token: String(req.headers.authorization).replace('Bearer ', ''), path, res })
      respond(req, res)
    }
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing fixture address')
  endpoint.base = `http://127.0.0.1:${address.port}/api/v1`
  apiClient.defaults.baseURL = endpoint.base
  apiClient.defaults.adapter = 'http'
  axios.defaults.adapter = 'http'
})
beforeEach(async () => {
  localStorage.clear()
  pinia = createPinia()
  setActivePinia(pinia)
  auth = useAuthStore()
  reads.length = 0
  refreshes = 0
  expireSoon = false
  respond = () => undefined
  await login('a')
})
afterEach(async () => {
  await auth.logout()
  server.closeAllConnections()
})
afterAll(async () => {
  Object.assign(apiClient.defaults, original)
  axios.defaults.adapter = rawAdapter
  await new Promise<void>(resolve => server.close(() => resolve()))
  vi.unstubAllGlobals()
})

it.each([200, 403])('keeps B data when delayed A finishes with HTTP %s after account switch', async oldCode => {
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  await login('b')
  await vi.waitFor(() => expect(waiting('fixture-b')).toHaveLength(2))
  release('fixture-b', 'B', 2)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$2.00'))
  if (oldCode === 200) release('fixture-a', 'A', 1)
  else for (const r of waiting('fixture-a')) json(r.res, 403, {})
  await new Promise(resolve => setTimeout(resolve, 30))
  expect(view.get('.gw-balance-value').text()).toBe('$2.00')
  expect(view.text()).toContain('B-private-campaign')
  expect(view.text()).not.toContain('A-private')
  expect(view.find('[role="alert"]').exists()).toBe(false)
})

it('clears previously loaded private data while the next account is loading', async () => {
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  release('fixture-a', 'A', 1)
  await vi.waitFor(() => expect(view.text()).toContain('A-private-campaign'))
  await login('b')
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('—'))
  expect(view.text()).not.toContain('A-private')
  expect(view.get('[data-testid="play-hub-shell"]').attributes('aria-busy')).toBe('true')
  await vi.waitFor(() => expect(waiting('fixture-b')).toHaveLength(2))
  release('fixture-b', 'B', 2)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$2.00'))
})

it('keeps the new account loading when an obsolete request fails first', async () => {
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  await login('b')
  await vi.waitFor(() => expect(waiting('fixture-b')).toHaveLength(2))
  for (const r of waiting('fixture-a')) json(r.res, 503, {})
  await new Promise(resolve => setTimeout(resolve, 30))
  expect(view.get('[data-testid="play-hub-shell"]').attributes('aria-busy')).toBe('true')
  expect(view.find('[role="alert"]').exists()).toBe(false)
  release('fixture-b', 'B', 2)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$2.00'))
})

it('discards pending private responses after logout and performs no guest refresh', async () => {
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  await auth.logout()
  release('fixture-a', 'A', 1)
  await new Promise(resolve => setTimeout(resolve, 30))
  expect(view.get('.gw-balance-value').text()).toBe('—')
  expect(view.text()).not.toContain('A-private')
  await view.get('[data-testid="refresh-status"]').trigger('click')
  await new Promise(resolve => setTimeout(resolve, 30))
  expect(reads).toHaveLength(2)
  expect(view.find('[role="alert"]').exists()).toBe(false)
})

it('clears ready data on logout and restores reads after login without remounting', async () => {
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  release('fixture-a', 'A', 1)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$1.00'))
  await auth.logout()
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('—'))
  expect(view.text()).not.toContain('A-private')
  await login('b')
  await vi.waitFor(() => expect(waiting('fixture-b')).toHaveLength(2))
  release('fixture-b', 'B', 2)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$2.00'))
})

it('recovers when the store adopts a token rotated by its real automatic refresh', async () => {
  expireSoon = true
  await login('a')
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  await vi.waitFor(() => expect(refreshes).toBe(1), { timeout: 3000 })
  await vi.waitFor(() => expect(localStorage.getItem('auth_token')).toBe('fixture-renewed'))
  // Production tokenRefresh persists first; checkAuth adopts that token pair.
  auth.checkAuth()
  await vi.waitFor(() => expect(auth.token).toBe('fixture-renewed'))
  await vi.waitFor(() => expect(waiting('fixture-renewed')).toHaveLength(2))
  release('fixture-renewed', 'renewed', 3)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$3.00'))
  release('fixture-a', 'obsolete', 1)
  await new Promise(resolve => setTimeout(resolve, 30))
  expect(view.get('.gw-balance-value').text()).toBe('$3.00')
  expect(view.text()).toContain('renewed-private-campaign')
  expect(view.text()).not.toContain('obsolete-private')
  expect(reads).toHaveLength(4)
})

it('completes an automatic Axios 401 refresh without discarding the successful retry', async () => {
  respond = (req, res) => {
    if (req.headers.authorization === 'Bearer fixture-a') json(res, 401, {})
    else replyRead(res, new URL(req.url || '/', 'http://fixture.test').pathname, 'renewed', 3)
  }
  const view = mountView()
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$3.00'))
  expect(refreshes).toBe(1)
  expect(localStorage.getItem('auth_token')).toBe('fixture-renewed')
  expect(view.text()).toContain('renewed-private-campaign')
  expect(view.get('[data-testid="play-hub-shell"]').attributes('aria-busy')).toBe('false')
})

it('does not revive an earlier session when the same account and token return after B', async () => {
  const view = mountView()
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(2))
  const oldA = [...waiting('fixture-a')]
  await login('b')
  await vi.waitFor(() => expect(waiting('fixture-b')).toHaveLength(2))
  await login('a')
  await vi.waitFor(() => expect(waiting('fixture-a')).toHaveLength(4))
  for (const r of waiting('fixture-a').filter(r => !oldA.includes(r))) replyRead(r.res, r.path, 'current-A', 5)
  await vi.waitFor(() => expect(view.get('.gw-balance-value').text()).toBe('$5.00'))
  for (const r of oldA) replyRead(r.res, r.path, 'obsolete-A', 1)
  release('fixture-b', 'obsolete-B', 2)
  await new Promise(resolve => setTimeout(resolve, 30))
  expect(view.get('.gw-balance-value').text()).toBe('$5.00')
  expect(view.text()).toContain('current-A-private-campaign')
  expect(view.text()).not.toContain('obsolete-')
})
