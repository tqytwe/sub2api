// Frontend contract simulation for PR359: real HTTP, actual page/Pinia/Axios; no backend/PG claim.
import { Agent, createServer, type Server, type ServerResponse } from 'node:http'
import axios from 'axios'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import { authAPI } from '@/api/auth'
import RequestLedgerView from '../RequestLedgerView.vue'
import en from '@/i18n/locales/en'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const endpoint = vi.hoisted(() => ({ base: '' }))
vi.mock('@/api/url', () => ({ getAPIBaseURL: () => endpoint.base }))
const id = '00000000-0000-4000-8000-000000000001'
type Read = { path: string; token: string; stage: string; res: ServerResponse; cursor: string }
let server: Server, router: Router, pinia: ReturnType<typeof createPinia>, auth: ReturnType<typeof useAuthStore>
let wrapper: VueWrapper | undefined
let routingStatus = 'complete', routingHTTP = 200, routingEvents = 3, wrongRequest = false, malformed = false
let role = 'user', hold = '', failB = false, renewSoon = false, unauthorizedOnce = false, refreshes = 0
const reads: Read[] = []
let holdAuth = false
const authReplies: {res: ServerResponse; data: unknown}[] = []
const saved = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawAdapter = axios.defaults.adapter
const previousAgent = apiClient.defaults.httpAgent, rawAgent = axios.defaults.httpAgent
const httpAgent = new Agent({ keepAlive: false })
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
  else if (read.stage === 'routing') reply(read.res, malformed ? {events: []} : routing(label), routingHTTP)
  else if (read.stage === 'summary') reply(read.res, { request })
  else if (read.stage === 'usage') reply(read.res, { id: 42, model: `private-${label}-usage`, billing_mode: 'image', image_count: 3, billed_cost: 6, batch_id: `private-${label}-batch` })
  else reply(read.res, { items: read.stage === 'attempts' ? [{ attempt_no: 1, attempt_id: `private-${label}-attempt`, phase: 'request', usage_state: 'known', execution_state: 'succeeded', upstream_request_id: `private-${label}-upstream` }]
    : read.stage === 'billing' ? [{ usage_log_id: 42, billed_cost: 6, settled: true, batch_id: `private-${label}-batch` }]
      : read.stage === 'diagnostics' ? [{ attempt_no: 1, ops_error_log_id: 42, available: false }] : [{ id: 99, operation: 'image_balance_capture' }],
  page_size: 20, has_more: !read.cursor, next_cursor: read.cursor ? '' : 'second', consistency: 'live', refresh_hint: 'new_records_late_evidence_and_state_changes' })
}
function routing(label: string) {
  return { request_id: wrongRequest ? '00000000-0000-4000-8000-000000000002' : id, coverage_version: routingStatus === 'not_recorded' ? '' : 'openai_compatible_http_text_v1', status: routingStatus, selection_count: 2, event_count: routingEvents, veto_count: 1, omitted_count: routingStatus === 'truncated' ? 7 : 0, incomplete: routingStatus === 'incomplete', source_population_complete: false, events: ['not_recorded', 'unsupported_coverage'].includes(routingStatus) ? [] : Array.from({length:routingEvents}, (_, i) => ({selection_no: i < 2 ? 1 : 2, stage: i === 0 ? 'pool_source' : i === 1 ? 'initial_filter' : 'selection', reason: i === 0 ? 'prefiltered_schedulable_pool' : i === 1 ? 'model_rate_limited' : 'selection_selected', ...(i > 0 ? {account_id: label.includes('obsolete') ? 987654 : 234 + i} : {}), value: 0, at: '2026-10-11T00:00:00Z'})) }
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
      const data = { access_token: `fixture-${account}`, refresh_token: `refresh-${account}`, ...(renewSoon ? { expires_in: 121 } : {}), user: user(account) }
      if (holdAuth) authReplies.push({ res, data }); else reply(res, data)
    } else if (url.pathname.endsWith('/auth/refresh')) {
      refreshes++; reply(res, { access_token: 'fixture-renewed', refresh_token: 'refresh-renewed', expires_in: 3600, token_type: 'Bearer' })
    } else if (url.pathname.endsWith('/auth/logout')) reply(res, {})
    else if (url.pathname.endsWith('/auth/me')) reply(res, user(token === 'fixture-b' ? 'b' : 'a'))
    else if (url.pathname.includes('/requests')) {
      if (url.pathname.endsWith('/routing') && (!token.startsWith('fixture-') || role !== 'admin')) { reply(res, {}, token.startsWith('fixture-') ? 403 : 401); return }
      const stage = url.pathname.endsWith('/routing') ? 'routing' : url.pathname.includes('/usage/') ? 'usage' : url.searchParams.get('section') || (url.searchParams.has('view') ? 'summary' : 'list')
      const read = { path: url.pathname, token, stage, res, cursor: url.searchParams.get('cursor') || '' }; reads.push(read)
      if (unauthorizedOnce && stage === 'attempts') { unauthorizedOnce = false; reply(res, {}, 401) }
      else if (failB && token === 'fixture-b') reply(res, {}, 503)
      else if (stage !== hold && !(hold === 'sections' && ['attempts', 'billing', 'wallet', 'diagnostics'].includes(stage))) complete(read)
    } else reply(res, {}, 404)
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing HTTP fixture')
  endpoint.base = `http://127.0.0.1:${address.port}/api/v1`
  Object.assign(apiClient.defaults, { baseURL: endpoint.base, adapter: 'http', httpAgent }); axios.defaults.adapter = 'http'; axios.defaults.httpAgent = httpAgent
})
beforeEach(async () => {
  holdAuth = false; authReplies.length = 0
  routingStatus = 'complete'; routingHTTP = 200; routingEvents = 3; wrongRequest = false; malformed = false
  localStorage.clear(); role = 'user'; hold = ''; failB = false; renewSoon = false; unauthorizedOnce = false; refreshes = 0; reads.length = 0
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore(); await login('a')
  router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/requests', name: 'Requests', component: { template: '<div />' } }, { path: '/usage', name: 'Usage', component: { template: '<div />' } }, { path: '/admin/usage', name: 'AdminUsage', component: { template: '<div />' } }] })
  await router.push(`/requests?private_id=${id}&lang=en`); await router.isReady()
})
afterEach(async () => {
  try { vi.restoreAllMocks(); wrapper?.unmount(); await auth.logout() }
  finally { wrapper = undefined; auth.$dispose(); server.closeAllConnections() }
})
afterAll(async () => { Object.assign(apiClient.defaults, saved, { httpAgent: previousAgent }); axios.defaults.adapter = rawAdapter; axios.defaults.httpAgent = rawAgent; httpAgent.destroy(); await new Promise<void>(resolve => server.close(() => resolve())); vi.unstubAllGlobals() })
async function start(admin = false) {
  if (admin) { role = 'admin'; await login('a'); router.getRoutes().find(r => r.path === '/requests')!.meta.requiresAdmin = true; router.currentRoute.value.meta.requiresAdmin = true }
  mountPage()
  await vi.waitFor(() => expect(reads.some(r => r.stage === 'list')).toBe(true))
  return wrapper!
}
function mountPage() {
  wrapper = mount(RequestLedgerView, { global: { plugins: [pinia, router, createI18n({ legacy: false, locale: 'en', messages: { en } })], stubs: { AppLayout: { template: '<main><slot /></main>' }, Teleport: true } } })
  return wrapper
}
async function ready(admin = false) {
  const view = await start(admin)
  await vi.waitFor(() => { expect(view.text()).toContain('private-fixture-a-attempt'); expect(view.text()).toContain('Usage record #42') })
  return view
}

const region = (view: VueWrapper) => view.get('[data-testid="ledger-section-routing"]')
it('reads the exact independent admin endpoint and identifies the recorded veto, not pool/selection', async () => {
  const view = await ready(true)
  await vi.waitFor(() => expect(reads.filter(r => r.stage === 'routing')).toHaveLength(1))
  expect(reads.find(r => r.stage === 'routing')!.path).toBe(`/api/v1/admin/requests/${id}/routing`)
  await vi.waitFor(() => expect(region(view).text()).toContain('Model rate limited'))
  expect(region(view).get('[data-testid="routing-first-veto"]').text()).toContain('235')
  expect(region(view).get('[data-testid="routing-first-veto"]').text()).not.toContain('236')
  expect(region(view).text()).toContain('does not prove forwarding')
  expect(region(view).text()).toContain('does not prove that the group has no accounts')
  expect(region(view).text()).toContain('openai_compatible_http_text_v1')
})
it.each(['complete', 'truncated', 'pending', 'incomplete', 'not_recorded', 'unsupported_coverage'])('distinguishes routing %s without changing billing/usage', async status => {
  routingStatus = status
  const view = await ready(true)
  await vi.waitFor(() => expect(region(view).find(`[data-routing-status="${status}"]`).exists()).toBe(true))
  expect(region(view).text()).toContain('Anthropic')
  expect(view.get('[data-testid="ledger-section-billing"]').text()).toContain('$6.0000')
  if (status === 'truncated') expect(region(view).get('[data-testid="routing-omitted"]').text()).toContain('7')
})
it('pages only returned routing events, keeps ledger cursor, and refreshes this region alone', async () => {
  routingEvents = 45
  const view = await ready(true)
  await vi.waitFor(() => expect(region(view).attributes('aria-busy')).toBe('false'))
  await view.get('[data-testid="ledger-section-attempts"] [data-action="next"]').trigger('click')
  await vi.waitFor(() => expect(view.get('[data-testid="ledger-section-attempts"]').text()).toContain('Page 2'))
  const count = reads.length
  await region(view).get('[data-action="next"]').trigger('click')
  expect(region(view).get('[data-testid="routing-page"]').text()).toContain('21–40')
  expect(reads).toHaveLength(count)
  await region(view).get('[data-action="refresh"]').trigger('click')
  await vi.waitFor(() => expect(reads).toHaveLength(count + 1))
  await vi.waitFor(() => expect(region(view).get('[data-testid="routing-page"]').text()).toContain('1–20'))
  expect(view.get('[data-testid="ledger-section-attempts"]').text()).toContain('Page 2')
})
it.each([403, 404, 503])('isolates routing HTTP %s and retries without losing usage', async status => {
  routingHTTP = status
  const view = await ready(true)
  await vi.waitFor(() => expect(region(view).find('[role="alert"]').exists()).toBe(true))
  await view.findAll('button').find(b => b.text() === 'Usage record #42')!.trigger('click')
  await vi.waitFor(() => expect(view.text()).toContain('private-fixture-a-usage'))
  routingHTTP = 200; await region(view).get('[data-action="retry"]').trigger('click')
  await vi.waitFor(() => expect(region(view).find('[role="alert"]').exists()).toBe(false))
  expect(view.text()).toContain('private-fixture-a-usage')
})
it.each(['wrong ID', 'malformed'])('rejects %s response only in routing region', async mode => {
  wrongRequest = mode === 'wrong ID'; malformed = mode === 'malformed'
  const view = await ready(true)
  await vi.waitFor(() => expect(region(view).find('[role="alert"]').exists()).toBe(true))
  expect(view.get('[data-testid="ledger-section-billing"]').text()).toContain('$6.0000')
})
it('never mounts or requests admin routing for ordinary users and denies direct user/guest reads', async () => {
  const view = await ready(false)
  expect(view.find('[data-testid="ledger-section-routing"]').exists()).toBe(false)
  expect(reads.filter(r => r.stage === 'routing')).toHaveLength(0)
  expect(view.text()).not.toContain('private-fixture-a-upstream')
  const target = `${endpoint.base}/admin/requests/${id}/routing`
  expect((await axios.get(target, {headers:{Authorization:'Bearer fixture-a'}, validateStatus:()=>true})).status).toBe(403)
  expect((await axios.get(target, {validateStatus:()=>true})).status).toBe(401)
})
it.each([200,503])('ignores late closed routing HTTP %s after reopening', async status => {
  hold = 'routing'; const view = await ready(true)
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
  const old = pending('fixture-a','routing')[0]!
  await view.get('button[aria-label="Close"]').trigger('click')
  await vi.waitFor(() => expect(old.res.destroyed).toBe(true))
  await view.findAll('button').find(b => b.text() === 'Details')!.trigger('click')
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(2))
  complete(old,'obsolete-closed',status); await pause()
  expect(region(view).attributes('aria-busy')).toBe('true')
  expect(region(view).find('[role="alert"]').exists()).toBe(false)
  complete(pending('fixture-a','routing').find(r=>r!==old)!)
  await vi.waitFor(() => expect(region(view).attributes('aria-busy')).toBe('false'))
  expect(region(view).text()).not.toContain('987654')
})
it.each(['switch', 'logout', 'same-account-login'])('clears and rejects late routing on %s via real Pinia actions', async action => {
  hold = 'routing'; const view = await ready(true)
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
  const old = pending('fixture-a','routing')[0]!
  if (action === 'logout') await auth.logout()
  else { failB = true; await login(action === 'switch' ? 'b' : 'a') }
  await vi.waitFor(() => expect(old.res.destroyed).toBe(true))
  complete(old,'obsolete-identity'); await pause()
  expect(view.find('[data-testid="ledger-section-routing"]').exists()).toBe(false)
  expect(view.text()).not.toContain('987654')
  expect(router.currentRoute.value.query.private_id).toBeUndefined()
})
it('keeps pending routing through real same-session token renewal', async () => {
  renewSoon = true; hold = 'routing'; const view = await ready(true)
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
  const old = pending('fixture-a','routing')[0]!
  await vi.waitFor(() => expect(refreshes).toBe(1),{timeout:3000})
  auth.checkAuth(); await vi.waitFor(() => expect(auth.token).toBe('fixture-renewed'))
  expect(old.res.destroyed).toBe(false)
  complete(old); await vi.waitFor(() => expect(region(view).attributes('aria-busy')).toBe('false'))
  expect(region(view).text()).toContain('Model rate limited')
})

// PR362 combination: keep the routing reader on the official winning session.
const settled = (promise: Promise<unknown>) => promise.then(value => ({value,error:null}), error => ({value:null,error}))
it('keeps routing on the current session when an old login arrives while a newer intent is pending', async () => {
  hold = 'routing'; const view = await ready(true)
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
  const oldRead = pending('fixture-a','routing')[0]!
  holdAuth = true; const obsolete = settled(login('b'))
  await vi.waitFor(() => expect(authReplies).toHaveLength(1))
  const winner = settled(login('a'))
  await vi.waitFor(() => expect(authReplies).toHaveLength(2))
  reply(authReplies[0]!.res,authReplies[0]!.data)
  expect((await obsolete).error?.code).toBe('AUTH_SESSION_CHANGED')
  expect(auth.user?.id).toBe(1)
  expect(oldRead.res.destroyed).toBe(false)
  expect(region(view).attributes('aria-busy')).toBe('true')
  reply(authReplies[1]!.res,authReplies[1]!.data)
  expect((await winner).error).toBeNull()
  await vi.waitFor(() => expect(oldRead.res.destroyed).toBe(true))
  complete(oldRead,'obsolete-auth-reply'); await pause()
  expect(view.find('[data-testid="ledger-section-routing"]').exists()).toBe(false)
  expect(view.text()).not.toContain('987654')
})
function failNextAuthUserWrite() {
  const target = Object.getPrototypeOf(localStorage), original = target.setItem
  let fired = false
  vi.spyOn(target,'setItem').mockImplementation(function(this: Storage,key: string,value: string) {
    if (this === localStorage && key === 'auth_user' && !fired) { fired = true; throw new DOMException('synthetic publication failure','QuotaExceededError') }
    return original.call(this,key,value)
  })
  return () => fired
}
it('clears pending routing on an owned publication failure and permits a clean explicit retry after recovery', async () => {
  hold = 'routing'; const view = await ready(true)
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
  const oldRead = pending('fixture-a','routing')[0]!, fired = failNextAuthUserWrite()
  expect((await settled(login('b'))).error?.name).toBe('QuotaExceededError')
  expect(fired()).toBe(true); expect(auth.isAuthenticated).toBe(false)
  await vi.waitFor(() => expect(oldRead.res.destroyed).toBe(true))
  complete(oldRead,'obsolete-storage'); await pause()
  expect(view.find('[data-testid="ledger-section-routing"]').exists()).toBe(false)
  expect(view.text()).not.toContain('987654')
  hold = ''; await login('a'); await router.push({query:{private_id:id,lang:'en'}})
  await vi.waitFor(() => expect(region(view).text()).toContain('Model rate limited'))
})
it('cannot clear a new routing read when an obsolete publication error reaches Pinia late', async () => {
  const view = await ready(true), fired = failNextAuthUserWrite(), realLogin = authAPI.login
  let observed = false, release!: () => void
  const barrier = new Promise<void>(resolve => { release = resolve })
  vi.spyOn(authAPI,'login').mockImplementation(async (...args) => {
    try { return await realLogin(...args) }
    catch (error) { if (args[0].email.startsWith('b@')) { observed = true; await barrier } throw error }
  })
  const old = settled(login('b'))
  try {
    await vi.waitFor(() => expect(observed).toBe(true)); expect(fired()).toBe(true)
    await login('a'); hold = 'routing'; await router.push({query:{private_id:id,lang:'en'}})
    await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
    const fresh = pending('fixture-a','routing')[0]!
    release(); expect((await old).error?.name).toBe('QuotaExceededError')
    expect(auth.isAuthenticated).toBe(true); expect(auth.user?.id).toBe(1)
    expect(fresh.res.destroyed).toBe(false); expect(region(view).attributes('aria-busy')).toBe('true')
    complete(fresh); await vi.waitFor(() => expect(region(view).text()).toContain('Model rate limited'))
  } finally { release(); await old }
})

it('preserves the winning session routing when newer login succeeds before an older success arrives', async () => {
  const view = await ready(true)
  holdAuth = true; const obsolete = settled(login('b'))
  await vi.waitFor(() => expect(authReplies).toHaveLength(1))
  const winner = settled(login('a'))
  await vi.waitFor(() => expect(authReplies).toHaveLength(2))
  reply(authReplies[1]!.res,authReplies[1]!.data)
  expect((await winner).error).toBeNull()
  await vi.waitFor(() => expect(router.currentRoute.value.query.private_id).toBeUndefined())
  await flushPromises()
  hold = 'routing'; await router.push({query:{private_id:id,lang:'en'}})
  await vi.waitFor(() => expect(pending('fixture-a','routing')).toHaveLength(1))
  const fresh = pending('fixture-a','routing')[0]!
  reply(authReplies[0]!.res,authReplies[0]!.data)
  expect((await obsolete).error?.code).toBe('AUTH_SESSION_CHANGED')
  expect(auth.user?.id).toBe(1); expect(auth.isAuthenticated).toBe(true)
  expect(fresh.res.destroyed).toBe(false); expect(region(view).attributes('aria-busy')).toBe('true')
  complete(fresh); await vi.waitFor(() => expect(region(view).text()).toContain('Model rate limited'))
})
