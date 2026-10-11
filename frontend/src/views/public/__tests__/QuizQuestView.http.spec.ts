// Real HTTP and the production Axios interceptors; no mocked API error objects.
import { createServer, type Server, type ServerResponse } from 'node:http'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import QuizQuestView from '@/views/public/QuizQuestView.vue'
import apiClient from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import zh from '@/i18n/locales/zh'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({ isAuthenticated: true, user: { id: 1 }, token: 'fixture-a', refreshUser: vi.fn() })
  return { useAuthStore: () => state }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn() }) }))
enableAutoUnmount(afterEach)
const auth = useAuthStore() as unknown as { isAuthenticated: boolean; user: { id: number }; token: string }
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const initial = {
  enabled: true, coupon_pool_ready: true, already_submitted: false, server_date: '2026-10-10', reward_per_correct: 0.1,
  questions: [{ id: 1, prompt: '隔离测试题', options: ['选项一', '选项二'] }],
}
let quiz: Record<string, unknown> = { ...initial }
let server: Server
let getCount = 0
let postCount = 0
let respond: (res: ServerResponse) => void
let submitResponse: (res: ServerResponse) => void
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter, timeout: apiClient.defaults.timeout }
const json = (res: ServerResponse, code: number, data: unknown) => {
  res.writeHead(code, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify(data))
}
const success = (res: ServerResponse) => json(res, 200, { code: 0, data: quiz })
const mountView = () => mount(QuizQuestView, { global: { plugins: [i18n], stubs: {
  AuthenticatedPlayShell: { template: '<div><slot /></div>' }, PublicPageToolbar: true,
  PublicPlayBackLink: true, SupportFloatingCard: true, CouponRewardCard: true,
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
} } })
beforeAll(async () => {
  server = createServer((req, res) => {
    if (req.url?.startsWith('/api/v1/play/quiz/today')) { getCount++; respond(res) }
    else if (req.method === 'POST') { postCount++; submitResponse(res) }
    else json(res, 404, { code: 404 })
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing fixture server')
  apiClient.defaults.baseURL = `http://127.0.0.1:${address.port}/api/v1`
  apiClient.defaults.adapter = 'http'
})
beforeEach(() => {
  auth.isAuthenticated = true; auth.user = { id: 1 }; auth.token = 'fixture-a'
  submitResponse = res => json(res, 500, { code: 500 })
  quiz = { ...initial }; getCount = 0; postCount = 0; respond = success; apiClient.defaults.timeout = 30000
})
afterEach(() => server.closeAllConnections())
afterAll(async () => {
  Object.assign(apiClient.defaults, original)
  await new Promise<void>(resolve => server.close(() => resolve()))
  vi.unstubAllGlobals()
})
describe('quiz read contract through real Axios', () => {
  it('rejects a same-tick manual refresh while submission settles', async () => {
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('input').exists()).toBe(true))
    await wrapper.get('input').setValue()
    let pendingPost: ServerResponse | undefined
    submitResponse = res => { pendingPost = res }
    respond = () => { /* A pre-commit read must not be issued. */ }
    await Promise.all([
      wrapper.get('button.play-btn-primary').trigger('click'),
      wrapper.get('[data-testid="refresh-quiz"]').trigger('click'),
    ])
    await vi.waitFor(() => expect(pendingPost).toBeDefined())
    await new Promise(resolve => setTimeout(resolve, 80))
    expect(getCount).toBe(1)
    quiz = { ...initial, questions: [], already_submitted: true, previous_score: 1, previous_total: 1, previous_reward: .1, previous_reward_type: 'balance' }
    respond = success
    json(pendingPost!, 200, { code: 0, data: { score: 1, total: 1, reward_amount: .1, reward_type: 'balance' } })
    await vi.waitFor(() => expect(wrapper.find('[aria-busy="true"]').exists()).toBe(false))
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.text()).toContain(i18n.global.t('quiz.done', { score: 1, total: 1, reward: '0.10' }))
    expect(postCount).toBe(1); expect(getCount).toBe(2)
  })
  it.each([503, 403, 0, 'timeout'] as const)('distinguishes %s from disabled and recovers', async code => {
    respond = res => {
      if (code === 0) res.destroy()
      else if (code !== 'timeout') json(res, code, { code, reason: 'FIXTURE_READ_FAILURE' })
    }
    if (code === 'timeout') apiClient.defaults.timeout = 100
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(true))
    expect(wrapper.get('[role="alert"]').text()).toMatch(/[\u4e00-\u9fff]/)
    expect(wrapper.text()).not.toContain(i18n.global.t('quiz.disabled'))
    expect(wrapper.find('input').exists()).toBe(false)
    respond = success; apiClient.defaults.timeout = 30000
    await wrapper.get('[role="alert"] button').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('隔离测试题'))
    expect(getCount).toBe(2)
  })
  it('keeps guest entry available without calling the authenticated endpoint', async () => {
    auth.isAuthenticated = false; auth.token = ''
    const wrapper = mountView()
    await new Promise(resolve => setTimeout(resolve, 100))
    expect(getCount).toBe(0)
    expect(wrapper.find('a[href="/login?redirect=/quiz-quest"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain(i18n.global.t('quiz.disabled'))
    auth.isAuthenticated = true; auth.token = 'fixture-b'
    await vi.waitFor(() => expect(wrapper.text()).toContain('隔离测试题'))
    expect(getCount).toBe(1)
  })
  it('coalesces repeated refresh, hides stale actions and prevents submit during read', async () => {
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('input').exists()).toBe(true))
    await wrapper.get('input').setValue()
    let pending: ServerResponse | undefined
    respond = res => { pending = res }
    const refresh = wrapper.get('button[data-testid="refresh-quiz"]')
    await Promise.all([refresh.trigger('click'), refresh.trigger('click')])
    await vi.waitFor(() => expect(pending).toBeDefined())
    expect(getCount).toBe(2)
    expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true)
    expect(wrapper.find('input').exists()).toBe(false)
    expect(postCount).toBe(0)
    quiz = { ...initial, enabled: false }; success(pending!)
    await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t('quiz.disabled')))
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
  it('shows an empty question pool explicitly and refreshes when questions become available', async () => {
    quiz = { ...initial, questions: [] }
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t('quiz.empty')))
    expect(wrapper.find('input').exists()).toBe(false)
    quiz = { ...initial }
    await wrapper.get('button[data-testid="refresh-quiz"]').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('隔离测试题'))
  })
  it('replaces a transient result with the authoritative result after the server date changes', async () => {
    const coupon = { user_coupon_id: 8, template_id: 3, name: '昨日隔离券', benefit_type: 'fixed_amount', benefit_value: 1, currency: 'USD', applicable_scopes: ['balance'], minimum_order_amount: 0, valid_from: '2026-10-10T00:00:00Z', expires_at: '2026-10-13T00:00:00Z' }
    submitResponse = res => {
      quiz = { ...initial, questions: [], already_submitted: true, previous_score: 1, previous_total: 1, previous_reward: 0, previous_reward_type: 'coupon', previous_coupon: coupon }
      json(res, 200, { code: 0, data: { score: 1, total: 1, reward_amount: 0, reward_type: 'coupon', coupon } })
    }
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('input').exists()).toBe(true))
    await wrapper.get('input').setValue()
    await wrapper.get('button.play-btn-primary').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t('quiz.couponDone', { score: 1, total: 1 })))
    quiz = { ...initial, server_date: '2026-10-11', questions: [], already_submitted: true, previous_score: 0, previous_total: 1, previous_reward: 0, previous_reward_type: 'none' }
    await wrapper.get('button[data-testid="refresh-quiz"]').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t('quiz.done', { score: 0, total: 1, reward: '0.00' })))
    expect(wrapper.text()).not.toContain(i18n.global.t('quiz.couponDone', { score: 1, total: 1 }))
  })

  it('preserves selected answers when the same account profile is refreshed', async () => {
    const wrapper = mountView()
    await vi.waitFor(() => expect(wrapper.find('input').exists()).toBe(true))
    await wrapper.get('input').setValue()
    auth.user = { id: 1 }
    await new Promise(resolve => setTimeout(resolve, 100))
    expect(getCount).toBe(1)
    expect((wrapper.get('input').element as HTMLInputElement).checked).toBe(true)
  })

})
