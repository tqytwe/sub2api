// Actual Pinia, proactive refresh timer and HTTP/Axios; only layout/toasts are stubbed.
import { createServer, type Server, type ServerResponse } from 'node:http'
import axios from 'axios'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import QuizQuestView from '@/views/public/QuizQuestView.vue'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import zh from '@/i18n/locales/zh'

vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const { showSuccess, showError, showInfo } = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError, showInfo }) }))
vi.mock('@/router', () => ({ default: { push: vi.fn() } }))
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const user = (id = 1, balance = 1) => ({ id, email: 'fixture@example.test', role: 'user', status: 'active', balance, concurrency: 5 })
const initial = { enabled: true, coupon_pool_ready: true, already_submitted: false, server_date: '2026-10-11', reward_per_correct: .1,
  questions: [{ id: 1, prompt: '隔离续期测试题', options: ['A', 'B'] }] }
let server: Server
let auth: ReturnType<typeof useAuthStore>
let pinia: ReturnType<typeof createPinia>
let wrapper: VueWrapper | undefined
let pending: ServerResponse | undefined
let pendingRead: ServerResponse | undefined
let holdRead = false
let submitted = false
let profileFails = false
let gets = 0
let posts = 0
let renewals = 0
let profiles = 0
const original = { baseURL: apiClient.defaults.baseURL, adapter: apiClient.defaults.adapter }
const rawOriginal = { baseURL: axios.defaults.baseURL, adapter: axios.defaults.adapter }
const reply = (res: ServerResponse, data: unknown, status = 200) => {
  res.writeHead(status, { 'Content-Type': 'application/json', Connection: 'close' })
  res.end(JSON.stringify({ code: status === 200 ? 0 : status, data }))
}
beforeAll(async () => {
  server = createServer((req, res) => {
    if (req.url?.startsWith('/api/v1/play/quiz/today')) {
      gets++
      if (holdRead && submitted) { pendingRead = res; return }
      reply(res, submitted ? { ...initial, questions: [], already_submitted: true, previous_score: 1, previous_total: 1, previous_reward: .1, previous_reward_type: 'balance' } : initial)
    } else if (req.url === '/api/v1/play/quiz/submit') { posts++; pending = res }
    else if (req.url === '/api/v1/auth/refresh') {
      renewals++; reply(res, { access_token: 'fixture-renewed', refresh_token: 'fixture-refresh-renewed', expires_in: 3600, token_type: 'Bearer' })
    } else if (req.url?.split('?')[0] === '/api/v1/auth/me') {
      profiles++; reply(res, user(req.headers.authorization === 'Bearer fixture-other' ? 2 : 1, submitted ? 1.1 : 1), profileFails ? 503 : 200)
    } else if (req.url === '/api/v1/auth/logout') reply(res, {})
    else reply(res, {}, 404)
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('Missing fixture server')
  const origin = `http://127.0.0.1:${address.port}`
  Object.assign(apiClient.defaults, { baseURL: origin + '/api/v1', adapter: 'http' })
  Object.assign(axios.defaults, { baseURL: origin, adapter: 'http' })
})
beforeEach(() => {
  localStorage.clear(); vi.clearAllMocks()
  pending = undefined; pendingRead = undefined; holdRead = false; submitted = false; profileFails = false; gets = 0; posts = 0; renewals = 0; profiles = 0
  pinia = createPinia(); setActivePinia(pinia); auth = useAuthStore()
})
afterEach(async () => { wrapper?.unmount(); wrapper = undefined; await auth.logout(); auth.$dispose(); server.closeAllConnections() })
afterAll(async () => {
  Object.assign(apiClient.defaults, original); Object.assign(axios.defaults, rawOriginal)
  await new Promise<void>(resolve => server.close(() => resolve())); vi.unstubAllGlobals()
})
async function start(renew = false, submit = true) {
  localStorage.setItem('auth_token', 'fixture-initial'); localStorage.setItem('auth_user', JSON.stringify(user()))
  if (renew) {
    localStorage.setItem('refresh_token', 'fixture-refresh')
    localStorage.setItem('token_expires_at', String(Date.now() + 120800))
  }
  auth.checkAuth()
  wrapper = mount(QuizQuestView, { global: { plugins: [pinia, i18n], stubs: {
    AuthenticatedPlayShell: { template: '<div><slot /></div>' }, PublicPageToolbar: true,
    PublicPlayBackLink: true, SupportFloatingCard: true, RouterLink: { template: '<a><slot /></a>' },
  } } })
  await vi.waitFor(() => expect(wrapper!.find('input').exists()).toBe(true))
  await wrapper.get('input').setValue()
  if (!submit) return
  await wrapper.get('button.play-btn-primary').trigger('click')
  await vi.waitFor(() => expect(pending).toBeDefined())
}
function commit() {
  submitted = true
  reply(pending!, { score: 1, total: 1, reward_amount: .1, reward_type: 'balance' })
}
describe('quiz session ownership through real Pinia and HTTP', () => {
  it('preserves selected answers during native renewal before submission', async () => {
    await start(true, false)
    expect((wrapper!.get('input').element as HTMLInputElement).checked).toBe(true)
    await vi.waitFor(() => expect(auth.token).toBe('fixture-renewed'), { timeout: 2500 })
    await new Promise(resolve => setTimeout(resolve, 80))
    expect((wrapper!.get('input').element as HTMLInputElement).checked).toBe(true)
    expect(gets).toBe(1); expect(posts).toBe(0); expect(renewals).toBe(1)
  })
  it('clears selected answers when entering a different account before submission', async () => {
    await start(false, false)
    await auth.setToken('fixture-other')
    await vi.waitFor(() => expect(auth.user?.id).toBe(2))
    await vi.waitFor(() => expect(wrapper!.findAll('input:checked')).toHaveLength(0))
    expect(posts).toBe(0)
  })
  it.each(['submit', 'result-read'] as const)('reconciles pending %s after native automatic token renewal', async stage => {
    await start(true)
    if (stage === 'result-read') { holdRead = true; commit(); await vi.waitFor(() => expect(pendingRead).toBeDefined()) }
    await vi.waitFor(() => expect(localStorage.getItem('auth_token')).toBe('fixture-renewed'), { timeout: 2500 })
    await vi.waitFor(() => expect(auth.token).toBe('fixture-renewed'))
    await new Promise(resolve => setTimeout(resolve, 80))
    expect(renewals).toBe(1)
    if (stage === 'submit') commit()
    else { holdRead = false; reply(pendingRead!, { ...initial, questions: [], already_submitted: true, previous_score: 1, previous_total: 1, previous_reward: .1, previous_reward_type: 'balance' }) }
    await vi.waitFor(() => expect(wrapper!.find('[aria-busy="true"]').exists()).toBe(false))
    expect(wrapper!.find('input').exists()).toBe(false)
    expect(wrapper!.text()).toContain(i18n.global.t('quiz.done', { score: 1, total: 1, reward: '0.10' }))
    await vi.waitFor(() => expect(auth.user?.balance).toBe(1.1))
    expect(posts).toBe(1); expect(gets).toBeGreaterThan(1); expect(showSuccess).toHaveBeenCalledOnce()
  })
  it.each(['account', 'logout', 'unmount', 'aba'] as const)('does not apply an old submit after %s', async change => {
    await start()
    if (change === 'unmount') { wrapper!.unmount(); wrapper = undefined }
    else {
      await auth.logout()
      if (change !== 'logout') {
        localStorage.setItem('auth_token', 'fixture-other'); localStorage.setItem('auth_user', JSON.stringify(user(2))); auth.checkAuth()
        await vi.waitFor(() => expect(auth.user?.id).toBe(2))
        await vi.waitFor(() => {
          expect(gets).toBe(2)
          expect(wrapper!.get('.play-page').attributes('aria-busy')).toBe('false')
        })
        if (change === 'aba') {
          await auth.logout(); localStorage.setItem('auth_token', 'fixture-initial'); localStorage.setItem('auth_user', JSON.stringify(user())); auth.checkAuth()
          await vi.waitFor(() => expect(auth.user?.id).toBe(1))
          await vi.waitFor(() => {
            expect(gets).toBe(3)
            expect(wrapper!.get('.play-page').attributes('aria-busy')).toBe('false')
          })
        }
      }
    }
    const reads = gets; const profileReads = profiles; commit()
    await new Promise(resolve => setTimeout(resolve, 120))
    expect(showSuccess).not.toHaveBeenCalled(); expect(showError).not.toHaveBeenCalled()
    expect(gets).toBe(reads); expect(profiles).toBe(profileReads); expect(posts).toBe(1)
  })
  it('keeps a committed result when profile refresh fails, then refresh retries the balance', async () => {
    await start(); profileFails = true; commit()
    await vi.waitFor(() => expect(wrapper!.find('[aria-busy="true"]').exists()).toBe(false))
    expect(wrapper!.find('input').exists()).toBe(false)
    expect(wrapper!.text()).toContain(i18n.global.t('quiz.done', { score: 1, total: 1, reward: '0.10' }))
    expect(showSuccess).toHaveBeenCalledOnce(); expect(showError).not.toHaveBeenCalled(); expect(auth.user?.balance).toBe(1)
    profileFails = false; await wrapper!.get('[data-testid="refresh-quiz"]').trigger('click')
    await vi.waitFor(() => expect(auth.user?.balance).toBe(1.1))
    expect(posts).toBe(1)
  })
})
