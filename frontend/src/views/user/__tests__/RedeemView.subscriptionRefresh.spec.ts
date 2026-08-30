import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import RedeemView from '../RedeemView.vue'

const redeem = vi.hoisted(() => vi.fn())
const getHistory = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const refreshActiveSubscriptionState = vi.hoisted(() => vi.fn())
const fetchPublicSettings = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showWarning = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 0, concurrency: 1 },
    refreshUser,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    supportContact: null,
    fetchPublicSettings,
    showSuccess,
    showError,
    showWarning,
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({ refreshActiveSubscriptionState }),
}))

vi.mock('@/api', () => ({
  redeemAPI: { redeem, getHistory },
}))

describe('RedeemView subscription refresh', () => {
  beforeEach(() => {
    redeem.mockReset().mockResolvedValue({
      message: 'Subscription assigned',
      type: 'subscription',
      value: 30,
    })
    getHistory.mockReset().mockResolvedValue([])
    refreshUser.mockReset().mockResolvedValue(undefined)
    refreshActiveSubscriptionState.mockReset().mockResolvedValue(undefined)
    fetchPublicSettings.mockReset().mockResolvedValue(undefined)
    showSuccess.mockReset()
    showError.mockReset()
    showWarning.mockReset()
  })

  it('refreshes normalized header progress immediately after a subscription redemption', async () => {
    const wrapper = mount(RedeemView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          SupportContactPanel: true,
          LoadingSpinner: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.find('#code').setValue('SUB-2026')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(redeem).toHaveBeenCalledWith('SUB-2026')
    expect(refreshActiveSubscriptionState).toHaveBeenCalledWith(true)
    expect(showWarning).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
  })
})
