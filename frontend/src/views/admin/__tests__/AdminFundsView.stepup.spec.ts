import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminFundsView from '@/views/admin/AdminFundsView.vue'

const {
  approveRefundRequest,
  executeSignupGift30,
  getRefundSensitivePayout,
  grantGift,
  grantOfflineRecharge,
  listRefundRequests,
  markRefundPaid,
  previewSignupGift30,
  rejectRefundRequest,
  routeState,
  routerPush,
  stepUpRun,
} = vi.hoisted(() => ({
  approveRefundRequest: vi.fn(),
  executeSignupGift30: vi.fn(),
  getRefundSensitivePayout: vi.fn(),
  grantGift: vi.fn(),
  grantOfflineRecharge: vi.fn(),
  listRefundRequests: vi.fn(),
  markRefundPaid: vi.fn(),
  previewSignupGift30: vi.fn(),
  rejectRefundRequest: vi.fn(),
  routeState: {
    path: '/admin/funds/grants',
    params: { tab: 'grants' as string },
  },
  routerPush: vi.fn(),
  stepUpRun: vi.fn(),
}))

vi.mock('@/api/admin/funds', () => ({
  default: {
    approveRefundRequest,
    executeSignupGift30,
    getRefundSensitivePayout,
    grantGift,
    grantOfflineRecharge,
    listRefundRequests,
    markRefundPaid,
    previewSignupGift30,
    rejectRefundRequest,
  },
  approveRefundRequest,
  executeSignupGift30,
  getRefundSensitivePayout,
  grantGift,
  grantOfflineRecharge,
  listRefundRequests,
  markRefundPaid,
  previewSignupGift30,
  rejectRefundRequest,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: routerPush }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    visible: { value: false },
    blockedReason: { value: '' },
    prompt: vi.fn(),
    onVerified: vi.fn(),
    onCancel: vi.fn(),
    run: stepUpRun,
  }),
  isStepUpCancelled: (error: unknown) => Boolean((error as { code?: string })?.code === 'STEP_UP_CANCELLED'),
}))

const messages: Record<string, string> = {
  'admin.funds.title': '资金管理',
  'admin.funds.description': '统一处理充值退回、线下充值、赠送余额和历史赠金复核。',
  'admin.funds.tabs.refunds': '退款申请',
  'admin.funds.tabs.grants': '赠送与线下充值',
  'admin.funds.tabs.classification': '历史赠金复核',
  'admin.funds.grants.giftTitle': '赠送余额',
  'admin.funds.grants.offlineTitle': '线下充值入账',
  'admin.funds.grants.submitGift': '确认赠送',
  'admin.funds.grants.submitOffline': '确认线下充值',
  'admin.funds.forms.userId': '用户 ID',
  'admin.funds.forms.amount': '金额，整数',
  'admin.funds.forms.reason': '原因，至少说明业务背景',
  'admin.funds.forms.externalRef': '外部付款凭证或备注编号',
  'admin.funds.validation.wholeAmountRequired': '金额必须是大于 0 的整数，例如 30',
  'admin.funds.validation.reasonTooShort': '原因至少需要 {min} 个字符',
  'admin.funds.validation.reasonTooLong': '原因不能超过 {max} 个字符',
  'admin.funds.validation.userRequired': '请输入正确的用户 ID',
  'admin.funds.validation.classificationSelectionRequired': '请选择至少一条历史赠金候选',
  'admin.funds.classification.title': '历史 30 赠金复核',
  'admin.funds.classification.description': '预览第一个 30 美元管理员加余额候选项，确认后分类为新用户赠金；不会直接改旧流水。',
  'admin.funds.classification.preview': '预览候选',
  'admin.funds.classification.empty': '暂无可复核候选',
  'admin.funds.classification.remaining': '估算剩余',
  'admin.funds.classification.reasonPlaceholder': '执行原因，10 到 500 个字符',
  'admin.funds.classification.execute': '确认分类 {count} 条',
  'admin.funds.table.select': '选择',
  'admin.funds.table.user': '用户',
  'admin.funds.table.transaction': '流水',
  'admin.funds.table.amount': '金额',
  'admin.funds.table.createdAt': '创建时间',
  'admin.funds.messages.giftGranted': '赠送余额已入账',
  'admin.funds.messages.grantFailed': '入账失败',
  'admin.funds.messages.classified': '已分类 {count} 条历史赠金',
  'admin.funds.messages.classificationFailed': '历史赠金复核失败',
  'admin.funds.loading': '加载中...',
  'common.refresh': '刷新',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string, params?: Record<string, unknown>) => {
        let value = messages[key] || key
        Object.entries(params || {}).forEach(([name, replacement]) => {
          value = value.replace(`{${name}}`, String(replacement))
        })
        return value
      },
    }),
  }
})

function mountView(tab: 'grants' | 'classification' = 'grants') {
  routeState.path = `/admin/funds/${tab}`
  routeState.params = { tab }
  return mount(AdminFundsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        TotpStepUpDialog: true,
      },
    },
  })
}

async function fillGiftForm(wrapper: ReturnType<typeof mount>, amount = '39') {
  const grantsTab = wrapper.findAll('button').find((button) => button.text() === '赠送与线下充值')
  if (grantsTab) {
    await grantsTab.trigger('click')
    await flushPromises()
  }
  const firstForm = wrapper.findAll('form')[0]
  expect(firstForm).toBeTruthy()
  const inputs = firstForm.findAll('input')
  await inputs[0].setValue('261')
  await inputs[1].setValue(amount)
  await firstForm.find('textarea').setValue('新用户注册赠送30')
  return firstForm
}

describe('AdminFundsView step-up actions', () => {
  beforeEach(() => {
    approveRefundRequest.mockReset()
    executeSignupGift30.mockReset()
    getRefundSensitivePayout.mockReset()
    grantGift.mockReset()
    grantOfflineRecharge.mockReset()
    listRefundRequests.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    markRefundPaid.mockReset()
    previewSignupGift30.mockReset().mockResolvedValue({
      mode: 'preview',
      generated_at: '2026-07-22T00:00:00Z',
      candidate_count: 1,
      candidates: [{
        user_id: 209,
        user_email: 'user@example.com',
        transaction_id: 14000,
        amount: '30',
        created_at: '2026-07-20T18:06:44Z',
        recommended_kind: 'signup_gift',
        estimated_remaining: '25.13',
        estimated_consumed: '4.87',
        current_balance: '25.13',
        existing_classified_funds: '0',
      }],
    })
    routerPush.mockReset()
    stepUpRun.mockReset().mockImplementation(async (action: () => Promise<unknown>) => {
      try {
        return await action()
      } catch (error) {
        if ((error as { reason?: string })?.reason === 'STEP_UP_REQUIRED') {
          return action()
        }
        throw error
      }
    })
  })

  it('retries gift grant through step-up instead of surfacing a generic 403', async () => {
    grantGift
      .mockRejectedValueOnce({ status: 403, reason: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ id: 9, source_type: 'ops_gift' })

    const wrapper = mountView('grants')
    const firstForm = await fillGiftForm(wrapper)
    await firstForm.trigger('submit')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(grantGift).toHaveBeenCalledTimes(2)
    expect(grantGift).toHaveBeenLastCalledWith({
      user_id: 261,
      amount: '39',
      reason: '新用户注册赠送30',
    })
    expect(wrapper.text()).toContain('赠送余额已入账')
  })

  it('shows local validation before sending decimal gift amounts', async () => {
    const wrapper = mountView('grants')
    const firstForm = await fillGiftForm(wrapper, '30.00')
    await firstForm.trigger('submit')
    await flushPromises()

    expect(stepUpRun).not.toHaveBeenCalled()
    expect(grantGift).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('金额必须是大于 0 的整数，例如 30')
  })

  it('retries historical signup gift classification through step-up', async () => {
    executeSignupGift30
      .mockRejectedValueOnce({ status: 403, reason: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ mode: 'execute', affected_count: 1, candidates: [] })

    const wrapper = mountView('classification')
    await flushPromises()
    await wrapper.find('input[placeholder="执行原因，10 到 500 个字符"]').setValue('将历史首笔30美元管理员加余额复核为新用户赠金')
    await wrapper.get('[data-testid="admin-funds-execute-classification"]').trigger('click')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(executeSignupGift30).toHaveBeenCalledTimes(2)
    expect(executeSignupGift30).toHaveBeenLastCalledWith({
      transaction_ids: [14000],
      reason: '将历史首笔30美元管理员加余额复核为新用户赠金',
    })
    expect(wrapper.text()).toContain('已分类 1 条历史赠金')
  })
})
