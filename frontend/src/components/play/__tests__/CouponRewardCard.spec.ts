import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CouponRewardCard from '../CouponRewardCard.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('CouponRewardCard', () => {
  it('shows a future activation time and withholds payment links until the coupon is active', () => {
    const wrapper = mount(CouponRewardCard, {
      props: {
        coupon: {
          user_coupon_id: 18,
          template_id: 7,
          name: 'Future recharge coupon',
          benefit_type: 'fixed_amount',
          benefit_value: 3,
          currency: 'USD',
          applicable_scopes: ['balance'],
          minimum_order_amount: 10,
          valid_from: '2099-01-01T00:00:00Z',
          expires_at: '2099-01-03T00:00:00Z',
        },
      },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })

    expect(wrapper.text()).toContain('coupon.reward.availableAt')
    expect(wrapper.text()).toContain('coupon.reward.pendingHint')
    expect(wrapper.findAll('a')).toHaveLength(0)
  })
})
