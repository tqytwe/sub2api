import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CouponWalletPanel from '../CouponWalletPanel.vue'
import { useAuthStore } from '@/stores/auth'

const getMyCoupons = vi.hoisted(() => vi.fn())

vi.mock('@/api/coupon', () => ({
  default: { getMyCoupons },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: 'en' }),
  }
})

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({
    isAuthenticated: true,
    user: { id: 9 },
    token: 'wallet-account-a',
  })
  return { useAuthStore: () => state }
})

const authState = useAuthStore() as unknown as {
  isAuthenticated: boolean
  user: { id: number } | null
  token: string | null
}

enableAutoUnmount(afterEach)

const coupon = {
  id: 18,
  template_id: 3,
  template_name: 'Subscription coupon',
  user_id: 9,
  status: 'used' as const,
  source: 'quiz' as const,
  terms_snapshot: {
    template_id: 3,
    template_version: 1,
    template_key: 'subscription',
    name: 'Subscription coupon',
    benefit_type: 'percentage' as const,
    benefit_value: 20,
    currency: 'USD',
    applicable_scopes: ['subscription' as const],
    minimum_order_amount: 10,
    eligible_plan_ids: [],
    validity_mode: 'relative_days' as const,
  },
  issued_at: '2026-07-27T00:00:00Z',
  valid_from: '2026-07-27T00:00:00Z',
  expires_at: '2026-08-03T00:00:00Z',
  created_at: '2026-07-27T00:00:00Z',
  updated_at: '2026-07-27T00:00:00Z',
}

const lockedCoupon = {
  ...coupon,
  id: 19,
  status: 'locked' as const,
  locked_order_id: 71,
  locked_at: '2026-07-27T01:00:00Z',
}

describe('CouponWalletPanel', () => {
  beforeEach(() => {
    authState.isAuthenticated = true
    authState.user = { id: 9 }
    authState.token = 'wallet-account-a'
    getMyCoupons.mockReset().mockResolvedValue({ data: { items: [coupon], total: 1, page: 1, page_size: 10, pages: 1 } })
  })

  it('loads the available wallet tab and requests the selected status when switching', async () => {
    const wrapper = mount(CouponWalletPanel, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()
    expect(getMyCoupons).toHaveBeenCalledWith({ page: 1, page_size: 10, status: 'available' })

    await wrapper.findAll('[role="tab"]')[2].trigger('click')
    await flushPromises()
    expect(getMyCoupons).toHaveBeenLastCalledWith({ page: 1, page_size: 10, status: 'used' })
    expect(wrapper.text()).toContain('Subscription coupon')
  })

  it('shows a separate processing tab for a coupon locked by an order without use actions', async () => {
    getMyCoupons
      .mockResolvedValueOnce({ data: { items: [coupon], total: 1, page: 1, page_size: 10, pages: 1 } })
      .mockResolvedValueOnce({ data: { items: [lockedCoupon], total: 1, page: 1, page_size: 10, pages: 1 } })
    const wrapper = mount(CouponWalletPanel, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    await flushPromises()

    expect(getMyCoupons).toHaveBeenLastCalledWith({ page: 1, page_size: 10, status: 'locked' })
    expect(wrapper.text()).toContain('locked')
    expect(wrapper.text()).toContain('coupon.wallet.lockedHint')
    expect(wrapper.find('span.bg-amber-50').exists()).toBe(true)
    expect(wrapper.findAll('a')).toHaveLength(0)
  })

  it('labels a future available coupon as pending and removes payment shortcuts', async () => {
    getMyCoupons.mockResolvedValue({
      data: {
        items: [{ ...coupon, id: 20, status: 'available', valid_from: '2099-01-01T00:00:00Z' }],
        total: 1,
        page: 1,
        page_size: 10,
        pages: 1,
      },
    })
    const wrapper = mount(CouponWalletPanel, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('coupon.wallet.pending')
    expect(wrapper.text()).toContain('coupon.wallet.availableAt')
    expect(wrapper.findAll('a')).toHaveLength(0)
  })

  it('does not render a delayed wallet response from a previous account', async () => {
    let resolveAccountA: ((value: { data: Record<string, unknown> }) => void) | undefined
    const accountBCoupon = { ...coupon, id: 27, template_name: 'Account B coupon' }
    getMyCoupons
      .mockReset()
      .mockReturnValueOnce(new Promise((resolve) => { resolveAccountA = resolve }))
      .mockResolvedValueOnce({ data: { items: [accountBCoupon], total: 1, page: 1, page_size: 10, pages: 1 } })

    const wrapper = mount(CouponWalletPanel, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await Promise.resolve()
    authState.token = 'wallet-account-b'
    authState.user = { id: 10 }
    await flushPromises()
    expect(wrapper.text()).toContain('Account B coupon')

    resolveAccountA?.({ data: { items: [{ ...coupon, template_name: 'Account A coupon' }], total: 1, page: 1, page_size: 10, pages: 1 } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('Account A coupon')
    expect(wrapper.text()).toContain('Account B coupon')
  })
})
