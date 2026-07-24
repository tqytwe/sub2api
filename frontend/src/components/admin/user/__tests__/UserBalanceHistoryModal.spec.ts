import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  getUserBalanceHistory: vi.fn(),
  getUserBalanceReconciliation: vi.fn(),
  listByUserSubscriptions: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserBalanceHistory: apiMocks.getUserBalanceHistory,
      getUserBalanceReconciliation: apiMocks.getUserBalanceReconciliation,
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
    template: '<div v-if="show"><h2>{{ title }}</h2><slot /></div>',
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
    items: [
      {
        id: 'play_reward:12',
        type: 'blindbox',
        source_type: 'play_reward_ledger',
        source_id: '12',
        amount: 0,
        balance_delta: 0,
        frozen_delta: 0,
        balance_before: null,
        balance_after: null,
        frozen_before: null,
        frozen_after: null,
        occurred_at: '2026-07-19T10:00:00Z',
        description: '盲盒净变动',
        actor_type: 'system',
        actor_user_id: null,
        related_object_type: 'play_blindbox_open',
        related_object_id: '88',
        reference: 'blindbox:1024:2026-07-19',
        notes: '',
        metadata: {
          blindbox_open_id: 88,
          cost_amount: 0.5,
          reward_amount: 0.5,
          net_amount: 0,
        },
        confidence: 'high',
      },
      {
        id: 'play_reward:11',
        type: 'quiz',
        source_type: 'play_reward_ledger',
        source_id: '11',
        amount: 0.5,
        balance_delta: 0.5,
        frozen_delta: 0,
        balance_before: null,
        balance_after: null,
        frozen_before: null,
        frozen_after: null,
        occurred_at: '2026-07-19T09:55:00Z',
        description: '答题奖励',
        actor_type: 'system',
        actor_user_id: null,
        related_object_type: 'play_reward_ledger',
        related_object_id: '11',
        reference: 'quiz:1024:2026-07-19',
        notes: '',
        metadata: { attempt_date: '2026-07-19' },
        confidence: 'high',
      },
      {
        id: 'play_reward:10',
        type: 'checkin',
        source_type: 'play_reward_ledger',
        source_id: '10',
        amount: 0.5,
        balance_delta: 0.5,
        frozen_delta: 0,
        balance_before: null,
        balance_after: null,
        frozen_before: null,
        frozen_after: null,
        occurred_at: '2026-07-19T09:50:00Z',
        description: '签到奖励',
        actor_type: 'system',
        actor_user_id: null,
        related_object_type: 'play_reward_ledger',
        related_object_id: '10',
        reference: 'checkin:1024:2026-07-19',
        notes: '',
        metadata: { checkin_date: '2026-07-19' },
        confidence: 'high',
      },
    ],
    total: 3,
    page: 1,
    page_size: 15,
    pages: 1,
    summary: {
      current_balance: 1,
      frozen_balance: 0,
      total_in: 1,
      total_out: 0,
      net_delta: 1,
      recharge_total: 0,
    },
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
  apiMocks.getUserBalanceReconciliation.mockResolvedValue({
    current_balance: 1,
    current_frozen: 0,
    ledger_balance_sum: 1,
    ledger_frozen_sum: 0,
    balance_difference: 0,
    frozen_difference: 0,
    recent: [],
    warnings: [],
  })
})

describe('UserBalanceHistoryModal', () => {
  it('renders balance flow summary, reward rows, and expandable blindbox details', async () => {
    const wrapper = mount(UserBalanceHistoryModal, {
      props: { show: false, user: user as any },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(apiMocks.getUserBalanceHistory).toHaveBeenCalledWith(1024, 1, 15, undefined)
    expect(wrapper.html()).toContain('admin.users.balanceHistoryTitle')
    expect(wrapper.text()).toContain('$1.00')
    expect(wrapper.text()).toContain('+$0.50')
    expect(wrapper.text()).toContain('blindbox:1024:2026-07-19')
    expect(wrapper.text()).toContain('quiz:1024:2026-07-19')
    expect(wrapper.text()).toContain('checkin:1024:2026-07-19')

    const details = wrapper.findAll('button[title="admin.users.flowDetails"]')
    expect(details.length).toBeGreaterThan(0)
    await details[0].trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('cost_amount')
    expect(wrapper.text()).toContain('reward_amount')
    expect(wrapper.html()).toContain('text-emerald-600')
  })

  it('renders fund-management ledger types with localized labels and visible reasons', async () => {
    apiMocks.getUserBalanceHistory.mockResolvedValueOnce({
      items: [
        {
          id: 'balance_transaction:9002',
          type: 'ops_gift',
          source_type: 'balance_transactions',
          source_id: '1024:1784728015759699561',
          amount: 30,
          balance_delta: 30,
          frozen_delta: 0,
          balance_before: 1,
          balance_after: 31,
          frozen_before: 0,
          frozen_after: 0,
          occurred_at: '2026-07-22T13:46:55Z',
          description: 'administrator gift balance',
          actor_type: 'admin',
          actor_user_id: 1,
          related_object_type: 'ops_gift',
          related_object_id: '1024:1784728015759699561',
          reference: 'ops_gift:1024:1784728015759699561',
          notes: '',
          metadata: { reason: '人工赠送新用户余额' },
          confidence: 'high',
        },
        {
          id: 'balance_transaction:9001',
          type: 'offline_recharge',
          source_type: 'balance_transactions',
          source_id: 'wire-1001',
          amount: 100,
          balance_delta: 100,
          frozen_delta: 0,
          balance_before: 31,
          balance_after: 131,
          frozen_before: 0,
          frozen_after: 0,
          occurred_at: '2026-07-22T13:40:00Z',
          description: 'offline recharge confirmed',
          actor_type: 'admin',
          actor_user_id: 1,
          related_object_type: 'offline_recharge',
          related_object_id: 'wire-1001',
          reference: 'offline_recharge:wire-1001',
          notes: '',
          metadata: { reason: '线下转账到账', external_ref: 'wire-1001' },
          confidence: 'high',
        },
        {
          id: 'balance_transaction:9000',
          type: 'fund_refund_reject',
          source_type: 'balance_transactions',
          source_id: 'FR202607220001',
          amount: 30,
          balance_delta: 30,
          frozen_delta: 0,
          balance_before: 1,
          balance_after: 31,
          frozen_before: 0,
          frozen_after: 0,
          occurred_at: '2026-07-22T13:30:00Z',
          description: 'recharge refund request restored',
          actor_type: 'admin',
          actor_user_id: 1,
          related_object_type: 'fund_refund_reject',
          related_object_id: 'FR202607220001',
          reference: 'fund_refund_reject:FR202607220001',
          notes: '',
          metadata: { reason: '资料不一致' },
          confidence: 'high',
        },
      ],
      total: 3,
      page: 1,
      page_size: 15,
      pages: 1,
      summary: {
        current_balance: 131,
        frozen_balance: 0,
        total_in: 160,
        total_out: 0,
        net_delta: 160,
        recharge_total: 100,
      },
    })

    const wrapper = mount(UserBalanceHistoryModal, {
      props: { show: false, user: user as any },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    const rowTitles = wrapper.findAll('[data-test="flow-title"]').map((node) => node.text())
    const rowDescriptions = wrapper.findAll('[data-test="flow-description"]').map((node) => node.text())
    const rowNotes = wrapper.findAll('[data-test="flow-notes"]').map((node) => node.text())

    expect(rowTitles).toEqual([
      'admin.users.typeOpsGift',
      'admin.users.typeOfflineRecharge',
      'admin.users.typeFundRefundReject',
    ])
    expect(rowDescriptions).toEqual([
      'admin.users.flowDescriptionOpsGift',
      'admin.users.flowDescriptionOfflineRecharge',
      'admin.users.flowDescriptionFundRefundRestored',
    ])
    expect(rowNotes).toEqual(['人工赠送新用户余额', '线下转账到账', '资料不一致'])
    expect(rowDescriptions).not.toContain('administrator gift balance')
    expect(rowDescriptions).not.toContain('offline recharge confirmed')
    expect(rowDescriptions).not.toContain('recharge refund request restored')
    expect(wrapper.find('[data-test="type-filter"]').text()).toContain('admin.users.typeWithdrawalSubmit')
    expect(wrapper.find('[data-test="flow-balance-range"]').classes()).toContain('whitespace-nowrap')

    await wrapper.find('[data-test="type-filter"]').setValue('ops_gift')
    await flushPromises()

    expect(apiMocks.getUserBalanceHistory).toHaveBeenCalledWith(1024, 1, 15, 'ops_gift')
  })

  it('uses user subscriptions instead of balance flow when filtering subscription records', async () => {
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
