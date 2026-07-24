import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import WalletView from '@/views/user/WalletView.vue'

const {
  getWalletSummaryMock,
  getWalletTransactionsMock,
  getWithdrawalAvailabilityMock,
  getWithdrawalAccountMock,
  getWithdrawalsMock,
  getWithdrawalMock,
  updateWithdrawalAccountMock,
  createWithdrawalMock,
  cancelWithdrawalMock,
  getFundRefundRequestsMock,
  createFundRefundRequestMock,
  cancelFundRefundRequestMock,
} = vi.hoisted(() => ({
  getWalletSummaryMock: vi.fn(),
  getWalletTransactionsMock: vi.fn(),
  getWithdrawalAvailabilityMock: vi.fn(),
  getWithdrawalAccountMock: vi.fn(),
  getWithdrawalsMock: vi.fn(),
  getWithdrawalMock: vi.fn(),
  updateWithdrawalAccountMock: vi.fn(),
  createWithdrawalMock: vi.fn(),
  cancelWithdrawalMock: vi.fn(),
  getFundRefundRequestsMock: vi.fn(),
  createFundRefundRequestMock: vi.fn(),
  cancelFundRefundRequestMock: vi.fn(),
}))

vi.mock('@/api/wallet', () => ({
  getWalletSummary: (...args: unknown[]) => getWalletSummaryMock(...args),
  getWalletTransactions: (...args: unknown[]) => getWalletTransactionsMock(...args),
  getWithdrawalAvailability: (...args: unknown[]) => getWithdrawalAvailabilityMock(...args),
  getWithdrawalAccount: (...args: unknown[]) => getWithdrawalAccountMock(...args),
  getWithdrawals: (...args: unknown[]) => getWithdrawalsMock(...args),
  getWithdrawal: (...args: unknown[]) => getWithdrawalMock(...args),
  updateWithdrawalAccount: (...args: unknown[]) => updateWithdrawalAccountMock(...args),
  createWithdrawal: (...args: unknown[]) => createWithdrawalMock(...args),
  cancelWithdrawal: (...args: unknown[]) => cancelWithdrawalMock(...args),
  getFundRefundRequests: (...args: unknown[]) => getFundRefundRequestsMock(...args),
  createFundRefundRequest: (...args: unknown[]) => createFundRefundRequestMock(...args),
  cancelFundRefundRequest: (...args: unknown[]) => cancelFundRefundRequestMock(...args),
  normalizeWithdrawalWholeAmount: (value: string) => value.trim().replace(/\.0+$/, ''),
}))

const messages: Record<string, string> = {
  'wallet.title': '钱包',
  'wallet.description': '查看余额、任务预留和统一流水。',
  'wallet.available': '可用余额',
  'wallet.withdrawable': '可提现',
  'wallet.refundableRecharge': '可退充值',
  'wallet.giftBalance': '赠送余额',
  'wallet.pendingWithdrawable': '待解冻',
  'wallet.withdrawalFrozen': '提现冻结',
  'wallet.refundFrozen': '退回冻结',
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
  'wallet.source.withdrawal': '提现',
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
  'wallet.table.withdrawalFrozenChange': '提现冻结变动',
  'wallet.table.withdrawableChange': '可提现变动',
  'wallet.summaryHints.withdrawable': '仅包含奖励类可提现权益',
  'wallet.summaryHints.refundableRecharge': '真实充值未消费部分可申请退回',
  'wallet.summaryHints.giftBalance': '赠送余额可消费，默认不可提现或退回',
  'wallet.refunds.title': '充值退回',
  'wallet.refunds.description': '真实充值和线下充值的未消费部分可从这里申请退回。',
  'wallet.refunds.availableTitle': '可退金额',
  'wallet.refunds.onlineRecharge': '在线充值',
  'wallet.refunds.offlineRecharge': '线下充值',
  'wallet.refunds.frozen': '退回冻结',
  'wallet.refunds.newRequest': '新建退回申请',
  'wallet.refunds.type': '退回类型',
  'wallet.refunds.types.online_recharge_refund': '在线充值退回',
  'wallet.refunds.types.offline_recharge_refund': '线下充值退回',
  'wallet.refunds.amount': '退回金额',
  'wallet.refunds.reason': '退回原因',
  'wallet.refunds.reasonPlaceholder': '请说明退回原因',
  'wallet.refunds.submit': '提交退回申请',
  'wallet.refunds.submitting': '提交中...',
  'wallet.refunds.history': '退回记录',
  'wallet.refunds.requestCount': '共 {count} 笔退回申请',
  'wallet.refunds.empty': '暂无退回申请',
  'wallet.refunds.paidAt': '打款时间',
  'wallet.refunds.cancel': '取消申请',
  'wallet.refunds.canceling': '取消中...',
  'wallet.refunds.statusLabel': '退回状态',
  'wallet.refunds.status.pending_review': '待审核',
  'wallet.refunds.status.payout_pending': '待线下打款',
  'wallet.refunds.status.paid': '已打款',
  'wallet.refunds.status.rejected': '已拒绝',
  'wallet.refunds.status.canceled': '已取消',
  'wallet.refunds.validation.accountRequired': '请先设置收款账户',
  'wallet.refunds.validation.integerAmountRequired': '退回金额必须为整数',
  'wallet.refunds.validation.amountTooLarge': '退回金额超过可退余额',
  'wallet.refunds.submitSuccess': '退回申请已提交',
  'wallet.refunds.submitFailed': '提交退回申请失败',
  'wallet.refunds.cancelSuccess': '退回申请已取消',
  'wallet.refunds.cancelFailed': '取消退回申请失败',
  'wallet.withdrawals.requestTitle': '申请提现',
  'wallet.withdrawals.payoutAccount': '收款账户',
  'wallet.withdrawals.accountReady': '已设置',
  'wallet.withdrawals.accountMissing': '未设置',
  'wallet.withdrawals.method': '收款方式',
  'wallet.withdrawals.accountMask': '账户掩码',
  'wallet.withdrawals.recipientMask': '收款人掩码',
  'wallet.withdrawals.currency': '收款币种',
  'wallet.withdrawals.recipientName': '收款人姓名',
  'wallet.withdrawals.recipientNamePlaceholder': '请输入真实收款人姓名',
  'wallet.withdrawals.bankName': '银行名称',
  'wallet.withdrawals.bankNamePlaceholder': '请输入开户行或收款银行',
  'wallet.withdrawals.account': '收款账号',
  'wallet.withdrawals.accountPlaceholder': '请输入收款账号',
  'wallet.withdrawals.savingAccount': '保存中...',
  'wallet.withdrawals.saveAccount': '保存收款账户',
  'wallet.withdrawals.newRequest': '新建提现申请',
  'wallet.withdrawals.minimumAmount': '最低提现金额',
  'wallet.withdrawals.remainingDaily': '今日剩余额度',
  'wallet.withdrawals.amount': '提现金额',
  'wallet.withdrawals.submitting': '提交中...',
  'wallet.withdrawals.requestWithdrawal': '申请提现',
  'wallet.withdrawals.historyTitle': '提现记录',
  'wallet.withdrawals.requestCount': '共 {count} 笔申请',
  'wallet.withdrawals.paidAt': '打款时间',
  'wallet.withdrawals.viewHistory': '查看状态',
  'wallet.withdrawals.canceling': '取消中...',
  'wallet.withdrawals.cancel': '取消申请',
  'wallet.withdrawals.empty': '暂无提现申请',
  'wallet.withdrawals.statusHistory': '状态历史',
  'wallet.withdrawals.statusLabel': '状态',
  'wallet.withdrawals.noEvents': '暂无状态事件',
  'wallet.withdrawals.loadingAvailability': '正在读取提现权限...',
  'wallet.withdrawals.availableNow': '当前可提交提现申请',
  'wallet.withdrawals.accountSaved': '收款账户已保存',
  'wallet.withdrawals.accountSaveFailed': '保存收款账户失败',
  'wallet.withdrawals.submitSuccess': '提现申请已提交',
  'wallet.withdrawals.submitFailed': '提交提现申请失败',
  'wallet.withdrawals.cancelFailed': '取消提现申请失败',
  'wallet.withdrawals.methods.alipay': '支付宝',
  'wallet.withdrawals.methods.bank_transfer': '银行转账',
  'wallet.withdrawals.methods.other': '其他方式',
  'wallet.withdrawals.status.pending_review': '待审核',
  'wallet.withdrawals.status.second_review': '待二审',
  'wallet.withdrawals.status.payout_pending': '待线下打款',
  'wallet.withdrawals.status.paid': '已入账',
  'wallet.withdrawals.status.rejected': '已拒绝',
  'wallet.withdrawals.status.canceled': '已取消',
  'wallet.withdrawals.disabledReasons.global_disabled': '平台提现总开关未开启',
  'wallet.withdrawals.disabledReasons.user_disabled': '当前账号暂未开启提现',
  'wallet.withdrawals.disabledReasons.needs_review': '历史可提现重算仍需复核',
  'wallet.withdrawals.disabledReasons.disabled': '当前账号暂不可提现',
  'wallet.withdrawals.disabledReasons.unknown': '当前暂不可提现',
  'wallet.withdrawals.validation.accountRequired': '请填写收款人和收款账号',
  'wallet.withdrawals.validation.integerAmountRequired': '提现金额必须为整数',
  'wallet.withdrawals.validation.unavailable': '当前条件不满足',
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
      refundable_recharge_balance: '25.00000000',
      online_recharge_balance: '20.00000000',
      offline_recharge_balance: '5.00000000',
      gift_balance: '10.00000000',
      signup_gift_balance: '7.50000000',
      ops_gift_balance: '2.50000000',
      refund_frozen_balance: '3.00000000',
      unclassified_balance: '4.50000000',
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
          withdrawable_delta: '12.34000000',
          withdrawal_frozen_delta: '0.00000000',
          balance_after: '58.34000000',
          frozen_after: '0.00000000',
          withdrawable_after: '12.34000000',
          withdrawal_frozen_after: '0.00000000',
          created_at: '2026-07-21T10:00:00Z',
        },
        {
          id: 87,
          source: 'image_task',
          direction: 'neutral',
          balance_delta: '-2.50000000',
          frozen_delta: '2.50000000',
          withdrawable_delta: '0.00000000',
          withdrawal_frozen_delta: '0.00000000',
          balance_after: '55.84000000',
          frozen_after: '2.50000000',
          withdrawable_after: '12.34000000',
          withdrawal_frozen_after: '0.00000000',
          created_at: '2026-07-21T09:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getWithdrawalAvailabilityMock.mockReset().mockResolvedValue({
      global_enabled: false,
      user_enabled: false,
      can_apply: false,
      disabled_reason: 'global_disabled',
      recalc_status: 'ready',
      minimum_amount: '10.00',
      daily_limit_amount: '500.00',
      daily_used_amount: '0.00',
      remaining_daily_amount: '500.00',
    })
    getWithdrawalAccountMock.mockReset().mockResolvedValue(null)
    getWithdrawalsMock.mockReset().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 10,
      pages: 1,
    })
    getWithdrawalMock.mockReset()
    updateWithdrawalAccountMock.mockReset()
    createWithdrawalMock.mockReset()
    cancelWithdrawalMock.mockReset()
    getFundRefundRequestsMock.mockReset().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 10,
      pages: 1,
    })
    createFundRefundRequestMock.mockReset()
    cancelFundRefundRequestMock.mockReset()
  })

  it('renders localized wallet balances and safe transaction rows', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('钱包')
    expect(wrapper.text()).toContain('可用余额')
    expect(wrapper.text()).toContain('US$42.50')
    expect(wrapper.text()).toContain('可提现')
    expect(wrapper.text()).toContain('US$12.00')
    expect(wrapper.text()).toContain('可退充值')
    expect(wrapper.text()).toContain('US$25.00')
    expect(wrapper.text()).toContain('赠送余额')
    expect(wrapper.text()).toContain('US$10.00')
    expect(wrapper.text()).toContain('待解冻')
    expect(wrapper.text()).toContain('US$4.50')
    expect(wrapper.text()).toContain('提现冻结')
    expect(wrapper.text()).toContain('US$1.25')
    expect(wrapper.text()).toContain('退回冻结')
    expect(wrapper.text()).toContain('US$3.00')
    expect(wrapper.text()).toContain('充值退回')
    expect(wrapper.text()).toContain('在线充值')
    expect(wrapper.text()).toContain('线下充值')
    expect(wrapper.text()).toContain('任务预留')
    expect(wrapper.text()).toContain('战队奖励')
    expect(wrapper.text()).toContain('入账')
    expect(wrapper.text()).toContain('图片任务')
    expect(wrapper.text()).toContain('无变化')
    expect(wrapper.text()).toContain('任务预留变动 +US$2.50')
    expect(wrapper.text()).toContain('申请提现')
    expect(wrapper.text()).toContain('收款账户')
    expect(wrapper.text()).toContain('最低提现金额')
    expect(wrapper.text()).not.toContain('双人审核阈值')
    const transactionRows = wrapper.get('tbody').html()
    expect(transactionRows).not.toContain('metadata')
    expect(transactionRows).not.toContain('source_id')
    expect(transactionRows).not.toContain('admin')
    expect(transactionRows).not.toContain('email')
  })

  it('keeps the transaction ledger table full-width on desktop while scrollable on narrow screens', async () => {
    const wrapper = mountView()
    await flushPromises()

    const table = wrapper.get('table')
    expect(table.classes()).toEqual(expect.arrayContaining(['w-full', 'min-w-[820px]', 'table-fixed']))
    expect(table.classes()).not.toContain('min-w-[720px]')
    expect(wrapper.get('colgroup').findAll('col')).toHaveLength(5)
  })
})
