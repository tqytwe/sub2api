import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CouponSelector from '../CouponSelector.vue'
import { useAuthStore } from '@/stores/auth'

const getMyCoupons = vi.hoisted(() => vi.fn())
const quotePaymentCoupon = vi.hoisted(() => vi.fn())

vi.mock('@/api/coupon', () => ({
  default: { getMyCoupons, quotePaymentCoupon },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => `${key}${params ? JSON.stringify(params) : ''}` }),
  }
})

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({
    isAuthenticated: true,
    user: { id: 9 },
    token: 'coupon-account-a',
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
  id: 12,
  template_id: 3,
  template_name: 'First recharge',
  user_id: 9,
  status: 'available' as const,
  source: 'blindbox' as const,
  terms_snapshot: {
    template_id: 3,
    template_version: 1,
    template_key: 'first-recharge',
    name: 'First recharge',
    benefit_type: 'fixed_amount' as const,
    benefit_value: 5,
    currency: 'USD',
    applicable_scopes: ['balance' as const],
    minimum_order_amount: 20,
    eligible_plan_ids: [],
    validity_mode: 'relative_days' as const,
  },
  issued_at: '2026-07-27T00:00:00Z',
  valid_from: '2026-07-27T00:00:00Z',
  expires_at: '2026-07-30T00:00:00Z',
  created_at: '2026-07-27T00:00:00Z',
  updated_at: '2026-07-27T00:00:00Z',
}

function mountSelector(props: Record<string, unknown> = {}) {
  return mount(CouponSelector, {
    props: {
      modelValue: null,
      amount: 30,
      orderType: 'balance',
      paymentType: 'stripe',
      ...props,
    },
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('CouponSelector', () => {
  beforeEach(() => {
    authState.isAuthenticated = true
    authState.user = { id: 9 }
    authState.token = 'coupon-account-a'
    getMyCoupons.mockReset().mockResolvedValue({ data: { items: [coupon], total: 1, page: 1, page_size: 50, pages: 1 } })
    quotePaymentCoupon.mockReset().mockResolvedValue({
      data: {
        user_coupon_id: 12,
        template_id: 3,
        list_amount: 30,
        gateway_base_amount: 25,
        discount_amount: 5,
        fee_amount: 0,
        pay_amount: 25,
        payment_currency: 'USD',
        qualifying_recharge_amount: 30,
      },
    })
  })

  it('loads available coupons, quotes the selected coupon, and emits the server result', async () => {
    const wrapper = mountSelector()
    await flushPromises()

    expect(getMyCoupons).toHaveBeenCalledWith({ page: 1, page_size: 50, status: 'available' })
    await wrapper.get('input[type="radio"]').setValue(true)
    await flushPromises()

    expect(quotePaymentCoupon).toHaveBeenCalledWith({
      coupon_id: 12,
      payment_type: 'stripe',
      order_type: 'balance',
      amount: 30,
      plan_id: undefined,
    })
    expect(wrapper.emitted('quote')?.at(-1)?.[0]).toMatchObject({ discount_amount: 5, pay_amount: 25 })
    expect(wrapper.text()).toContain('coupon.selector.quoteDiscount')
  })

  it('quotes a retained coupon after the selector mounts again', async () => {
    const wrapper = mountSelector({ modelValue: 12 })
    await flushPromises()

    expect(quotePaymentCoupon).toHaveBeenCalledWith({
      coupon_id: 12,
      payment_type: 'stripe',
      order_type: 'balance',
      amount: 30,
      plan_id: undefined,
    })
    expect(wrapper.emitted('quote')?.at(-1)?.[0]).toMatchObject({ user_coupon_id: 12 })
  })

  it('labels a future coupon and prevents it from being quoted before activation', async () => {
    getMyCoupons.mockResolvedValue({
      data: {
        items: [{ ...coupon, valid_from: '2099-01-01T00:00:00Z' }],
        total: 1,
        page: 1,
        page_size: 50,
        pages: 1,
      },
    })
    const wrapper = mountSelector()
    await flushPromises()

    expect(wrapper.text()).toContain('coupon.selector.availableAt')
    expect(wrapper.get('input[type="radio"]').attributes('disabled')).toBeDefined()
    expect(quotePaymentCoupon).not.toHaveBeenCalled()
  })

  it('keeps the selector usable when no coupons are available', async () => {
    getMyCoupons.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 50, pages: 1 } })
    const wrapper = mountSelector()
    await flushPromises()
    expect(wrapper.text()).toContain('coupon.selector.empty')
    expect(wrapper.find('input[type="radio"]').exists()).toBe(false)
  })

  it('paginates the available wallet so coupons beyond the first 50 remain selectable', async () => {
    const laterCoupon = {
      ...coupon,
      id: 72,
      template_id: 9,
      template_name: 'Later coupon',
      terms_snapshot: {
        ...coupon.terms_snapshot,
        template_id: 9,
        template_key: 'later-coupon',
        name: 'Later coupon',
      },
    }
    getMyCoupons
      .mockReset()
      .mockResolvedValueOnce({ data: { items: [coupon], total: 51, page: 1, page_size: 50, pages: 2 } })
      .mockResolvedValueOnce({ data: { items: [laterCoupon], total: 51, page: 2, page_size: 50, pages: 2 } })

    const wrapper = mountSelector()
    await flushPromises()

    await wrapper.get('[data-test="coupon-page-next"]').trigger('click')
    await flushPromises()

    expect(getMyCoupons).toHaveBeenLastCalledWith({ page: 2, page_size: 50, status: 'available' })
    expect(wrapper.text()).toContain('Later coupon')
    await wrapper.get('input[type="radio"]').setValue(true)
    await flushPromises()

    expect(quotePaymentCoupon).toHaveBeenLastCalledWith(expect.objectContaining({ coupon_id: 72 }))
  })

  it('re-quotes a selected coupon when the amount or payment method changes', async () => {
    const wrapper = mountSelector()
    await flushPromises()

    await wrapper.get('input[type="radio"]').setValue(true)
    await flushPromises()
    await wrapper.setProps({ modelValue: 12, amount: 45, paymentType: 'alipay' })
    await flushPromises()

    expect(quotePaymentCoupon).toHaveBeenLastCalledWith({
      coupon_id: 12,
      payment_type: 'alipay',
      order_type: 'balance',
      amount: 45,
      plan_id: undefined,
    })
    expect(wrapper.emitted('quote')?.at(-1)?.[0]).toMatchObject({ discount_amount: 5, pay_amount: 25 })
  })

  it('cancels an in-flight quote when paging so the next page remains usable', async () => {
    const laterCoupon = {
      ...coupon,
      id: 72,
      template_id: 9,
      template_name: 'Later coupon',
      terms_snapshot: {
        ...coupon.terms_snapshot,
        template_id: 9,
        template_key: 'later-coupon',
        name: 'Later coupon',
      },
    }
    let resolveQuote: ((value: { data: Record<string, unknown> }) => void) | undefined
    getMyCoupons
      .mockReset()
      .mockResolvedValueOnce({ data: { items: [coupon], total: 51, page: 1, page_size: 50, pages: 2 } })
      .mockResolvedValueOnce({ data: { items: [laterCoupon], total: 51, page: 2, page_size: 50, pages: 2 } })
    quotePaymentCoupon.mockReset().mockReturnValueOnce(new Promise((resolve) => {
      resolveQuote = resolve
    }))

    const wrapper = mountSelector()
    await flushPromises()
    await wrapper.get('input[type="radio"]').setValue(true)
    await flushPromises()
    expect(wrapper.get('input[type="radio"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="coupon-page-next"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Later coupon')
    expect(wrapper.get('input[type="radio"]').attributes('disabled')).toBeUndefined()

    resolveQuote?.({
      data: {
        user_coupon_id: 12,
        template_id: 3,
        list_amount: 30,
        gateway_base_amount: 25,
        discount_amount: 5,
        fee_amount: 0,
        pay_amount: 25,
        payment_currency: 'USD',
        qualifying_recharge_amount: 30,
      },
    })
    await flushPromises()

    expect(wrapper.emitted('quote')?.some(([value]) => (value as { user_coupon_id?: number } | null)?.user_coupon_id === 12)).toBe(false)
  })

  it('cancels a delayed quote when the parent clears the selected coupon', async () => {
    let resolveQuote: ((value: { data: Record<string, unknown> }) => void) | undefined
    quotePaymentCoupon.mockReset().mockReturnValueOnce(new Promise((resolve) => {
      resolveQuote = resolve
    }))

    const wrapper = mountSelector()
    await flushPromises()
    await wrapper.get('input[type="radio"]').setValue(true)
    await wrapper.setProps({ modelValue: 12 })
    await flushPromises()
    await wrapper.setProps({ modelValue: null })
    await flushPromises()

    expect(wrapper.get('input[type="radio"]').attributes('disabled')).toBeUndefined()

    resolveQuote?.({
      data: {
        user_coupon_id: 12,
        template_id: 3,
        list_amount: 30,
        gateway_base_amount: 25,
        discount_amount: 5,
        fee_amount: 0,
        pay_amount: 25,
        payment_currency: 'USD',
        qualifying_recharge_amount: 30,
      },
    })
    await flushPromises()

    expect(wrapper.emitted('quote')?.some(([value]) => (value as { user_coupon_id?: number } | null)?.user_coupon_id === 12)).toBe(false)
  })

  it('does not render a delayed coupon wallet response from a previous account', async () => {
    let resolveAccountA: ((value: { data: Record<string, unknown> }) => void) | undefined
    const accountBCoupon = {
      ...coupon,
      id: 13,
      template_id: 4,
      template_name: 'Account B coupon',
      terms_snapshot: { ...coupon.terms_snapshot, template_id: 4, name: 'Account B coupon' },
    }
    getMyCoupons
      .mockReset()
      .mockReturnValueOnce(new Promise((resolve) => { resolveAccountA = resolve }))
      .mockResolvedValueOnce({ data: { items: [accountBCoupon], total: 1, page: 1, page_size: 50, pages: 1 } })

    const wrapper = mountSelector()
    await Promise.resolve()
    authState.token = 'coupon-account-b'
    authState.user = { id: 10 }
    await flushPromises()
    expect(wrapper.text()).toContain('Account B coupon')

    resolveAccountA?.({ data: { items: [{ ...coupon, template_name: 'Account A coupon' }], total: 1, page: 1, page_size: 50, pages: 1 } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('Account A coupon')
    expect(wrapper.text()).toContain('Account B coupon')
  })

  it('drops a delayed quote when the account changes', async () => {
    let resolveQuote: ((value: { data: Record<string, unknown> }) => void) | undefined
    quotePaymentCoupon.mockReset().mockReturnValueOnce(new Promise((resolve) => {
      resolveQuote = resolve
    }))

    const wrapper = mountSelector()
    await flushPromises()
    await wrapper.get('input[type="radio"]').setValue(true)
    await Promise.resolve()

    authState.token = 'coupon-account-b'
    authState.user = { id: 10 }
    await flushPromises()
    resolveQuote?.({
      data: {
        user_coupon_id: 12,
        template_id: 3,
        list_amount: 30,
        gateway_base_amount: 25,
        discount_amount: 5,
        fee_amount: 0,
        pay_amount: 25,
        payment_currency: 'USD',
        qualifying_recharge_amount: 30,
      },
    })
    await flushPromises()

    expect(wrapper.emitted('quote')?.some(([value]) => (value as { user_coupon_id?: number } | null)?.user_coupon_id === 12)).toBe(false)
  })
})
