// Fault injection at the HTTP boundary: playAPI and the real Axios transport /
// interceptors stay intact. PostgreSQL-backed cases live in CheckInView.http.
import { createServer, type Server, type ServerResponse } from 'node:http'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import CheckInView from '@/views/user/CheckInView.vue'
import apiClient from '@/api/client'
import zh from '@/i18n/locales/zh'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const { showSuccess, showError, showInfo, refreshUser } = vi.hoisted(() => ({
  showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn(), refreshUser: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { balance: 1 }, refreshUser }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError, showInfo }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/utils/growthAnalytics', () => ({ trackQuestCompleteOnce: vi.fn() }))
enableAutoUnmount(afterEach)

const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const initialStatus = {
  enabled: true, checked_in_today: false, server_date: '2026-10-10',
  reward_amount: 0.5, redeemable_reward_eligible: true,
  can_makeup: true, makeup_date: '2026-10-09',
  growth_eligibility: { primary_reason: 'email_unverified' },
}
let status = { ...initialStatus }
let server: Server
let postCount = 0
let getCount = 0
let respond: (res: ServerResponse) => void
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter, timeout: apiClient.defaults.timeout }
const json = (res: ServerResponse, code: number, data: unknown) => {
  res.writeHead(code, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify(data))
}
const mountView = () => mount(CheckInView, {
  global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' } } },
})

beforeAll(async () => {
  server = createServer((req, res) => {
    if (req.method === 'GET' && req.url?.startsWith('/api/v1/play/checkin/status')) {
      getCount++
      json(res, 200, { code: 0, data: status })
    } else if (req.method === 'GET' && req.url?.startsWith('/api/v1/auth/me')) {
      json(res, 200, { code: 0, data: { balance: 1 } })
    } else if (req.method === 'POST') {
      postCount++
      respond(res)
    } else json(res, 404, { code: 404 })
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing test server address')
  apiClient.defaults.baseURL = `http://127.0.0.1:${address.port}/api/v1`
  apiClient.defaults.adapter = 'http'
})
beforeEach(() => {
  vi.clearAllMocks()
  status = { ...initialStatus }
  postCount = 0
  getCount = 0
  apiClient.defaults.timeout = 30000
  refreshUser.mockResolvedValue(undefined)
})
afterAll(async () => {
  Object.assign(apiClient.defaults, original)
  server.closeAllConnections()
  await new Promise<void>(resolve => server.close(() => resolve()))
  vi.unstubAllGlobals()
})

describe('CheckInView errors through the real Axios HTTP client', () => {
  it.each([
    ['PLAY_CHECKIN_ALREADY_DONE', 409, 'alreadyDone', 'info', true],
    ['PLAY_FEATURE_DISABLED', 400, 'disabled', 'error', false],
    ['PLAY_GROWTH_REWARD_INELIGIBLE', 403, 'growthReasons.email_unverified', 'info', false],
    ['PLAY_GROWTH_GOVERNANCE_NOT_APPROVED', 403, 'governanceNotApproved', 'info', false],
    ['PLAY_GROWTH_GOVERNANCE_ROLLOUT_EXCLUDED', 403, 'governanceRolloutExcluded', 'info', false],
    ['PLAY_GROWTH_GOVERNANCE_BUDGET_EXHAUSTED', 403, 'governanceBudgetExhausted', 'info', false],
    ['PLAY_GROWTH_GOVERNANCE_UNAVAILABLE', 503, 'governanceUnavailable', 'error', false],
    ['COUPON_REWARD_POOL_UNAVAILABLE', 503, 'poolUnavailable', 'info', true],
    ['PLAY_GROWTH_QUALIFICATION_UNAVAILABLE', 503, 'qualificationUnavailable', 'error', false],
    ['UNRECOGNIZED_REASON', 500, 'failed', 'error', false],
  ] as const)('localizes %s with numeric HTTP code %s', async (reason, code, key, kind, reload) => {
    respond = res => {
      if (reason === 'PLAY_CHECKIN_ALREADY_DONE') status.checked_in_today = true
      json(res, code, { code, reason, message: 'Backend message must not replace Chinese copy' })
    }
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('button.w-full').exists()).toBe(true))
    refreshUser.mockClear()
    await wrapper.get('button.w-full').trigger('click')
    const toast = kind === 'info' ? showInfo : showError
    await vi.waitFor(() => expect(toast).toHaveBeenCalledWith(i18n.global.t(`checkin.${key}`)))
    await vi.waitFor(() => expect(getCount).toBe(reload ? 2 : 1))
    expect(postCount).toBe(1)
    expect(showSuccess).not.toHaveBeenCalled()
    expect(refreshUser).not.toHaveBeenCalled()
    expect(kind === 'info' ? showError : showInfo).not.toHaveBeenCalled()
  })

  it.each(['PLAY_CHECKIN_MAKEUP_UNAVAILABLE', 'PLAY_CHECKIN_MAKEUP_ALREADY_DONE'])('refreshes stale makeup state on %s', async reason => {
    respond = res => {
      status.can_makeup = false
      const code = reason.endsWith('ALREADY_DONE') ? 409 : 400
      json(res, code, { code, reason })
    }
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.findAll('button.gw-btn-primary')).toHaveLength(2))
    await wrapper.findAll('button.gw-btn-primary')[1].trigger('click')
    await vi.waitFor(() => expect(showError).toHaveBeenCalledWith(i18n.global.t('checkin.makeupUnavailable')))
    await vi.waitFor(() => expect(wrapper.findAll('button.gw-btn-primary')).toHaveLength(1))
    expect(postCount).toBe(1)
    expect(getCount).toBe(2)
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it.each(['checkin', 'makeup'])('recovers from an actual %s HTTP timeout and suppresses rapid duplicate clicks', async action => {
    let pending: ServerResponse | undefined
    respond = res => { pending = res }
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.findAll('button.gw-btn-primary')).toHaveLength(2))
    apiClient.defaults.timeout = 150
    const button = wrapper.findAll('button.gw-btn-primary')[action === 'checkin' ? 0 : 1]
    // Dispatch in the same tick as well as after disabled has rendered.
    const first = button.trigger('click')
    const second = button.trigger('click')
    // The refresh control's DOM disabled attribute has not rendered yet.
    const refresh = wrapper.get('button[data-testid="refresh-status"]').trigger('click')
    await Promise.all([first, second, refresh])
    expect(button.attributes('disabled')).toBeDefined()
    await vi.waitFor(() => expect(showError).toHaveBeenCalledWith(i18n.global.t(`checkin.${action === 'checkin' ? 'failed' : 'makeupFailed'}`)), { timeout: 3000 })
    expect(postCount).toBe(1)
    expect(getCount).toBe(1)
    expect(button.attributes('disabled')).toBeUndefined()
    expect(showSuccess).not.toHaveBeenCalled()
    pending?.destroy()
  })

  it('keeps a committed reward successful when the shared account refresh fails', async () => {
    respond = res => {
      status.checked_in_today = true
      json(res, 200, { code: 0, data: { balance_added: 0.5, reward_type: 'balance' } })
    }
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('button.w-full').exists()).toBe(true))
    refreshUser.mockRejectedValueOnce(new Error('Account refresh unavailable'))
    await wrapper.get('button.w-full').trigger('click')
    await vi.waitFor(() => expect(showSuccess).toHaveBeenCalledOnce())
    await vi.waitFor(() => expect(wrapper.get('button.w-full').text()).toBe(i18n.global.t('checkin.doneButton')))
    expect(showError).not.toHaveBeenCalled()
    expect(wrapper.get('.gw-balance-value').text()).toBe('$1.00')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('button.w-full').attributes('disabled')).toBeDefined()
    expect(postCount).toBe(1)
    expect(getCount).toBe(2)
  })
})
