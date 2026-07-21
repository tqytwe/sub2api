import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import WalletView from '@/views/user/WalletView.vue'

const { getWalletSummaryMock, getWalletTransactionsMock } = vi.hoisted(() => ({
  getWalletSummaryMock: vi.fn(),
  getWalletTransactionsMock: vi.fn(),
}))

vi.mock('@/api/wallet', () => ({
  getWalletSummary: (...args: unknown[]) => getWalletSummaryMock(...args),
  getWalletTransactions: (...args: unknown[]) => getWalletTransactionsMock(...args),
}))

const messages: Record<string, string> = {
  'wallet.title': '钱包',
  'wallet.description': '查看余额、任务预留和统一流水。',
  'wallet.available': '可用余额',
  'wallet.withdrawable': '可提现',
  'wallet.pendingWithdrawable': '待解冻',
  'wallet.withdrawalFrozen': '提现冻结',
  'wallet.taskReserved': '任务预留',
  'wallet.totalCredits': '累计入账',
  'wallet.totalDebits': '累计扣减',
  'wallet.transactions': '余额流水',
  'wallet.transactionCount': '共 {count} 条流水',
  'wallet.sourceLabel': '来源筛选',
  'wallet.source.all': '全部来源',
  'wallet.source.team_reward': '战队奖励',
  'wallet.source.arena_reward': '农场奖励',
  'wallet.source.recharge': '充值',
  'wallet.source.redeem': '兑换',
  'wallet.source.affiliate': '邀请返利',
  'wallet.source.checkin': '签到奖励',
  'wallet.source.quiz': '答题奖励',
  'wallet.source.blind_box': '盲盒',
  'wallet.source.usage': '用量扣费',
  'wallet.source.image_task': '图片任务',
  'wallet.source.refund': '退款',
  'wallet.source.admin_adjustment': '管理员调整',
  'wallet.source.promotion': '活动赠送',
  'wallet.source.subscription': '订阅',
  'wallet.source.other': '其他',
  'wallet.direction.credit': '入账',
  'wallet.direction.debit': '扣减',
  'wallet.direction.neutral': '无变化',
  'wallet.table.time': '时间',
  'wallet.table.source': '来源',
  'wallet.table.direction': '方向',
  'wallet.table.amount': '金额',
  'wallet.table.balanceAfter': '变动后余额',
  'wallet.table.taskReservedChange': '任务预留变动',
  'wallet.pageInfo': '第 {page} / {pages} 页',
  'wallet.loading': '加载中...',
  'wallet.empty': '暂无余额流水',
  'wallet.loadFailed': '加载钱包数据失败',
  'common.refresh': '刷新',
  'common.previous': '上一页',
  'common.next': '下一页',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
      locale: { value: 'zh' },
    }),
  }
})

function mountView() {
  return mount(WalletView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('WalletView', () => {
  beforeEach(() => {
    getWalletSummaryMock.mockReset().mockResolvedValue({
      available_balance: '42.50000000',
      withdrawable_balance: '12.00000000',
      pending_withdrawable_balance: '4.50000000',
      withdrawal_frozen_balance: '1.25000000',
      task_reserved_balance: '3.25000000',
      total_credits: '100.00000000',
      total_debits: '57.50000000',
      transaction_count: 2,
      last_transaction_at: '2026-07-21T10:00:00Z',
    })
    getWalletTransactionsMock.mockReset().mockResolvedValue({
      items: [
        {
          id: 88,
          source: 'team_reward',
          direction: 'credit',
          balance_delta: '12.34000000',
          frozen_delta: '0.00000000',
          balance_after: '58.34000000',
          frozen_after: '0.00000000',
          created_at: '2026-07-21T10:00:00Z',
        },
        {
          id: 87,
          source: 'image_task',
          direction: 'neutral',
          balance_delta: '-2.50000000',
          frozen_delta: '2.50000000',
          balance_after: '55.84000000',
          frozen_after: '2.50000000',
          created_at: '2026-07-21T09:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
  })

  it('renders localized wallet balances and safe transaction rows', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('钱包')
    expect(wrapper.text()).toContain('可用余额')
    expect(wrapper.text()).toContain('US$42.50')
    expect(wrapper.text()).toContain('可提现')
    expect(wrapper.text()).toContain('US$12.00')
    expect(wrapper.text()).toContain('待解冻')
    expect(wrapper.text()).toContain('US$4.50')
    expect(wrapper.text()).toContain('提现冻结')
    expect(wrapper.text()).toContain('US$1.25')
    expect(wrapper.text()).toContain('任务预留')
    expect(wrapper.text()).toContain('战队奖励')
    expect(wrapper.text()).toContain('入账')
    expect(wrapper.text()).toContain('图片任务')
    expect(wrapper.text()).toContain('无变化')
    expect(wrapper.text()).toContain('任务预留变动 +US$2.50')
    const transactionRows = wrapper.get('tbody').html()
    expect(transactionRows).not.toContain('metadata')
    expect(transactionRows).not.toContain('source_id')
    expect(transactionRows).not.toContain('admin')
    expect(transactionRows).not.toContain('email')
  })
})
