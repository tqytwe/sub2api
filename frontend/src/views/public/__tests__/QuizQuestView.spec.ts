import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import QuizQuestView from '@/views/public/QuizQuestView.vue'
import { useAuthStore } from '@/stores/auth'

const {
  getQuizToday,
  submitQuiz,
  refreshUser,
  showError,
  showInfo,
  showSuccess,
} = vi.hoisted(() => ({
  getQuizToday: vi.fn(),
  submitQuiz: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/play', () => ({
  default: {
    getQuizToday,
    submitQuiz,
  },
}))

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({
    isAuthenticated: false,
    user: null as { id: number } | null,
    token: null as string | null,
    refreshUser,
  })
  return { useAuthStore: () => state }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showInfo, showSuccess }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

enableAutoUnmount(afterEach)

const authState = useAuthStore() as unknown as {
  isAuthenticated: boolean
  user: { id: number } | null
  token: string | null
  refreshUser: typeof refreshUser
}

function mountView() {
  return mount(QuizQuestView, {
    global: {
      stubs: {
        AuthenticatedPlayShell: { template: '<div><slot /></div>' },
        PublicPageToolbar: true,
        PublicPlayBackLink: true,
        SupportFloatingCard: true,
        CouponRewardCard: {
          props: ['coupon'],
          template: '<div data-testid="coupon-reward"><span>{{ coupon.name }}</span><time>{{ coupon.expires_at }}</time></div>',
        },
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('QuizQuestView', () => {
  beforeEach(() => {
    authState.isAuthenticated = false
    authState.user = null
    authState.token = null
    refreshUser.mockReset()
    authState.refreshUser = refreshUser
    submitQuiz.mockReset()
    showError.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
    getQuizToday.mockReset().mockResolvedValue({
      enabled: true,
      coupon_pool_ready: true,
      questions: Array.from({ length: 5 }, (_, index) => ({
        id: index + 1,
        prompt: `Question ${index + 1}`,
        options: ['A', 'B'],
      })),
      already_submitted: false,
      reward_per_correct: 0.1,
      server_date: '2026-07-27',
    })
  })

  it('shows the full balance reward for the complete quiz rather than a per-correct amount', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('quiz.rewardHint:{"amount":"0.50"}')
    expect(wrapper.text()).not.toContain('quiz.rewardHint:{"amount":"0.10"}')
  })

  it('restores a coupon reward after the completed quiz page is refreshed', async () => {
    getQuizToday.mockResolvedValueOnce({
      enabled: true,
      coupon_pool_ready: true,
      questions: [],
      already_submitted: true,
      previous_score: 4,
      previous_total: 5,
      previous_reward: 0,
      previous_reward_type: 'coupon',
      previous_coupon: {
        user_coupon_id: 18,
        template_id: 7,
        name: 'Recharge 3 off',
        benefit_type: 'fixed_amount',
        benefit_value: 3,
        currency: 'USD',
        applicable_scopes: ['balance'],
        minimum_order_amount: 10,
        valid_from: '2026-07-27T00:00:00Z',
        expires_at: '2026-07-30T00:00:00Z',
      },
      previous_coupon_pool_version: 'quiz-launch-v1',
      reward_per_correct: 0.1,
      server_date: '2026-07-27',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="coupon-reward"]').text()).toContain('Recharge 3 off')
    expect(wrapper.get('[data-testid="coupon-reward"]').text()).toContain('2026-07-30T00:00:00Z')
    expect(wrapper.text()).toContain('quiz.couponDone:{"score":4,"total":5}')
    expect(wrapper.text()).not.toContain('quiz.done:{"score":4,"total":5,"reward":"0.00"}')
  })

  it('ignores a delayed completed-quiz response after the account changes', async () => {
    let resolveAccountA: ((value: Record<string, unknown>) => void) | undefined
    authState.isAuthenticated = true
    authState.user = { id: 41 }
    authState.token = 'account-a'
    getQuizToday
      .mockReset()
      .mockReturnValueOnce(new Promise((resolve) => { resolveAccountA = resolve }))
      .mockResolvedValueOnce({
        enabled: true,
        coupon_pool_ready: true,
        questions: [{ id: 99, prompt: 'Account B question', options: ['A', 'B'] }],
        already_submitted: false,
        reward_per_correct: 0.1,
        server_date: '2026-07-27',
      })

    const wrapper = mountView()
    await Promise.resolve()

    authState.token = 'account-b'
    authState.user = { id: 42 }
    await flushPromises()
    expect(wrapper.text()).toContain('Account B question')

    resolveAccountA?.({
      enabled: true,
      coupon_pool_ready: true,
      questions: [],
      already_submitted: true,
      previous_score: 5,
      previous_total: 5,
      previous_reward: 0,
      previous_reward_type: 'coupon',
      previous_coupon: {
        user_coupon_id: 18,
        template_id: 7,
        name: 'Account A coupon',
        benefit_type: 'fixed_amount',
        benefit_value: 3,
        currency: 'USD',
        applicable_scopes: ['balance'],
        minimum_order_amount: 10,
        valid_from: '2026-07-27T00:00:00Z',
        expires_at: '2026-07-30T00:00:00Z',
      },
      reward_per_correct: 0.1,
      server_date: '2026-07-27',
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('Account A coupon')
    expect(wrapper.text()).toContain('Account B question')
  })

  it('keeps a completed quiz successful when the non-critical account refresh fails', async () => {
    authState.isAuthenticated = true
    refreshUser.mockRejectedValueOnce(new Error('account refresh unavailable'))
    submitQuiz.mockResolvedValueOnce({
      score: 5,
      total: 5,
      reward_amount: 0.5,
      reward_type: 'balance',
      server_date: '2026-07-27',
    })

    const wrapper = mountView()
    await flushPromises()
    const radios = wrapper.findAll('input[type="radio"]')
    for (let index = 0; index < radios.length; index += 2) {
      await radios[index].setValue()
    }

    await wrapper.get('.play-btn-primary').trigger('click')
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('quiz.success:{"score":5,"total":5,"reward":"0.50"}')
    expect(showError).not.toHaveBeenCalled()
    expect(getQuizToday).toHaveBeenCalledTimes(2)
  })

  it('keeps explorer participation available when the redeemable reward pool is unavailable', async () => {
    authState.isAuthenticated = true
    authState.user = { id: 61 }
    authState.token = 'explorer'
    getQuizToday.mockResolvedValue({
      enabled: true,
      coupon_pool_ready: false,
      questions: [{ id: 1, prompt: 'Explorer question', options: ['A', 'B'] }],
      already_submitted: false,
      reward_per_correct: 0.1,
      server_date: '2026-08-29',
      growth_eligibility: {
        tier: 'explorer',
        reward_mode: 'energy',
        primary_reason: 'no_recent_activity',
        email_verified: true,
        account_age_days: 8,
        has_recent_usage: false,
        net_balance_recharge_30d: 0,
        has_active_subscription: false,
        progress: {
          email_verified: true, account_age_days: 8, minimum_account_age_days: 3,
          account_age_requirement_met: true, has_recent_usage: false,
          net_balance_recharge_30d: 0, minimum_recharge_cny: 10,
          has_active_subscription: false, activity_requirement_met: false,
          next_action: 'no_recent_activity',
        },
      },
    })
    submitQuiz.mockResolvedValue({
      score: 1,
      total: 1,
      reward_amount: 0,
      reward_type: 'none',
      growth_energy: 1,
      server_date: '2026-08-29',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('quiz.energyHint')
    expect(wrapper.text()).not.toContain('quiz.couponPoolUnavailable')
    await wrapper.get('input[type="radio"]').setValue()
    await wrapper.get('.play-btn-primary').trigger('click')
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('quiz.energySuccess:{"score":1,"total":1,"amount":1}')
  })

  it('does not render or toast a delayed submission result from the previous account', async () => {
    let resolveAccountASubmit: ((value: Record<string, unknown>) => void) | undefined
    authState.isAuthenticated = true
    authState.user = { id: 51 }
    authState.token = 'account-a'
    submitQuiz.mockReturnValueOnce(new Promise((resolve) => { resolveAccountASubmit = resolve }))

    const wrapper = mountView()
    await flushPromises()
    const radios = wrapper.findAll('input[type="radio"]')
    for (let index = 0; index < radios.length; index += 2) {
      await radios[index].setValue()
    }

    await wrapper.get('.play-btn-primary').trigger('click')
    authState.token = 'account-b'
    authState.user = { id: 52 }
    await flushPromises()

    resolveAccountASubmit?.({
      score: 5,
      total: 5,
      reward_amount: 0,
      reward_type: 'coupon',
      coupon: {
        user_coupon_id: 19,
        template_id: 8,
        name: 'Account A delayed coupon',
        benefit_type: 'fixed_amount',
        benefit_value: 3,
        currency: 'USD',
        applicable_scopes: ['balance'],
        minimum_order_amount: 10,
        valid_from: '2026-07-27T00:00:00Z',
        expires_at: '2026-07-30T00:00:00Z',
      },
      server_date: '2026-07-27',
    })
    await flushPromises()

    expect(showSuccess).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('Account A delayed coupon')
  })
})
