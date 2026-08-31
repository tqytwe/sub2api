import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CheckInView from '@/views/user/CheckInView.vue'
import { useAuthStore } from '@/stores/auth'

const {
  getCheckinStatus,
  checkin,
  checkinMakeup,
  refreshUser,
  showError,
  showInfo,
  showSuccess,
  trackQuestCompleteOnce,
} = vi.hoisted(() => ({
  getCheckinStatus: vi.fn(),
  checkin: vi.fn(),
  checkinMakeup: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn(),
  trackQuestCompleteOnce: vi.fn(),
}))

vi.mock('@/api/play', () => ({
  default: {
    getCheckinStatus,
    checkin,
    checkinMakeup,
  },
}))

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({
    user: { id: 61, balance: 0 },
    refreshUser,
  })
  return { useAuthStore: () => state }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showInfo, showSuccess }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/utils/growthAnalytics', () => ({ trackQuestCompleteOnce }))

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
  user: { id: number; balance: number }
  refreshUser: typeof refreshUser
}

const explorerStatus = {
  enabled: true,
  eligible: true,
  checked_in_today: false,
  reward_amount: 0.5,
  coupon_pool_ready: false,
  coupon_weight_bp: 8000,
  redeem_code_weight_bp: 2000,
  balance_weight_bp: 0,
  server_date: '2026-08-29',
  growth_energy_enabled: true,
  redeemable_reward_eligible: false,
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
}

function mountView() {
  return mount(CheckInView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('CheckInView', () => {
  beforeEach(() => {
    authState.user = { id: 61, balance: 0 }
    authState.refreshUser = refreshUser
    getCheckinStatus.mockReset().mockResolvedValue(explorerStatus)
    checkin.mockReset()
    checkinMakeup.mockReset()
    refreshUser.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
    trackQuestCompleteOnce.mockReset()
  })

  it('keeps explorer check-in available and confirms only growth energy', async () => {
    checkin.mockResolvedValue({
      reward_amount: 0,
      balance_added: 0,
      reward_type: 'none',
      growth_energy: 1,
      server_date: '2026-08-29',
      growth_eligibility: explorerStatus.growth_eligibility,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('checkin.energyHint')
    expect(wrapper.text()).toContain('checkin.energyTitle')
    expect(wrapper.text()).not.toContain('checkin.randomRewardHint')
    const button = wrapper.get('button.gw-btn-primary')
    expect(button.attributes('disabled')).toBeUndefined()

    await button.trigger('click')
    await flushPromises()

    expect(checkin).toHaveBeenCalledOnce()
    expect(showSuccess).toHaveBeenCalledWith('checkin.energySuccess:{"amount":1}')
    expect(showSuccess).not.toHaveBeenCalledWith(expect.stringContaining('checkin.success'))
    expect(trackQuestCompleteOnce).toHaveBeenCalledWith('checkin')
    expect(refreshUser).toHaveBeenCalledOnce()
    expect(getCheckinStatus).toHaveBeenCalledTimes(2)
    expect(showError).not.toHaveBeenCalled()
  })
})
