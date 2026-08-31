import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import GrowthGovernanceOperations from '@/components/admin/play/GrowthGovernanceOperations.vue'

const {
  getGrowthCohort,
  getGrowthGovernance,
  approveGrowthGovernance,
  revokeGrowthGovernance,
  showError,
  showSuccess,
  stepUpRun,
} = vi.hoisted(() => ({
  getGrowthCohort: vi.fn(),
  getGrowthGovernance: vi.fn(),
  approveGrowthGovernance: vi.fn(),
  revokeGrowthGovernance: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  stepUpRun: vi.fn(),
}))

vi.mock('@/api/admin/play', () => ({
  default: {
    getGrowthCohort,
    getGrowthGovernance,
    approveGrowthGovernance,
    revokeGrowthGovernance,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    visible: { value: false },
    blockedReason: { value: '' },
    onVerified: vi.fn(),
    onCancel: vi.fn(),
    run: stepUpRun,
  }),
  isStepUpCancelled: (error: unknown) => (error as { code?: string })?.code === 'STEP_UP_CANCELLED',
  isStepUpBlocked: (error: unknown) => (error as { reason?: string })?.reason === 'STEP_UP_TOTP_NOT_ENABLED',
  stepUpBlockReason: () => 'STEP_UP_TOTP_NOT_ENABLED',
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.playOps.growthGovernance.title': '增长福利治理',
    'admin.playOps.growthGovernance.description': '服务端统一控制奖励资格、预算和灰度。',
    'admin.playOps.growthGovernance.refresh': '刷新数据',
    'admin.playOps.growthGovernance.cohort.title': '两周观察 cohort',
    'admin.playOps.growthGovernance.cohort.incomplete': '指标不完整，不能批准',
    'admin.playOps.growthGovernance.cohort.available': '指标完整',
    'admin.playOps.growthGovernance.metrics.unavailable': '暂不可用',
    'admin.playOps.growthGovernance.metrics.realCall7d': '7 日真实调用',
    'admin.playOps.growthGovernance.metrics.realCall30d': '30 日真实调用',
    'admin.playOps.growthGovernance.metrics.firstRecharge': '首充转化',
    'admin.playOps.growthGovernance.metrics.couponRedemption': '优惠券核销',
    'admin.playOps.growthGovernance.metrics.d7Retention': 'D7 留存',
    'admin.playOps.growthGovernance.metrics.abnormalRedemption': '异常兑换率',
    'admin.playOps.growthGovernance.metrics.appealFalsePositive': '误伤申诉率',
    'admin.playOps.growthGovernance.metrics.actualRewardCost': '实际奖励成本',
    'admin.playOps.growthGovernance.metrics.participation': '参与人数',
    'admin.playOps.growthGovernance.status.title': '当前治理状态',
    'admin.playOps.growthGovernance.status.none': '未批准',
    'admin.playOps.growthGovernance.status.approved': '已批准',
    'admin.playOps.growthGovernance.status.revoked': '已撤销',
    'admin.playOps.growthGovernance.actions.approve': '审核并批准',
    'admin.playOps.growthGovernance.actions.revoke': '撤销批准',
    'admin.playOps.growthGovernance.approve.title': '审核增长奖励灰度',
    'admin.playOps.growthGovernance.approve.submit': '确认批准',
    'admin.playOps.growthGovernance.form.budget': '预算上限',
    'admin.playOps.growthGovernance.form.rollout': '灰度比例',
    'admin.playOps.growthGovernance.form.reason': '审核理由',
    'admin.playOps.growthGovernance.form.reasonHint': '请填写 10 至 500 个字符。',
    'admin.playOps.growthGovernance.success.approved': '增长奖励灰度已批准',
    'admin.playOps.growthGovernance.success.revoked': '增长奖励灰度已撤销',
  }
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string) => messages[key] || key,
    }),
  }
})

type Cohort = ReturnType<typeof cohortWith>

function cohortWith(available: boolean) {
  return {
    window_start: '2026-07-31T00:00:00Z',
    window_end: '2026-08-14T00:00:00Z',
    metrics_available: available,
    participation_users: 1248,
    real_call_7d_users: 526,
    real_call_7d_ratio: 0.421,
    real_call_30d_users: 396,
    real_call_30d_ratio: 0.317,
    first_recharge_users: 107,
    first_recharge_ratio: 0.086,
    coupons_issued: 373,
    coupons_redeemed: 53,
    coupon_redemption_ratio: 0.142,
    actual_reward_cost: 23.75,
    d7_retained_users: 473,
    d7_retention_ratio: 0.379,
    abnormal_redemption_users: available ? 14 : 0,
    abnormal_redemption_ratio: available ? 0.011 : null,
    appeal_count: available ? 4 : 0,
    false_positive_appeals: available ? 1 : 0,
    appeal_false_positive_ratio: available ? 0.0008 : null,
    unavailable_metrics: available ? [] : ['abnormal_redemption_ratio', 'appeal_false_positive_ratio'],
  }
}

function governance(
  decision: 'none' | 'approved' | 'revoked' = 'none',
  cohort: Cohort = cohortWith(false),
  active = decision === 'approved',
) {
  return {
    id: decision === 'none' ? 0 : 9,
    decision,
    approved: active,
    budget_amount: decision === 'approved' ? 100 : 0,
    budget_spent: decision === 'approved' ? 12 : 0,
    budget_remaining: decision === 'approved' ? 88 : 0,
    rollout_percent: decision === 'approved' ? 10 : 0,
    cohort,
    rule_version: decision === 'none' ? '' : 'v1',
    reason: decision === 'none' ? '' : 'The reviewed cohort supports a limited rollout.',
    created_at: decision === 'none' ? '' : '2026-08-31T10:00:00Z',
  }
}

function mountView() {
  return mount(GrowthGovernanceOperations, {
    global: {
      stubs: {
        Icon: true,
        TotpStepUpDialog: true,
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show" :data-title="title"><slot /><slot name="footer" /></section>',
        },
      },
    },
  })
}

describe('GrowthGovernanceOperations', () => {
  beforeEach(() => {
    getGrowthCohort.mockReset().mockResolvedValue(cohortWith(false))
    getGrowthGovernance.mockReset().mockResolvedValue(governance())
    approveGrowthGovernance.mockReset().mockResolvedValue(governance('approved', cohortWith(true)))
    revokeGrowthGovernance.mockReset().mockResolvedValue(governance('revoked', cohortWith(true)))
    showError.mockReset()
    showSuccess.mockReset()
    stepUpRun.mockReset().mockImplementation((action: () => Promise<unknown>) => action())
  })

  it('renders server cohort evidence and never turns unavailable risk metrics into zero', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getGrowthCohort).toHaveBeenCalledOnce()
    expect(getGrowthGovernance).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('两周观察 cohort')
    expect(wrapper.text()).toContain('暂不可用')
    expect(wrapper.text()).toContain('指标不完整，不能批准')
    expect(wrapper.get('[data-testid="growth-governance-approval-open"]').attributes('disabled')).toBeDefined()
  })

  it('submits only the current complete cohort through step-up with budget, rollout, and audited reason', async () => {
    const complete = cohortWith(true)
    getGrowthCohort.mockResolvedValue(complete)
    getGrowthGovernance.mockResolvedValue(governance('none', complete))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="growth-governance-approval-open"]').trigger('click')
    await wrapper.get('[data-testid="growth-governance-budget"]').setValue('125')
    await wrapper.get('[data-testid="growth-governance-rollout"]').setValue('15')
    await wrapper.get('[data-testid="growth-governance-approval-reason"]').setValue('The complete cohort supports a controlled and auditable rollout.')
    const submit = wrapper.get('[data-testid="growth-governance-approval-submit"]')
    expect(submit.attributes('type')).toBe('submit')
    expect(submit.attributes('form')).toBe('growth-governance-approval-form')
    await wrapper.get('[data-testid="growth-governance-approval-dialog"]').trigger('submit')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledOnce()
    expect(approveGrowthGovernance).toHaveBeenCalledWith({
      budget_amount: 125,
      rollout_percent: 15,
      cohort: complete,
      reason: 'The complete cohort supports a controlled and auditable rollout.',
    })
    expect(showSuccess).toHaveBeenCalledWith('增长奖励灰度已批准')
  })

  it('shows an inline retryable error state when read-only evidence cannot load', async () => {
    getGrowthCohort.mockRejectedValueOnce({ reason: 'PLAY_GROWTH_GOVERNANCE_UNAVAILABLE' })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="growth-governance-error"]').exists()).toBe(true)
    await wrapper.get('[data-testid="growth-governance-retry"]').trigger('click')
    await flushPromises()
    expect(getGrowthCohort).toHaveBeenCalledTimes(2)
  })

  it('keeps an inactive latest approval revocable instead of offering a second approval', async () => {
    const complete = cohortWith(true)
    getGrowthCohort.mockResolvedValue(complete)
    getGrowthGovernance.mockResolvedValue(governance('approved', complete, false))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="growth-governance-approval-open"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="growth-governance-revoke-open"]').attributes('disabled')).toBeUndefined()
  })
})
