import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  getUserBalanceHistory: vi.fn(),
  listByUserSubscriptions: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserBalanceHistory: apiMocks.getUserBalanceHistory,
    },
    subscriptions: {
      listByUser: apiMocks.listByUserSubscriptions,
    },
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /></div>',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'Select',
    props: ['modelValue', 'options'],
    emits: ['update:modelValue', 'change'],
    template: `
      <select
        data-test="type-filter"
        :value="modelValue"
        @change="$emit('update:modelValue', $event.target.value); $emit('change')"
      >
        <option v-for="option in options" :key="String(option.value)" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    `,
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'Icon',
    props: ['name'],
    template: '<span :data-icon="name" />',
  },
}))

import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const user = {
  id: 1024,
  email: 'buyer@example.com',
  username: 'buyer',
  balance: 0,
  notes: '',
  created_at: '2026-07-18T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  apiMocks.getUserBalanceHistory.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 15,
    pages: 1,
    total_recharged: 0,
  })
  apiMocks.listByUserSubscriptions.mockResolvedValue([
    {
      id: 77,
      user_id: 1024,
      group_id: 11,
      status: 'active',
      starts_at: '2026-07-18T15:39:56Z',
      expires_at: '2026-08-17T15:39:56Z',
      daily_usage_usd: 0,
      weekly_usage_usd: 0,
      monthly_usage_usd: 0,
      daily_window_start: null,
      weekly_window_start: null,
      monthly_window_start: null,
      created_at: '2026-07-18T15:39:56Z',
      updated_at: '2026-07-18T15:39:56Z',
      assigned_at: '2026-07-18T15:40:56Z',
      notes: 'payment order 102',
      group: { id: 11, name: 'pro订阅套餐' },
      purchase_order: {
        id: 102,
        out_trade_no: 'sub2_20260718abc',
        payment_type: 'wxpay',
        pay_amount: 143,
        amount: 20,
        paid_at: '2026-07-18T15:39:56Z',
        subscription_days: 30,
        audit_action: 'SUBSCRIPTION_ASSIGNED',
      },
    },
  ])
})

describe('UserBalanceHistoryModal', () => {
  it('uses user subscriptions instead of redeem balance history when filtering subscription records', async () => {
    const wrapper = mount(UserBalanceHistoryModal, {
      props: { show: false, user: user as any },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(apiMocks.getUserBalanceHistory).toHaveBeenCalledWith(1024, 1, 15, undefined)

    await wrapper.find('[data-test="type-filter"]').setValue('subscription')
    await flushPromises()

    expect(apiMocks.listByUserSubscriptions).toHaveBeenCalledWith(1024)
    expect(apiMocks.getUserBalanceHistory).not.toHaveBeenCalledWith(1024, 1, 15, 'subscription')
    expect(wrapper.html()).toContain('pro订阅套餐')
    expect(wrapper.html()).toContain('#102')
    expect(wrapper.html()).toContain('wxpay')
    expect(wrapper.html()).toContain('143.00')
  })
})
