// Run through scripts/test-welfare-contract.sh. Actual HTTP, Pinia, Axios and PG.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import QuizQuestView from '@/views/public/QuizQuestView.vue'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import playAPI from '@/api/play'
import zh from '@/i18n/locales/zh'
vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
const { showSuccess, showError, showInfo } = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError, showInfo }) }))
enableAutoUnmount(afterEach)
afterAll(() => vi.unstubAllGlobals())
const baseURL = process.env.WELFARE_CONTRACT_BASE_URL
const mode = process.env.WELFARE_CONTRACT_MODE
const fixtures = JSON.parse(process.env.WELFARE_CONTRACT_FIXTURES || '{}') as Record<string, { user: number; answers: { QuestionID: number; ChoiceIndex: number }[] }>
const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
const mountView = (pinia: ReturnType<typeof createPinia>) => mount(QuizQuestView, { global: { plugins: [pinia, i18n], stubs: {
  AuthenticatedPlayShell: { template: '<div><slot /></div>' }, PublicPageToolbar: true,
  PublicPlayBackLink: true, SupportFloatingCard: true, RouterLink: { template: '<a><slot /></a>' },
} } })
async function answer(wrapper: ReturnType<typeof mountView>, answers: { QuestionID: number; ChoiceIndex: number }[]) {
  await vi.waitFor(() => expect(wrapper.findAll('.quiz-question-card')).toHaveLength(2))
  for (const a of answers) await wrapper.get(`#quiz-question-${a.QuestionID}`).findAll('input')[a.ChoiceIndex].setValue()
}
function settledText() {
  return i18n.global.t(mode === 'energy' ? 'quiz.energyDone' : mode === 'coupon' ? 'quiz.couponDone' : 'quiz.done', { score: 2, total: 2, amount: 1, reward: '0.20' })
}
describe.skipIf(!baseURL)(`quiz ${mode} actual PostgreSQL roundtrip`, () => {
  for (const label of ['normal', 'unknown']) it(`${label}: click, retry and remount reconcile the same single grant`, async () => {
    expect(baseURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/api\/v1$/)
    const fixture = fixtures[label]
    expect(fixture.user).toBeGreaterThan(0)
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore(); auth.token = 'fixture-only'
    apiClient.defaults.baseURL = baseURL; apiClient.defaults.adapter = 'http'
    apiClient.defaults.headers.common['X-Welfare-Test-User'] = String(fixture.user)
    delete apiClient.defaults.headers.common['X-Welfare-Test-Lose-Response']
    await auth.refreshUser(); vi.clearAllMocks()
    const stalePinia = createPinia()
    const staleAuth = useAuthStore(stalePinia); staleAuth.token = 'fixture-only'; await staleAuth.refreshUser()
    const first = mountView(pinia); const stale = mountView(stalePinia)
    await answer(first, fixture.answers); await answer(stale, fixture.answers)
    await auth.refreshUser()
    expect(first.findAll('input:checked')).toHaveLength(2)
    if (label === 'unknown') apiClient.defaults.headers.common['X-Welfare-Test-Lose-Response'] = '1'
    const submit = first.get('button.play-btn-primary')
    await Promise.all([submit.trigger('click'), submit.trigger('click')])
    if (label === 'unknown') {
      await vi.waitFor(() => expect(showError).toHaveBeenCalledOnce())
      expect(showSuccess).not.toHaveBeenCalled()
      // The server committed before closing the socket. Retry hits real 409.
      expect((await playAPI.getQuizToday()).already_submitted).toBe(true)
      await vi.waitFor(() => expect(submit.attributes('disabled')).toBeUndefined())
      await submit.trigger('click')
      await vi.waitFor(() => expect(showInfo).toHaveBeenCalledWith(i18n.global.t('quiz.alreadyDone')))
    } else {
      await vi.waitFor(() => expect(showSuccess).toHaveBeenCalledOnce())
      expect(showError).not.toHaveBeenCalled()
      if (mode === 'redeem_code') expect(showSuccess.mock.calls[0][0]).toContain('兑换码')
    }
    await vi.waitFor(() => expect(first.find('[aria-busy="true"]').exists()).toBe(false))
    expect(first.find('input').exists()).toBe(false)
    expect(first.text()).toContain(mode === 'redeem_code' ? '兑换码' : settledText())
    expect(auth.user?.balance).toBe(mode === 'balance' ? 1.2 : 1)
    const settled = await playAPI.getQuizToday()
    expect(settled.already_submitted).toBe(true)
    expect(settled.previous_reward_type).toBe(mode === 'energy' ? 'none' : mode)
    await stale.get('button.play-btn-primary').trigger('click')
    await vi.waitFor(() => expect(stale.find('[aria-busy="true"]').exists()).toBe(false))
    expect(stale.find('input').exists()).toBe(false)
    expect(stale.text()).toContain(mode === 'redeem_code' ? '兑换码' : settledText())
    expect(showInfo).toHaveBeenCalledWith(i18n.global.t('quiz.alreadyDone'))
    expect(staleAuth.user?.balance).toBe(mode === 'balance' ? 1.2 : 1)
    first.unmount(); stale.unmount()
    const reloaded = mountView(pinia)
    if (mode === 'redeem_code') {
      expect(settled.previous_redeem_code?.code).toMatch(/^FIXTURE-/)
      await vi.waitFor(() => expect(reloaded.text()).toContain(settled.previous_redeem_code!.code))
    } else {
      await vi.waitFor(() => expect(reloaded.text()).toContain(settledText()))
      if (mode === 'coupon') expect(reloaded.text()).toContain('隔离福利券')
    }
    expect(reloaded.find('input').exists()).toBe(false)
    expect(auth.user?.balance).toBe(mode === 'balance' ? 1.2 : 1)
    await expect(playAPI.submitQuiz(fixture.answers.map(a => ({ question_id: a.QuestionID, choice_index: a.ChoiceIndex })))).rejects.toMatchObject({ status: 409, reason: 'PLAY_QUIZ_ALREADY_DONE' })
  }, 15000)
})
