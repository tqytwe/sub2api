// Real HTTP responses pass through playAPI and the production Axios interceptors.
import { createServer, type Server, type ServerResponse } from 'node:http'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import CheckInView from '@/views/user/CheckInView.vue'
import PlayHubView from '@/views/user/PlayHubView.vue'
import apiClient from '@/api/client'
import zh from '@/i18n/locales/zh'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1, balance: 1 }, token: 'fixture-only', refreshUser: vi.fn() }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn() }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/utils/growthAnalytics', () => ({ trackQuestCompleteOnce: vi.fn(), trackGrowthEvent: vi.fn() }))
vi.mock('@/utils/featureFlags', async importOriginal => {
  const actual = await importOriginal<typeof import('@/utils/featureFlags')>()
  return { ...actual, isFeatureFlagEnabled: (flag: unknown) => flag === actual.FeatureFlags.playCheckin }
})
enableAutoUnmount(afterEach)

const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const initialStatus = {
  enabled: true, checked_in_today: false, server_date: '2026-10-10', reward_amount: 0.5,
  redeemable_reward_eligible: true, can_makeup: true, makeup_date: '2026-10-09',
  streak_count: 6, next_milestone_days: 7, next_milestone_bonus: 1,
}
let status = { ...initialStatus }
let server: Server
let getCount = 0
let respond: (res: ServerResponse, hub: boolean) => void
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter, timeout: apiClient.defaults.timeout }
const json = (res: ServerResponse, code: number, data: unknown) => {
  res.writeHead(code, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify(data))
}
const success = (res: ServerResponse, hub: boolean) => json(res, 200, { code: 0, data: hub ? {
  any_enabled: status.enabled, pending_actions: status.enabled && !status.checked_in_today ? 1 : 0,
  growth: { balance: 1 }, checkin: status.enabled ? status : null,
} : status })
const mountView = (hub = false) => mount(hub ? PlayHubView : CheckInView, {
  global: { plugins: [i18n], stubs: {
    AppLayout: { template: '<div><slot /></div>' }, Icon: true,
    RouterLink: { template: '<a><slot /></a>' },
  } },
})

beforeAll(async () => {
  server = createServer((req, res) => {
    if (req.url?.startsWith('/api/v1/auth/me')) json(res, 200, { code: 0, data: { balance: 1 } })
    else if (req.method === 'GET') {
      getCount++
      respond(res, !!req.url?.startsWith('/api/v1/play/hub'))
    } else json(res, 405, { code: 405 })
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing test server address')
  apiClient.defaults.baseURL = `http://127.0.0.1:${address.port}/api/v1`
  apiClient.defaults.adapter = 'http'
})
beforeEach(() => {
  status = { ...initialStatus }
  getCount = 0
  respond = success
  apiClient.defaults.timeout = 30000
})
afterEach(() => server.closeAllConnections())
afterAll(async () => {
  Object.assign(apiClient.defaults, original)
  await new Promise<void>(resolve => server.close(() => resolve()))
  vi.unstubAllGlobals()
})

for (const hub of [false, true]) {
  const scope = hub ? 'playHub' : 'checkin'
  describe(`${scope} status through real Axios`, () => {
    it.each([503, 403, 0, 'timeout'] as const)('distinguishes %s from disabled and recovers via retry', async code => {
      respond = res => {
        if (code === 0) res.destroy()
        else if (code !== 'timeout') json(res, code, { code, reason: 'FIXTURE_FAILURE' })
      }
      if (code === 'timeout') apiClient.defaults.timeout = 100
      const wrapper = mountView(hub)
      const key = code === 403 ? 'loadForbidden' : code === 0 || code === 'timeout' ? 'loadNetworkFailed' : 'loadFailed'
      await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(true))
      const message = i18n.global.t(`${scope}.${key}`)
      expect(message).toMatch(/[\u4e00-\u9fff]/)
      expect(wrapper.get('[role="alert"]').text()).toContain(message)
      expect(wrapper.text()).not.toContain(i18n.global.t(hub ? 'playHub.empty' : 'checkin.disabled'))
      expect(wrapper.find('button.w-full').exists()).toBe(false)
      respond = success
      apiClient.defaults.timeout = 30000
      await wrapper.get('[role="alert"] button').trigger('click')
      await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(false))
      await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t(hub ? 'nav.checkIn' : 'checkin.button')))
      expect(getCount).toBe(2)
    })

    it('allows one retry in flight, shows loading and hides stale actions', async () => {
      respond = res => json(res, 503, { code: 503 })
      const wrapper = mountView(hub)
      await vi.waitFor(() => expect(wrapper.find('[role="alert"] button').exists()).toBe(true))
      let pending: ServerResponse | undefined
      respond = res => { pending = res }
      const retry = wrapper.get('[role="alert"] button')
      await Promise.all([retry.trigger('click'), retry.trigger('click')])
      await vi.waitFor(() => expect(pending).toBeDefined())
      expect(getCount).toBe(2)
      expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true)
      expect(wrapper.get('button[data-testid="refresh-status"]').attributes('disabled')).toBeDefined()
      expect(wrapper.find('button.w-full').exists()).toBe(false)
      success(pending!, hub)
      await vi.waitFor(() => expect(wrapper.find('[aria-busy="true"]').exists()).toBe(false))
      expect(getCount).toBe(2)
    })

    it('refreshes a closed feature, reopens it and discards stale displayed actions during refresh', async () => {
      status.enabled = false
      const wrapper = mountView(hub)
      await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t(hub ? 'playHub.empty' : 'checkin.disabled')))
      expect(wrapper.find('[role="alert"]').exists()).toBe(false)
      status.enabled = true
      await wrapper.get('button[data-testid="refresh-status"]').trigger('click')
      await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t(hub ? 'playHub.actionCheckin' : 'checkin.button')))
      let pending: ServerResponse | undefined
      respond = res => { pending = res }
      await wrapper.get('button[data-testid="refresh-status"]').trigger('click')
      await vi.waitFor(() => expect(pending).toBeDefined())
      expect(wrapper.text()).not.toContain(i18n.global.t(hub ? 'playHub.actionCheckin' : 'checkin.makeupButton'))
      status.enabled = false
      success(pending!, hub)
      await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t(hub ? 'playHub.empty' : 'checkin.disabled')))
      expect(getCount).toBe(3)
    })

    it('does not let an old response overwrite a reopened page', async () => {
      let pending: ServerResponse | undefined
      respond = res => { pending = res }
      const old = mountView(hub)
      await vi.waitFor(() => expect(pending).toBeDefined())
      old.unmount()
      respond = success
      const current = mountView(hub)
      await vi.waitFor(() => expect(current.text()).toContain(i18n.global.t(hub ? 'playHub.actionCheckin' : 'checkin.button')))
      status.enabled = false
      success(pending!, hub)
      await new Promise(resolve => setTimeout(resolve, 50))
      expect(current.text()).not.toContain(i18n.global.t(hub ? 'playHub.empty' : 'checkin.disabled'))
      expect(getCount).toBe(2)
    })
  })
}

it('renders remaining days from the absolute next milestone day without changing reward amounts', async () => {
  const wrapper = mountView()
  await vi.waitFor(() => expect(wrapper.text()).toContain('再签 1 天可额外获得 $1.00'))
  status.streak_count = 7
  status.next_milestone_days = 14
  status.next_milestone_bonus = 2
  await wrapper.get('button[data-testid="refresh-status"]').trigger('click')
  await vi.waitFor(() => expect(wrapper.text()).toContain('再签 7 天可额外获得 $2.00'))
  status.next_milestone_days = 0
  await wrapper.get('button[data-testid="refresh-status"]').trigger('click')
  await vi.waitFor(() => expect(wrapper.text()).not.toContain('可额外获得'))
})
