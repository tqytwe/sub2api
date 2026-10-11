// Regression for late /auth/me responses: real Pinia store + Axios + HTTP.
import { createServer, type Server, type ServerResponse } from 'node:http'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import CheckInView from '@/views/user/CheckInView.vue'
import zh from '@/i18n/locales/zh'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const { showSuccess } = vi.hoisted(() => ({ showSuccess: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError: vi.fn(), showInfo: vi.fn() }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/utils/growthAnalytics', () => ({ trackQuestCompleteOnce: vi.fn() }))
enableAutoUnmount(afterEach)
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
setActivePinia(createPinia())
const auth = useAuthStore()
let server: Server
let balance = 1
let id = 1
let profileResponse: (res: ServerResponse) => void
let statusResponse: (res: ServerResponse) => void
let postResponse: (res: ServerResponse) => void
let statusGets = 0
let posts = 0
let checkedIn = false
let canMakeup = true
const statusData = () => ({ enabled: true, checked_in_today: checkedIn, can_makeup: canMakeup,
  redeemable_reward_eligible: true, makeup_date: '2026-10-09', server_date: '2026-10-10' })
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const json = (res: ServerResponse, code: number, data: unknown) => {
  res.writeHead(code, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify({ code: code === 200 ? 0 : code, data }))
}
const success = (res: ServerResponse) => json(res, 200, { id, balance, email: 'fixture@example.test', role: 'user' })
const mountView = () => mount(CheckInView, {
  global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' } } },
})
beforeAll(async () => {
  server = createServer((req, res) => {
    if (req.method === 'POST') { posts++; postResponse(res) }
    else if (req.url?.startsWith('/api/v1/auth/me')) profileResponse(res)
    else { statusGets++; statusResponse(res) }
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing test server address')
  apiClient.defaults.baseURL = `http://127.0.0.1:${address.port}/api/v1`
  apiClient.defaults.adapter = 'http'
})
beforeEach(async () => {
  balance = 1
  id = 1
  profileResponse = success
  statusResponse = res => json(res, 200, statusData())
  postResponse = res => json(res, 405, {})
  checkedIn = false
  canMakeup = true
  statusGets = 0
  posts = 0
  showSuccess.mockClear()
  await auth.setToken('fixture-only')
})
afterAll(async () => {
  auth.$dispose()
  Object.assign(apiClient.defaults, original)
  server.closeAllConnections()
  await new Promise<void>(resolve => server.close(() => resolve()))
  vi.unstubAllGlobals()
})

it('does not write a late unmounted profile response into the new page or shared account', async () => {
  let pending: ServerResponse | undefined
  profileResponse = res => { pending = res }
  const old = mountView()
  await vi.waitFor(() => expect(pending).toBeDefined())
  old.unmount()
  balance = 2.5
  profileResponse = success
  await auth.refreshUser()
  const current = mountView()
  await vi.waitFor(() => expect(current.get('.gw-balance-value').text()).toBe('$2.50'))
  json(pending!, 200, { id: 1, balance: 1 })
  await new Promise(resolve => setTimeout(resolve, 50))
  expect(current.get('.gw-balance-value').text()).toBe('$2.50')
  expect(auth.user?.balance).toBe(2.5)
})

it('discards a profile from the previous account and reads the new account', async () => {
  let pending: ServerResponse | undefined
  profileResponse = res => { pending = res }
  const wrapper = mountView()
  await vi.waitFor(() => expect(pending).toBeDefined())
  id = 2
  balance = 3
  profileResponse = success
  await auth.setToken('second-fixture-only')
  json(pending!, 200, { id: 1, balance: 1 })
  await vi.waitFor(() => expect(wrapper.get('.gw-balance-value').text()).toBe('$3.00'))
  expect(auth.user?.id).toBe(2)
  expect(auth.user?.balance).toBe(3)
})

it('does not expose cached balance when the profile read fails and retries the read', async () => {
  profileResponse = res => json(res, 503, {})
  const wrapper = mountView()
  await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(true))
  expect(wrapper.get('.gw-balance-value').text()).toBe('—')
  expect(wrapper.find('button.w-full').exists()).toBe(false)
  profileResponse = success
  await wrapper.get('[role="alert"] button').trigger('click')
  await vi.waitFor(() => expect(wrapper.get('.gw-balance-value').text()).toBe('$1.00'))
})

it.each(['checkin', 'makeup'])('refreshes after %s even when token renewal started an older status read', async action => {
  let pendingPost: ServerResponse | undefined
  let pendingStatus: ServerResponse | undefined
  postResponse = res => { pendingPost = res }
  const wrapper = mountView()
  await vi.waitFor(() => expect(wrapper.findAll('button.gw-btn-primary')).toHaveLength(2))
  await wrapper.findAll('button.gw-btn-primary')[action === 'checkin' ? 0 : 1].trigger('click')
  await vi.waitFor(() => expect(pendingPost).toBeDefined())
  const oldStatus = statusData()
  statusResponse = res => { pendingStatus = res }
  auth.token = 'renewed-fixture-only'
  await vi.waitFor(() => expect(pendingStatus).toBeDefined())
  checkedIn = action === 'checkin'
  canMakeup = false
  json(pendingPost!, 200, { balance_added: 0.5, reward_type: 'balance' })
  await vi.waitFor(() => expect(showSuccess).toHaveBeenCalledOnce())
  // Let the POST completion request a fresh read while the older GET is held.
  await new Promise(resolve => setTimeout(resolve, 50))
  statusResponse = res => json(res, 200, statusData())
  json(pendingStatus!, 200, oldStatus)
  await vi.waitFor(() => expect(statusGets).toBe(3))
  await vi.waitFor(() => expect(wrapper.findAll('button.gw-btn-primary')).toHaveLength(1))
  if (action === 'checkin') expect(wrapper.get('button.w-full').attributes('disabled')).toBeDefined()
  expect(posts).toBe(1)
})
