import { createServer, type Server, type ServerResponse } from 'node:http'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import BlindboxView from '@/views/public/BlindboxView.vue'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import zh from '@/i18n/locales/zh'
vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const { showError, showInfo } = vi.hoisted(() => ({ showError: vi.fn(), showInfo: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showInfo }) }))
vi.mock('@/router', () => ({ default: { push: vi.fn() } }))
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const pool = { version: 'fixture', cost: .5, rtp_cap: .9, tiers: [{ amount: .2, weight: 10000 }] }
const initial = { enabled: true, coupon_pool_ready: true, coupon_weight_bp: 0, redeem_code_weight_bp: 0, balance_weight_bp: 10000,
  cost_amount: .5, pool, daily_limit: 2, opens_today: 0, can_open: true, growth_eligibility: { reward_mode: 'redeemable' } }
const user = (id = 1) => ({ id, email: 'fixture@example.test', role: 'user', status: 'active', balance: 1, concurrency: 5 })
let server: Server
let auth: ReturnType<typeof useAuthStore>
let pinia: ReturnType<typeof createPinia>
let wrapper: VueWrapper | undefined
let gets = 0; let recents = 0; let posts = 0; let publicGets = 0
let pendingPost: ServerResponse | undefined; let heldProfile: ServerResponse | undefined
let holdProfile = false
let statusData = { ...initial }
let statusResponse: (res: ServerResponse) => void
let recentResponse: (res: ServerResponse) => void
const recent = (user = 'fixture-recent-user') => [{ user, reward: .2, when: '2026-10-11T00:00:00Z' }]
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter, timeout: apiClient.defaults.timeout }
const reply = (res: ServerResponse, data: unknown, status = 200) => {
  res.writeHead(status, { 'Content-Type': 'application/json', Connection: 'close' }); res.end(JSON.stringify({ code: status === 200 ? 0 : status, data }))
}
beforeAll(async () => {
  server = createServer((req, res) => {
    const path = req.url?.split('?')[0]
    if (path === '/api/v1/play/blindbox/status') { gets++; statusResponse(res) }
    else if (path === '/api/v1/play/blindbox/pool') { publicGets++; reply(res, { enabled: true, coupon_pool_ready: true }) }
    else if (path === '/api/v1/play/blindbox/recent') { recents++; recentResponse(res) }
    else if (path === '/api/v1/play/blindbox/open') { posts++; pendingPost = res }
    else if (path === '/api/v1/auth/me') {
      if (holdProfile && req.headers.authorization === 'Bearer fixture-a') heldProfile = res
      else reply(res, user(req.headers.authorization === 'Bearer fixture-b' ? 2 : 1))
    } else if (path === '/api/v1/auth/logout') reply(res, {})
    else reply(res, {}, 404)
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing fixture')
  Object.assign(apiClient.defaults, { baseURL: `http://127.0.0.1:${address.port}/api/v1`, adapter: 'http' })
})
beforeEach(() => {
  localStorage.clear(); sessionStorage.clear(); vi.clearAllMocks()
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore()
  gets = 0; recents = 0; posts = 0; publicGets = 0; pendingPost = undefined; heldProfile = undefined; holdProfile = false
  statusData = { ...initial }; statusResponse = res => reply(res, statusData); recentResponse = res => reply(res, recent()); apiClient.defaults.timeout = 30000
})
afterEach(async () => { wrapper?.unmount(); wrapper = undefined; await auth.logout(); server.closeAllConnections() })
afterAll(async () => { Object.assign(apiClient.defaults, original); await new Promise<void>(resolve => server.close(() => resolve())); vi.unstubAllGlobals() })
async function signIn(id = 1) {
  localStorage.setItem('auth_token', id === 1 ? 'fixture-a' : 'fixture-b'); localStorage.setItem('auth_user', JSON.stringify(user(id)))
  auth.checkAuth(); await vi.waitFor(() => expect(auth.user?.id).toBe(id))
}
async function start(guest = false) {
  if (!guest) await signIn()
  wrapper = mount(BlindboxView, { global: { plugins: [pinia, i18n], stubs: {
    AuthenticatedPlayShell: { template: '<div><slot /></div>' }, PublicPageToolbar: true, PublicPlayBackLink: true,
    SupportFloatingCard: true, RouterLink: { template: '<a><slot /></a>' }, RewardCelebrationOverlay: { props: ['open'], template: '<div v-if="open" data-testid="visible-reward">reward</div>' },
  } } })
}
async function open() {
  await start(); await vi.waitFor(() => expect(wrapper!.get('button.play-btn-primary').attributes('disabled')).toBeUndefined())
  await wrapper!.get('button.play-btn-primary').trigger('click'); await vi.waitFor(() => expect(pendingPost).toBeDefined())
}
function commit() { reply(pendingPost!, { cost_amount: .5, reward_amount: .2, net_amount: -.3, reward_type: 'balance', opens_today: 1, pool_version: 'fixture', server_date: '2026-10-11' }) }
describe('blindbox production Pinia/Axios through HTTP', () => {
  it.each([403, 503, 0, 'timeout'] as const)('recovers a %s status read without showing disabled', async code => {
    statusResponse = res => { if (code === 0) res.destroy(); else if (code !== 'timeout') reply(res, {}, code) }
    if (code === 'timeout') apiClient.defaults.timeout = 100
    await start(); await vi.waitFor(() => expect(wrapper!.find('[role="alert"]').exists()).toBe(true))
    expect(wrapper!.text()).not.toContain(i18n.global.t('blindbox.disabled'))
    statusResponse = res => reply(res, statusData); apiClient.defaults.timeout = 30000
    await wrapper!.get('[data-testid="blindbox-status-retry"]').trigger('click')
    await vi.waitFor(() => expect(wrapper!.get('button.play-btn-primary').attributes('disabled')).toBeUndefined())
  })
  it('keeps anonymous and Explorer previews free of private reward details', async () => {
    await start(true); await vi.waitFor(() => expect(publicGets).toBe(1)); expect(gets).toBe(0)
    expect(wrapper!.find('.play-prize-amount').exists()).toBe(false)
    statusData = { ...initial, can_open: false, growth_eligibility: { reward_mode: 'energy' } }
    await signIn(); await vi.waitFor(() => expect(wrapper!.text()).toContain(i18n.global.t('blindbox.growthIneligible')))
    expect(wrapper!.find('[data-testid="blindbox-branches"]').exists()).toBe(false)
  })
  it('does not launch a recent-feed request from a status response after unmount', async () => {
    let pending: ServerResponse | undefined; statusResponse = res => { pending = res }
    await start(); await vi.waitFor(() => expect(pending).toBeDefined())
    wrapper!.unmount(); wrapper = undefined; reply(pending!, statusData)
    await new Promise(resolve => setTimeout(resolve, 100)); expect(recents).toBe(0)
  })
  it.each(['unmount', 'account'] as const)('retains an unobserved committed result for its owner after %s', async change => {
    await open()
    const key = sessionStorage.getItem('blindbox.pending-open:1'); expect(key).toBeTruthy()
    if (change === 'unmount') { wrapper!.unmount(); wrapper = undefined }
    else { await auth.logout(); await signIn(2) }
    commit(); await new Promise(resolve => setTimeout(resolve, 100))
    expect(sessionStorage.getItem('blindbox.pending-open:1')).toBe(key)
    expect(showError).not.toHaveBeenCalled()
  })
  it.each(['disabled', 'pool-unavailable', 'locked'] as const)('can resume an existing request when current status is %s', async state => {
    statusData = { ...initial, enabled: state !== 'disabled', coupon_pool_ready: state !== 'pool-unavailable', can_open: false,
      growth_eligibility: { reward_mode: state === 'locked' ? 'energy' : 'redeemable' } }
    sessionStorage.setItem('blindbox.pending-open:1', 'fixture-pending')
    await start()
    await vi.waitFor(() => expect(wrapper!.get('[data-testid="blindbox-resume"]').attributes('disabled')).toBeUndefined())
    const resume = wrapper!.get('[data-testid="blindbox-resume"]')
    await Promise.all([resume.trigger('click'), resume.trigger('click')])
    await vi.waitFor(() => expect(pendingPost).toBeDefined()); expect(posts).toBe(1)
    commit(); await vi.waitFor(() => expect(sessionStorage.getItem('blindbox.pending-open:1')).toBeNull())
    expect(showError).not.toHaveBeenCalled()
  })
  it.each(['account', 'unmount'] as const)('does not refresh another view after a settled open and delayed profile (%s)', async change => {
    await open(); holdProfile = true; commit(); await vi.waitFor(() => expect(heldProfile).toBeDefined())
    if (change === 'unmount') { wrapper!.unmount(); wrapper = undefined }
    else { await auth.logout(); await signIn(2); await vi.waitFor(() => expect(gets).toBe(2)) }
    await new Promise(resolve => setTimeout(resolve, 80)); const reads = gets; const recentReads = recents
    reply(heldProfile!, user())
    await new Promise(resolve => setTimeout(resolve, 100))
    expect(gets).toBe(reads); expect(recents).toBe(recentReads); expect(posts).toBe(1)
  })
})

describe('blindbox identity isolation', () => {
  it('clears an accepted reward immediately when the owner logs out', async () => {
    await open(); commit()
    await vi.waitFor(() => expect(wrapper!.find('[data-testid="visible-reward"]').exists()).toBe(true))
    await vi.waitFor(() => expect(sessionStorage.getItem('blindbox.pending-open:1')).toBeNull())
    await auth.logout()
    await vi.waitFor(() => expect(publicGets).toBe(1))
    expect(wrapper!.find('[data-testid="visible-reward"]').exists()).toBe(false)
  })
  it('clears old qualified prize data while a new account status is pending', async () => {
    await start()
    await vi.waitFor(() => expect(wrapper!.find('.play-prize-amount').exists()).toBe(true))
    let pendingStatus: ServerResponse | undefined
    statusResponse = res => { pendingStatus = res }
    await signIn(2)
    await vi.waitFor(() => expect(pendingStatus).toBeDefined())
    expect(wrapper!.find('.play-prize-amount').exists()).toBe(false)
  })
  it.each([false, true])('reloads recent wins only for a qualified new account (Explorer=%s)', async explorer => {
    await start(); await vi.waitFor(() => expect(recents).toBe(1))
    statusData = { ...initial, growth_eligibility: { reward_mode: explorer ? 'energy' : 'redeemable' } }
    await signIn(2); await vi.waitFor(() => expect(gets).toBe(2))
    await vi.waitFor(() => expect(wrapper!.text()).toContain(explorer ? i18n.global.t('blindbox.growthIneligible') : '$0.20'))
    await new Promise(resolve => setTimeout(resolve, 100))
    expect(recents).toBe(explorer ? 1 : 2)
    expect(wrapper!.text().includes('fixture-recent-user')).toBe(!explorer)
  })
})

describe('blindbox qualification failure recovery', () => {
  it.each(['mount', 'switch'] as const)('keeps recent rewards hidden until qualification retry succeeds (%s)', async phase => {
    if (phase === 'switch') { await start(); await vi.waitFor(() => expect(recents).toBe(1)) }
    const before = recents
    statusResponse = res => reply(res, {}, 503)
    if (phase === 'mount') await start(); else await signIn(2)
    await vi.waitFor(() => expect(wrapper!.find('[role="alert"]').exists()).toBe(true))
    await new Promise(resolve => setTimeout(resolve, 100))
    expect(recents).toBe(before); expect(wrapper!.text()).not.toContain('fixture-recent-user')
    statusResponse = res => reply(res, initial)
    await wrapper!.get('[data-testid="blindbox-status-retry"]').trigger('click')
    await vi.waitFor(() => expect(wrapper!.text()).toContain('fixture-recent-user'))
    expect(recents).toBe(before + 1)
  })
})

describe('blindbox same-account qualification invalidation', () => {
  it('clears an already displayed public feed after settlement qualification fails and restores it on retry', async () => {
    await open(); await vi.waitFor(() => expect(wrapper!.text()).toContain('fixture-recent-user'))
    statusResponse = res => reply(res, {}, 503); commit()
    await vi.waitFor(() => expect(wrapper!.find('[role="alert"]').exists()).toBe(true))
    expect(wrapper!.text()).not.toContain('fixture-recent-user'); expect(recents).toBe(1)
    statusResponse = res => reply(res, initial); recentResponse = res => reply(res, recent('current-recent-user'))
    await wrapper!.get('[data-testid="blindbox-status-retry"]').trigger('click')
    await vi.waitFor(() => expect(wrapper!.text()).toContain('current-recent-user'))
    expect(posts).toBe(1); expect(recents).toBe(2)
  })
  it.each(['before-retry', 'after-retry'] as const)('ignores a real delayed public feed after qualification becomes unknown (%s)', async arrival => {
    let heldRecent: ServerResponse | undefined; let oldResponseObserved = false
    const observer = apiClient.interceptors.response.use(response => {
      if (response.config.url?.endsWith('/blindbox/recent') && response.data?.[0]?.user === 'old-recent-user') oldResponseObserved = true
      return response
    })
    try {
      recentResponse = res => { heldRecent = res }
      await open(); await vi.waitFor(() => expect(heldRecent).toBeDefined())
      statusResponse = res => reply(res, {}, 503); commit()
      await vi.waitFor(() => expect(wrapper!.find('[role="alert"]').exists()).toBe(true))
      const releaseOld = async () => {
        reply(heldRecent!, recent('old-recent-user'))
        await vi.waitFor(() => expect(oldResponseObserved).toBe(true)); await flushPromises()
        expect(wrapper!.text()).not.toContain('old-recent-user')
      }
      if (arrival === 'before-retry') await releaseOld()
      statusResponse = res => reply(res, initial); recentResponse = res => reply(res, recent('current-recent-user'))
      await wrapper!.get('[data-testid="blindbox-status-retry"]').trigger('click')
      await vi.waitFor(() => expect(wrapper!.text()).toContain('current-recent-user'))
      if (arrival === 'after-retry') await releaseOld()
      expect(wrapper!.text()).toContain('current-recent-user'); expect(posts).toBe(1); expect(recents).toBe(2)
    } finally { apiClient.interceptors.response.eject(observer) }
  })
})
