// Actual production API/Pinia/Vue backed by the disposable PostgreSQL HTTP fixture.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import BlindboxView from '@/views/public/BlindboxView.vue'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import playAPI from '@/api/play'
import zh from '@/i18n/locales/zh'
vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const { showSuccess, showError, showInfo } = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError, showInfo }) }))
vi.mock('@/router', () => ({ default: { push: vi.fn() } }))
enableAutoUnmount(afterEach)
afterAll(() => vi.unstubAllGlobals())
const baseURL = process.env.WELFARE_CONTRACT_BASE_URL
const mode = process.env.WELFARE_CONTRACT_MODE
const userID = Number(process.env.WELFARE_CONTRACT_USER)
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const mountView = (pinia: ReturnType<typeof createPinia>) => mount(BlindboxView, { global: { plugins: [pinia, i18n], stubs: {
  AuthenticatedPlayShell: { template: '<div><slot /></div>' }, PublicPageToolbar: true,
  PublicPlayBackLink: true, SupportFloatingCard: true, RouterLink: { template: '<a><slot /></a>' },
  RewardCelebrationOverlay: { props: ['open', 'title', 'amount', 'details'], template: '<div v-if="open" data-testid="settled-reward">{{title}} {{amount}} {{details}}</div>' },
} } })
describe.skipIf(!baseURL)(`blindbox ${mode} actual PostgreSQL recovery`, () => {
  it('replays the last unknown open after remount despite the exhausted limit/balance', async () => {
    expect(baseURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/api\/v1$/); expect(userID).toBeGreaterThan(0)
    const pinia = createPinia(); setActivePinia(pinia)
    const auth = useAuthStore(); auth.token = 'fixture-only'
    apiClient.defaults.baseURL = baseURL; apiClient.defaults.adapter = 'http'
    apiClient.defaults.headers.common['X-Welfare-Test-User'] = String(userID)
    delete apiClient.defaults.headers.common['X-Welfare-Test-Lose-Response']
    await auth.refreshUser(); sessionStorage.clear(); vi.clearAllMocks()
    const wrapper = mountView(pinia)
    await vi.waitFor(() => expect(wrapper.find('button.play-btn-primary').attributes('disabled')).toBeUndefined())
    apiClient.defaults.headers.common['X-Welfare-Test-Lose-Response'] = '1'
    const open = wrapper.get('button.play-btn-primary')
    await Promise.all([open.trigger('click'), open.trigger('click')])
    await vi.waitFor(() => expect(showError).toHaveBeenCalledOnce())
    const key = sessionStorage.getItem(`blindbox.pending-open:${userID}`)
    expect(key).toBeTruthy()
    const settled = await playAPI.getBlindboxStatus()
    expect(settled.opens_today).toBe(2); expect(settled.can_open).toBe(false)
    wrapper.unmount()
    const reloaded = mountView(pinia)
    await vi.waitFor(() => expect(reloaded.get('[data-testid="blindbox-resume"]').attributes('disabled')).toBeUndefined())
    await reloaded.get('[data-testid="blindbox-resume"]').trigger('click')
    await vi.waitFor(() => expect(reloaded.find('[data-testid="settled-reward"]').exists()).toBe(true))
    await vi.waitFor(() => expect(auth.user?.balance).toBe(mode === 'balance' ? .4 : 0))
    expect(reloaded.get('[data-testid="settled-reward"]').text()).toContain(mode === 'balance' ? '$0.20' : mode === 'coupon' ? '隔离福利券' : 'FIXTURE-')
    expect(sessionStorage.getItem(`blindbox.pending-open:${userID}`)).toBeNull()
    expect((await playAPI.getBlindboxStatus()).opens_today).toBe(2)
    const replay = await playAPI.openBlindbox(key!)
    expect(replay.reward_type).toBe(mode); expect(replay.opens_today).toBe(2)
    expect(showError).toHaveBeenCalledOnce()
  }, 15000)
})
