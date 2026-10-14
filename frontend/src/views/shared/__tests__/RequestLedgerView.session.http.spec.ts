// Actual page, Pinia/auth actions, Axios interceptors and HTTP. Only the layout is stubbed.
import { createServer, type Server, type ServerResponse } from 'node:http'
import axios from 'axios'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import { AUTH_SESSION_KEY, finishAuthSession, getAuthSessionIdentity } from '@/api/tokenRefresh'
import RequestLedgerView from '../RequestLedgerView.vue'
import en from '@/i18n/locales/en'
import { holdHttpBoundary } from './fixtures/sessionHttpBoundary'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const endpoint = vi.hoisted(() => ({ base: '' }))
vi.mock('@/api/url', () => ({ getAPIBaseURL: () => endpoint.base }))
const id = '00000000-0000-4000-8000-000000000001'
type Read = { token: string; stage: string; res: ServerResponse; cursor: string; closed: boolean }
let server: Server, router: Router, pinia: ReturnType<typeof createPinia>, auth: ReturnType<typeof useAuthStore>
let wrapper: VueWrapper | undefined
let boundary: ReturnType<typeof holdHttpBoundary> | undefined
let role = 'user', hold = '', failB = false, renewSoon = false, unauthorizedOnce = false, refreshes = 0
const reads: Read[] = []
const saved = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawAdapter = axios.defaults.adapter
const reply = (res: ServerResponse, data: unknown, status = 200) => {
  res.writeHead(status, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify({ code: status === 200 ? 0 : status, data }))
}
const user = (account: string) => ({ id: account === 'a' ? 1 : 2, role, email: `${account}@example.test`, balance: 94 })
const record = (label: string) => ({ id, route: `/private-${label}`, method: 'POST', kind: 'http', user_id: 1, api_key_id: 201, started_at: '2026-10-11T00:00:00Z', execution_state: 'succeeded', usage_state: 'known', settlement_state: 'settled', attempt_count: 1, output_observed: true, error_code: '', http_status: 200 })
function complete(read: Read, label = read.token, status = 200) {
  const request = record(label)
  if (status !== 200) { reply(read.res, {}, status); return }
  if (read.stage === 'list') reply(read.res, { items: [request], total: 1, page: 1, page_size: 20 })
  else if (read.stage === 'summary') reply(read.res, { request })
  else if (read.stage === 'usage') reply(read.res, { id: 42, model: `private-${label}-usage`, billing_mode: 'image', image_count: 3, billed_cost: 6, batch_id: `private-${label}-batch` })
  else reply(read.res, { items: read.stage === 'attempts' ? [{ attempt_no: 1, attempt_id: `private-${label}-attempt`, phase: 'request', usage_state: 'known', execution_state: 'succeeded', upstream_request_id: `private-${label}-upstream` }]
    : read.stage === 'billing' ? [{ usage_log_id: 42, billed_cost: 6, settled: true, batch_id: `private-${label}-batch` }]
      : read.stage === 'diagnostics' ? [{ attempt_no: 1, ops_error_log_id: 42, available: false }] : [{ id: 99, operation: 'image_balance_capture' }],
  page_size: 20, has_more: !read.cursor, next_cursor: read.cursor ? '' : 'second', consistency: 'live', refresh_hint: 'new_records_late_evidence_and_state_changes' })
}
const pending = (token: string, stage: string) => reads.filter(r => r.token === token && r.stage === stage && !r.res.writableEnded)
const login = (account: string) => auth.login({ email: `${account}@example.test`, password: 'fixture-only' })
const pause = () => new Promise(resolve => setTimeout(resolve, 30))
beforeAll(async () => {
  server = createServer(async (req, res) => {
    const url = new URL(req.url || '/', 'http://fixture.test'), token = String(req.headers.authorization).replace('Bearer ', '')
    if (url.pathname.endsWith('/auth/login')) {
      let body = ''; for await (const chunk of req) body += chunk
      const account = JSON.parse(body).email.startsWith('b@') ? 'b' : 'a'
      reply(res, { access_token: `fixture-${account}`, refresh_token: `refresh-${account}`, ...(renewSoon ? { expires_in: 121 } : {}), user: user(account) })
    } else if (url.pathname.endsWith('/auth/refresh')) {
      refreshes++; reply(res, { access_token: 'fixture-renewed', refresh_token: 'refresh-renewed', expires_in: 3600, token_type: 'Bearer' })
    } else if (url.pathname.endsWith('/auth/logout')) reply(res, {})
    else if (url.pathname.endsWith('/auth/me')) reply(res, user(token === 'fixture-b' ? 'b' : 'a'))
    else if (url.pathname.includes('/requests')) {
      const stage = url.pathname.includes('/usage/') ? 'usage' : url.searchParams.get('section') || (url.searchParams.has('view') ? 'summary' : 'list')
      const read = { token, stage, res, cursor: url.searchParams.get('cursor') || '', closed: false }; reads.push(read)
      res.once('close', () => { read.closed = true })
      if (unauthorizedOnce && stage === 'attempts') { unauthorizedOnce = false; reply(res, {}, 401) }
      else if (failB && token === 'fixture-b') reply(res, {}, 503)
      else if (stage !== hold && !(hold === 'sections' && ['attempts', 'billing', 'wallet', 'diagnostics'].includes(stage))) complete(read)
    } else reply(res, {}, 404)
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing HTTP fixture')
  endpoint.base = `http://127.0.0.1:${address.port}/api/v1`
  Object.assign(apiClient.defaults, { baseURL: endpoint.base, adapter: 'http' }); axios.defaults.adapter = 'http'
})
beforeEach(async () => {
  localStorage.clear(); role = 'user'; hold = ''; failB = false; renewSoon = false; unauthorizedOnce = false; refreshes = 0; reads.length = 0
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore(); await login('a')
  router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/requests', name: 'Requests', component: { template: '<div />' } }, { path: '/usage', name: 'Usage', component: { template: '<div />' } }, { path: '/admin/usage', name: 'AdminUsage', component: { template: '<div />' } }] })
  await router.push(`/requests?private_id=${id}&lang=en`); await router.isReady()
})
afterEach(async () => {
  vi.useRealTimers()
  boundary?.release(); boundary = undefined; apiClient.defaults.adapter = 'http'
  try { wrapper?.unmount(); await auth.logout() }
  finally { wrapper = undefined; auth.$dispose(); server.closeAllConnections() }
})
afterAll(async () => { Object.assign(apiClient.defaults, saved); axios.defaults.adapter = rawAdapter; await new Promise<void>(resolve => server.close(() => resolve())); vi.unstubAllGlobals() })
async function start(admin = false) {
  if (admin) { role = 'admin'; await login('a'); router.currentRoute.value.meta.requiresAdmin = true }
  mountPage()
  await vi.waitFor(() => expect(reads.some(r => r.stage === 'list')).toBe(true))
  return wrapper!
}
function mountPage() {
  wrapper = mount(RequestLedgerView, { global: { plugins: [pinia, router, createI18n({ legacy: false, locale: 'en', messages: { en } })], stubs: { AppLayout: { template: '<main><slot /></main>' }, Teleport: true } } })
  return wrapper
}
// A response's server-side close is asynchronous even after the page aborts its read.
// Keep the destruction assertion; a missing transport cancellation must still fail.
async function waitForClosed(read: Read) {
  await vi.waitFor(() => {
    expect(read.closed).toBe(true)
    expect(read.res.destroyed).toBe(true)
  })
}
// A rendered attempt does not imply that sibling HTTP requests have arrived.
// Require the entire initial user request set and finished UI reads before a count
// snapshot. Exact membership also catches unexpected/duplicate initial requests.
async function waitForInitialUserReads(view: VueWrapper) {
  await vi.waitFor(() => {
    expect(reads.map(read => read.stage).sort()).toEqual(['attempts', 'billing', 'list', 'summary', 'wallet'])
    expect(reads.every(read => read.res.writableFinished)).toBe(true)
    expect(view.find('[aria-busy="true"]').exists()).toBe(false)
  })
}
async function ready(admin = false) {
  const view = await start(admin)
  await vi.waitFor(() => { expect(view.text()).toContain('private-fixture-a-attempt'); expect(view.text()).toContain('Usage record #42') })
  if (!admin) await waitForInitialUserReads(view)
  else await vi.waitFor(() => expect(view.find('[aria-busy="true"]').exists()).toBe(false))
  return view
}
it.each(['summary', 'billing'])('waits for server close after an already requested %s abort', async stage => {
  boundary = holdHttpBoundary(stage, 'abort'); apiClient.defaults.adapter = boundary.adapter
  hold = stage
  const view = await start()
  await vi.waitFor(() => expect(pending('fixture-a', stage)).toHaveLength(1))
  const old = pending('fixture-a', stage)[0]!
  hold = ''; failB = true; await login('b')
  await vi.waitFor(() => expect(boundary!.state.reached).toBe(true))
  expect(old.res.destroyed).toBe(false)
  // Advance beyond the former 30ms observation deadline without sleeping or
  // releasing the transport. Elapsed time alone must not finish this wait.
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  let result = 'pending'
  const closing = waitForClosed(old).then(() => { result = 'closed' }, () => { result = 'failed' })
  await vi.advanceTimersByTimeAsync(30)
  expect(result).toBe('pending')
  vi.useRealTimers()
  boundary.release(); await closing
  expect(result).toBe('closed')
  complete(old, 'obsolete-A'); await pause()
  expect(view.text()).not.toContain('obsolete-A')
  expect(view.text()).not.toContain('private-fixture-a')
})
it.each(['billing', 'wallet'])('waits for the initial request set while %s has not arrived', async stage => {
  boundary = holdHttpBoundary(stage, 'arrival'); apiClient.defaults.adapter = boundary.adapter
  const view = await start()
  await vi.waitFor(() => expect(view.text()).toContain('private-fixture-a-attempt'))
  expect(boundary.state.reached).toBe(true)
  expect(reads.some(read => read.stage === stage)).toBe(false)
  let ready = false
  const settling = waitForInitialUserReads(view).then(() => { ready = true })
  await flushPromises(); expect(ready).toBe(false)
  boundary.release(); await settling
  expect(reads).toHaveLength(5)
})
it.each([false, true])('clears loaded details/list on same-role account switch even when B fails (admin=%s)', async admin => {
  const view = await ready(admin)
  await view.findAll('button').find(b => b.text() === 'Usage record #42')!.trigger('click')
  await vi.waitFor(() => expect(view.text()).toContain('private-fixture-a-usage'))
  failB = true; await login('b'); await pause(); await flushPromises()
  expect(view.text()).not.toContain('private-fixture-a')
  expect(view.find('[data-testid="ledger-section-billing"]').exists()).toBe(false)
  expect((view.get('[name="private_id"]').element as HTMLInputElement).value).toBe('')
  expect(router.currentRoute.value.query.private_id).toBeUndefined()
  await vi.waitFor(() => expect(reads.some(r => r.token === 'fixture-b')).toBe(true))
  expect(view.text()).not.toContain(id)
})
it.each(['list', 'summary', 'attempts', 'billing', 'wallet', 'diagnostics', 'usage'])('cancels late old %s success without unmounting the page', async stage => {
  hold = stage
  const view = await start(stage === 'diagnostics')
  if (stage === 'usage') {
    await vi.waitFor(() => expect(view.text()).toContain('Usage record #42'))
    await view.findAll('button').find(b => b.text() === 'Usage record #42')!.trigger('click')
  }
  await vi.waitFor(() => expect(pending('fixture-a', stage)).toHaveLength(1))
  const old = pending('fixture-a', stage)[0]!
  hold = ''; failB = true; await login('b')
  await waitForClosed(old)
  complete(old, 'obsolete-A'); await pause()
  expect(view.text()).not.toContain('obsolete-A')
  expect(view.text()).not.toContain('private-fixture-a')
})
it('clears on logout synchronously, does no guest read, and isolates A→B→A', async () => {
  const view = await ready(); hold = 'attempts'
  await view.get('[data-testid="ledger-section-attempts"] [data-action="next"]').trigger('click')
  await vi.waitFor(() => expect(pending('fixture-a', 'attempts')).toHaveLength(1))
  const old = pending('fixture-a', 'attempts')[0]!
  await auth.logout(); await flushPromises()
  expect(view.text()).not.toContain('private-fixture-a')
  const count = reads.length; await view.get('[data-testid="ledger-refresh"]').trigger('click'); await pause(); expect(reads).toHaveLength(count)
  hold = ''; await login('b'); await vi.waitFor(() => expect(view.text()).toContain('private-fixture-b'))
  await login('a'); await vi.waitFor(() => expect(view.text()).toContain('private-fixture-a'))
  complete(old, 'obsolete-A'); await pause(); expect(view.text()).not.toContain('obsolete-A')
})
it('treats a fresh login to the same account as a new session', async () => {
  const view = await ready(); await login('a'); await pause()
  expect(view.find('[data-testid="ledger-section-attempts"]').exists()).toBe(false)
  expect(router.currentRoute.value.query.private_id).toBeUndefined()
})
it('preserves a pending page and its cursor across real automatic token renewal', async () => {
  renewSoon = true; await login('a'); const view = await ready(); hold = 'attempts'
  await view.get('[data-testid="ledger-section-attempts"] [data-action="next"]').trigger('click')
  await vi.waitFor(() => expect(pending('fixture-a', 'attempts')).toHaveLength(1))
  const old = pending('fixture-a', 'attempts')[0]!, count = reads.length
  await vi.waitFor(() => expect(refreshes).toBe(1), { timeout: 3000 })
  // Public session restoration must preserve the identity of a normal rotation.
  auth.checkAuth(); await vi.waitFor(() => expect(auth.token).toBe('fixture-renewed'))
  expect(old.res.destroyed).toBe(false); expect(reads).toHaveLength(count)
  complete(old, 'renewal-kept'); await vi.waitFor(() => expect(view.text()).toContain('private-renewal-kept-attempt'))
  expect(view.get('[data-testid="ledger-section-attempts"] [data-action="previous"]').attributes('disabled')).toBeUndefined()
})
it('accepts the real Axios 401 refresh retry within the same account', async () => {
  unauthorizedOnce = true; const view = await start()
  await vi.waitFor(() => expect(view.text()).toContain('private-fixture-renewed-attempt'))
  expect(refreshes).toBe(1); expect(view.find('[data-testid="ledger-section-attempts"] [role="alert"]').exists()).toBe(false)
})


it.each(['attempts', 'billing', 'wallet', 'diagnostics'].flatMap(section => [200, 503].map(status => ({ section, status }))))('closes/reopens a real dialog while $section returns stale HTTP $status', async ({ section, status }) => {
  const view = await ready(true), selector = `[data-testid="ledger-section-${section}"]`
  hold = section
  await view.get(`${selector} [data-action="next"]`).trigger('click')
  await vi.waitFor(() => expect(pending('fixture-a', section)).toHaveLength(1))
  const old = pending('fixture-a', section)[0]!
  await view.get('button[aria-label="Close"]').trigger('click')
  expect(view.find(selector).exists()).toBe(false)
  await waitForClosed(old)
  await view.findAll('button').find(b => b.text() === 'Details')!.trigger('click')
  await vi.waitFor(() => expect(pending('fixture-a', section)).toHaveLength(2))
  const fresh = pending('fixture-a', section).find(r => r !== old)!
  complete(old, 'obsolete-closed', status); await pause()
  expect(view.get(selector).attributes('aria-busy')).toBe('true')
  expect(view.find(`${selector} [role="alert"]`).exists()).toBe(false)
  expect(view.text()).not.toContain('obsolete-closed')
  complete(fresh, 'current-reopen'); await vi.waitFor(() => expect(view.get(selector).attributes('aria-busy')).toBe('false'))
  expect(view.get(`${selector} [data-action="previous"]`).attributes('disabled')).toBeDefined()
  expect(view.get(`${selector} [data-action="next"]`).attributes('disabled')).toBeUndefined()
})

it('keeps all four sections independent when replies and a retry complete out of order', async () => {
  const view = await ready(true), sections = ['attempts', 'billing', 'wallet', 'diagnostics']
  hold = 'sections'
  for (const section of sections) {
    await vi.waitFor(() => expect(view.get(`[data-testid="ledger-section-${section}"]`).attributes('aria-busy')).toBe('false'))
    await view.get(`[data-testid="ledger-section-${section}"] [data-action="next"]`).trigger('click')
  }
  await vi.waitFor(() => expect(sections.every(s => pending('fixture-a', s).length === 1)).toBe(true))
  const region = (s: string) => view.get(`[data-testid="ledger-section-${s}"]`)
  for (const section of ['wallet', 'billing', 'diagnostics', 'attempts']) {
    complete(pending('fixture-a', section)[0]!, `second-${section}`, section === 'billing' ? 503 : 200)
    await vi.waitFor(() => expect(region(section).attributes('aria-busy')).toBe('false'))
  }
  expect(region('billing').find('[role="alert"]').exists()).toBe(true)
  for (const section of ['attempts', 'wallet', 'diagnostics']) {
    expect(region(section).find('[role="alert"]').exists()).toBe(false)
    expect(region(section).get('[data-action="previous"]').attributes('disabled')).toBeUndefined()
  }
  const count = reads.length
  await region('billing').get('[data-action="retry"]').trigger('click')
  await vi.waitFor(() => expect(pending('fixture-a', 'billing')).toHaveLength(1))
  expect(pending('fixture-a', 'billing')[0]!.cursor).toBe('second')
  complete(pending('fixture-a', 'billing')[0]!, 'retry-billing')
  await vi.waitFor(() => expect(region('billing').find('[role="alert"]').exists()).toBe(false))
  expect(reads).toHaveLength(count + 1)
})

// A peer tab publishes new credentials under the same official session marker.
// Use the wire format only to arrange the peer state; production reads the API.
function beginPeerRotation() {
  const marker = `pending:${getAuthSessionIdentity().sessionID}|fixture-peer-publication`
  localStorage.setItem(AUTH_SESSION_KEY, marker)
  window.dispatchEvent(new StorageEvent('storage', { key: AUTH_SESSION_KEY, newValue: marker, storageArea: localStorage }))
  return () => {
    expect(finishAuthSession(marker)).toBe(true)
    window.dispatchEvent(new StorageEvent('storage', { key: AUTH_SESSION_KEY, oldValue: marker, newValue: localStorage.getItem(AUTH_SESSION_KEY), storageArea: localStorage }))
  }
}
it('resumes initial reads mounted during same-session peer publication when ready', async () => {
  const finish = beginPeerRotation(), view = mountPage()
  await pause(); expect(reads).toHaveLength(0)
  finish()
  await vi.waitFor(() => expect(view.text()).toContain('private-fixture-a-attempt'))
  expect(reads.filter(r => r.stage === 'list')).toHaveLength(1)
  expect(reads.filter(r => r.stage === 'summary')).toHaveLength(1)
  expect(router.currentRoute.value.query.private_id).toBe(id)
})
it('resumes an explicit section cursor after peer readiness and cancels a closed waiting read', async () => {
  const view = await ready(), selector = '[data-testid="ledger-section-attempts"]'
  const finish = beginPeerRotation(), count = reads.length
  await view.get(`${selector} [data-action="next"]`).trigger('click')
  await pause(); expect(reads).toHaveLength(count)
  expect(view.get(selector).attributes('aria-busy')).toBe('true')
  finish()
  await vi.waitFor(() => expect(reads).toHaveLength(count + 1))
  expect(reads.at(-1)!.cursor).toBe('second')
  await vi.waitFor(() => expect(view.get(selector).attributes('aria-busy')).toBe('false'))
  expect(view.get(`${selector} [data-action="previous"]`).attributes('disabled')).toBeUndefined()
  const finishAgain = beginPeerRotation()
  await view.get(`${selector} [data-action="previous"]`).trigger('click')
  await view.get('button[aria-label="Close"]').trigger('click')
  finishAgain(); await pause()
  expect(reads).toHaveLength(count + 1)
  expect(view.find(selector).exists()).toBe(false)
})
it('cancels a waiting old cursor before a new identity can dispatch it', async () => {
  const view = await ready(), selector = '[data-testid="ledger-section-attempts"]'
  beginPeerRotation()
  const count = reads.length
  await view.get(`${selector} [data-action="next"]`).trigger('click')
  await pause(); expect(reads).toHaveLength(count)
  failB = true; await login('b')
  await vi.waitFor(() => expect(reads.some(r => r.token === 'fixture-b')).toBe(true))
  expect(reads.slice(count).every(r => r.token === 'fixture-b' && r.stage === 'list' && r.cursor === '')).toBe(true)
  expect(view.find(selector).exists()).toBe(false)
  expect(view.text()).not.toContain('private-fixture-a')
})
it('ends abandoned publication with retryable errors and resumes after ready', async () => {
  const finish = beginPeerRotation(), view = mountPage()
  await vi.waitFor(() => expect(view.find('[data-testid="ledger-retry"]').exists()).toBe(true), { timeout: 6500 })
  expect(reads).toHaveLength(0)
  expect(view.get('[data-testid="ledger-refresh"]').attributes('disabled')).toBeUndefined()
  finish()
  await view.get('[data-testid="ledger-retry"]').trigger('click')
  await view.findAll('button').find(b => b.text() === 'Retry' && !b.attributes('data-testid'))!.trigger('click')
  await vi.waitFor(() => expect(view.text()).toContain('private-fixture-a-attempt'))
  await waitForInitialUserReads(view)
  const finishAgain = beginPeerRotation(), selector = '[data-testid="ledger-section-attempts"]', count = reads.length
  await view.get(`${selector} [data-action="next"]`).trigger('click')
  await vi.waitFor(() => expect(view.find(`${selector} [data-action="retry"]`).exists()).toBe(true), { timeout: 6500 })
  expect(reads).toHaveLength(count)
  expect(view.text()).toContain('private-fixture-a-attempt')
  finishAgain(); await view.get(`${selector} [data-action="retry"]`).trigger('click')
  await vi.waitFor(() => expect(view.get(`${selector} [data-action="previous"]`).attributes('disabled')).toBeUndefined())
  expect(reads.at(-1)!.cursor).toBe('second')
}, 16000)
