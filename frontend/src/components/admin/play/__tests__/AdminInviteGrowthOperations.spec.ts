import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminInviteGrowthOperations from '../AdminInviteGrowthOperations.vue'

const api = vi.hoisted(() => ({
  getInviteGrowthOverview: vi.fn(),
  listReferralCampaigns: vi.fn(),
  getReferralCampaign: vi.fn(),
  createReferralCampaign: vi.fn(),
  updateReferralCampaign: vi.fn(),
  setReferralCampaignStatus: vi.fn(),
  getReferralCampaignEarlyClosePreview: vi.fn(),
  earlyCloseReferralCampaign: vi.fn(),
  reviewReferralCampaign: vi.fn(),
  listReferralCampaignParticipants: vi.fn(),
  listReferralCampaignInvites: vi.fn(),
  listReferralCampaignRewards: vi.fn(),
  resolveReferralRewardDebt: vi.fn(),
}))

const store = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))
const stepUp = vi.hoisted(() => ({ run: vi.fn() }))

vi.mock('@/api/admin/play', () => ({ default: api }))
vi.mock('@/stores', () => ({ useAppStore: () => store }))
vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    visible: { value: false },
    blockedReason: { value: '' },
    run: stepUp.run,
    onVerified: vi.fn(),
    onCancel: vi.fn(),
  }),
  isStepUpCancelled: (cause: unknown) => (cause as { code?: string })?.code === 'STEP_UP_CANCELLED',
  isStepUpBlocked: (cause: unknown) => Boolean((cause as { code?: string })?.code?.startsWith('STEP_UP_')),
  stepUpBlockReason: (cause: unknown) => (cause as { code?: string })?.code || '',
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string, params?: Record<string, unknown>) => {
        const labels: Record<string, string> = {
          'admin.playOps.inviteGrowth.title': '邀请增长',
          'admin.playOps.inviteGrowth.newCampaign': '新建邀请活动',
          'admin.playOps.inviteGrowth.legacyRebate.title': '本地化返佣标题',
          'admin.playOps.inviteGrowth.legacyRebate.hint': '本地化返佣说明',
          'admin.playOps.inviteGrowth.legacyRebate.exclude': '本地化默认返佣规则',
          'admin.playOps.inviteGrowth.legacyRebate.stack': '本地化叠加返佣规则',
          'admin.playOps.inviteGrowth.publicRulesLabel': '本地化活动规则标签',
          'admin.playOps.inviteGrowth.inviteeNoticeLabel': '本地化受邀人说明标签',
          'admin.playOps.inviteGrowth.defaults.publicRules': '本地化活动规则默认内容',
          'admin.playOps.inviteGrowth.defaults.inviteeNotice': '本地化受邀人说明默认内容',
          'admin.playOps.inviteGrowth.maxLiability': '理论最大负债',
          'admin.playOps.inviteGrowth.tabs.participants': '参与者',
          'admin.playOps.inviteGrowth.tabs.invites': '邀请关联',
          'admin.playOps.inviteGrowth.tabs.rewards': '奖励记录',
          'admin.playOps.inviteGrowth.resolveDebt': '处理追缴',
          'admin.playOps.inviteGrowth.statuses.settling': '领奖中',
          'admin.playOps.inviteGrowth.earlyClose': '提前结束领奖',
          'admin.playOps.inviteGrowth.earlyCloseTitle': '提前结束领奖期',
          'admin.playOps.inviteGrowth.earlyCloseWarning': '未领取奖励将失效',
          'admin.playOps.inviteGrowth.earlyCloseReason': '结束原因',
          'admin.playOps.inviteGrowth.earlyCloseConfirm': '我确认作废未领取奖励',
          'admin.playOps.inviteGrowth.claimableRewards': '将作废',
          'admin.playOps.inviteGrowth.preservedRewards': '保留',
          'admin.playOps.inviteGrowth.claimDeadline': '领奖截止',
          'admin.playOps.inviteGrowth.rewardCountAmount': '{count} 笔 / {amount}',
        }
        let value = labels[key] || key
        for (const [name, replacement] of Object.entries(params || {})) {
          value = value.replace(`{${name}}`, String(replacement))
        }
        return value
      },
    }),
  }
})

const campaign = {
  id: 7,
  key: 'august-invite',
  name: '八月邀请争霸赛',
  status: 'draft',
  version: 2,
  registration_from: '2026-08-01T00:00:00Z',
  registration_to: '2026-08-10T00:00:00Z',
  starts_at: '2026-08-01T00:00:00Z',
  ends_at: '2026-08-31T00:00:00Z',
  qualification_to: '2026-09-05T00:00:00Z',
  claim_deadline: '2026-09-10T00:00:00Z',
  risk_hold_hours: 72,
  pay_threshold: 100,
  usage_threshold: 20,
  max_enrollments: 100,
  budget_total: 100000,
  budget_reserved: 300,
  budget_paid: 200,
  reward_mode: 'additive',
  created_by: 1,
}

const detail = {
  campaign,
  tiers: [
    { tier: 1, required_invites: 2, reward_amount: 100, currency: 'CNY' },
    { tier: 2, required_invites: 5, reward_amount: 300, currency: 'CNY' },
  ],
  stats: {
    campaign_id: 7,
    enrolled: 10,
    attributed: 8,
    qualified: 5,
    risk_pending: 2,
    risk_rejected: 1,
    rewards_reserved: 300,
    rewards_claimed: 200,
    rewards_expired: 0,
    rewards_revoked: 0,
  },
  approvals: [],
}

function page<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 20 }
}

function mountComponent() {
  return mount(AdminInviteGrowthOperations, {
    global: {
      stubs: {
        Icon: true,
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show" data-testid="dialog"><h2>{{ title }}</h2><slot/><slot name="footer"/></section>',
        },
      },
    },
  })
}

describe('AdminInviteGrowthOperations', () => {
  beforeEach(() => {
    Object.values(api).forEach(mock => mock.mockReset())
    stepUp.run.mockReset()
    stepUp.run.mockImplementation((action: () => Promise<unknown>) => action())
    store.showSuccess.mockReset()
    store.showError.mockReset()
    api.getInviteGrowthOverview.mockResolvedValue({
      invited_count: 8,
      qualified_count: 5,
      paid_invitee_count: 6,
      reward_unlocked: '500',
      reward_claimed: '200',
      ranking: [
        { rank: 1, email_masked: 'own***@example.com', qualified_count: 5, reward_amount: 500 },
      ],
    })
    api.listReferralCampaigns.mockResolvedValue(page([campaign]))
    api.getReferralCampaign.mockResolvedValue(detail)
    api.listReferralCampaignParticipants.mockResolvedValue(page([
      { user_id: 11, email: 'owner@example.com', username: 'owner', enrolled_at: '2026-08-01T00:00:00Z', invited_count: 3, qualified_count: 2, reward_unlocked: 100, reward_claimed: 0 },
    ]))
    api.listReferralCampaignInvites.mockResolvedValue(page([]))
    api.listReferralCampaignRewards.mockResolvedValue(page([]))
    api.getReferralCampaignEarlyClosePreview.mockResolvedValue({
      campaign_id: 7,
      campaign_version: 3,
      status: 'settling',
      claim_deadline: '2026-09-10T00:00:00Z',
      claimable_reward_count: 2,
      claimable_reward_amount: 300,
      preserved_reward_count: 1,
      preserved_reward_amount: 200,
      can_early_close: true,
    })
  })

  it('loads campaigns, selects the first draft, and exposes linked participant data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(api.listReferralCampaigns).toHaveBeenCalledWith(expect.objectContaining({ page: 1 }))
    expect(api.getReferralCampaign).toHaveBeenCalledWith(7)
    expect(api.listReferralCampaignParticipants).toHaveBeenCalledWith(7, expect.objectContaining({ page: 1 }))
    expect(wrapper.text()).toContain('八月邀请争霸赛')
    expect(wrapper.text()).toContain('owner@example.com')
    expect(wrapper.text()).toContain('own***@example.com')
  })

  it('shows additive maximum liability and creates a structured draft', async () => {
    api.createReferralCampaign.mockResolvedValue({ ...campaign, id: 8, version: 1 })
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="new-referral-campaign"]').trigger('click')
    await wrapper.get('[data-testid="referral-key"]').setValue('summer-growth')
    await wrapper.get('[data-testid="referral-name"]').setValue('暑期邀请活动')
    await wrapper.get('[data-testid="referral-budget"]').setValue('50000')
    await wrapper.get('[data-testid="referral-capacity"]').setValue('100')
    await wrapper.get('[data-testid="referral-tier-reward-0"]').setValue('100')
    await wrapper.get('[data-testid="referral-add-tier"]').trigger('click')
    await wrapper.get('[data-testid="referral-tier-reward-1"]').setValue('300')

    expect(wrapper.get('[data-testid="referral-liability"]').text()).toContain('40,000')
    await wrapper.get('[data-testid="save-referral-campaign"]').trigger('click')
    await flushPromises()

    expect(api.createReferralCampaign).toHaveBeenCalledWith(expect.objectContaining({
      key: 'summer-growth',
      name: '暑期邀请活动',
      reward_mode: 'additive',
      max_enrollments: 100,
      budget_total: 50000,
      tiers: expect.arrayContaining([
        expect.objectContaining({ tier: 1, reward_amount: 100 }),
        expect.objectContaining({ tier: 2, reward_amount: 300 }),
      ]),
    }))
  })

  it('localizes the new campaign public copy and system defaults', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="new-referral-campaign"]').trigger('click')

    expect(wrapper.text()).toContain('本地化返佣标题')
    expect(wrapper.text()).toContain('本地化返佣说明')
    expect(wrapper.text()).toContain('本地化默认返佣规则')
    expect(wrapper.text()).toContain('本地化叠加返佣规则')
    expect(wrapper.text()).toContain('本地化活动规则标签')
    expect(wrapper.text()).toContain('本地化受邀人说明标签')
    expect((wrapper.get('[data-testid="referral-public-rules"]').element as HTMLTextAreaElement).value).toBe('本地化活动规则默认内容')
    expect((wrapper.get('[data-testid="referral-invitee-notice"]').element as HTMLTextAreaElement).value).toBe('本地化受邀人说明默认内容')
  })

  it('submits status and four-party review operations with optimistic versions', async () => {
    api.setReferralCampaignStatus.mockResolvedValue({ ...campaign, status: 'review', version: 3 })
    api.reviewReferralCampaign.mockResolvedValue({ ...campaign, status: 'review', version: 4 })
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="status-note"]').setValue('提交运营复核')
    await wrapper.get('[data-testid="status-review"]').trigger('click')
    await flushPromises()
    expect(api.setReferralCampaignStatus).toHaveBeenCalledWith(7, {
      expected_version: 2,
      status: 'review',
      note: '提交运营复核',
    })

    api.getReferralCampaign.mockResolvedValue({ ...detail, campaign: { ...campaign, status: 'review', version: 3 } })
    await (wrapper.vm as unknown as { selectCampaign: (id: number) => Promise<void> }).selectCampaign(7)
    await flushPromises()
    await wrapper.get('[data-testid="review-note"]').setValue('运营规则核对通过')
    await wrapper.get('[data-testid="review-approve-ops"]').trigger('click')
    await flushPromises()
    expect(api.reviewReferralCampaign).toHaveBeenCalledWith(7, {
      expected_version: 3,
      review_type: 'ops',
      decision: 'approved',
      note: '运营规则核对通过',
    })
  })

  it('blocks a draft whose budget cannot cover maximum liability', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="new-referral-campaign"]').trigger('click')
    await wrapper.get('[data-testid="referral-key"]').setValue('underfunded-growth')
    await wrapper.get('[data-testid="referral-name"]').setValue('预算不足活动')
    await wrapper.get('[data-testid="referral-budget"]').setValue('5000')
    await wrapper.get('[data-testid="referral-capacity"]').setValue('100')
    await wrapper.get('[data-testid="referral-tier-reward-0"]').setValue('100')
    await wrapper.get('[data-testid="save-referral-campaign"]').trigger('click')
    await flushPromises()

    expect(api.createReferralCampaign).not.toHaveBeenCalled()
    expect(store.showError).toHaveBeenCalledWith(expect.stringContaining('budgetInsufficient'))
  })

  it('loads invite and reward ledgers and resolves debt_review records', async () => {
    api.listReferralCampaignInvites.mockResolvedValue(page([
      { attribution_id: 91, inviter_id: 11, inviter_email: 'owner@example.com', invitee_id: 12, invitee_email: 'new@example.com', registered_at: '2026-08-02T00:00:00Z', status: 'approved', net_paid: 120, actual_cost: 30, risk_status: 'approved', qualification_status: 'qualified' },
    ]))
    api.listReferralCampaignRewards.mockResolvedValue(page([
      { id: 44, campaign_id: 7, user_id: 11, tier: 1, reward_type: 'tier', amount: 100, currency: 'CNY', status: 'debt_review', version: 5, email: 'owner@example.com', username: 'owner' },
    ]))
    api.resolveReferralRewardDebt.mockResolvedValue({ id: 44, status: 'resolved' })
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="detail-tab-invites"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('new@example.com')

    await wrapper.get('[data-testid="detail-tab-rewards"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="resolve-debt-44"]').trigger('click')
    await wrapper.get('[data-testid="debt-note"]').setValue('已核对退款与账户余额')
    await wrapper.get('[data-testid="debt-waive"]').trigger('click')
    await flushPromises()

    expect(api.resolveReferralRewardDebt).toHaveBeenCalledWith(7, 44, {
      expected_version: 5,
      decision: 'waived',
      note: '已核对退款与账户余额',
    })
  })

  it('uses a dedicated early-close preview instead of a normal settling status transition', async () => {
    api.getReferralCampaign.mockResolvedValue({ ...detail, campaign: { ...campaign, status: 'settling', version: 3 } })
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.find('[data-testid="status-closed"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="early-close-campaign"]').text()).toContain('提前结束领奖')
    expect(wrapper.text()).toContain('领奖中')

    await wrapper.get('[data-testid="early-close-campaign"]').trigger('click')
    await flushPromises()
    expect(api.getReferralCampaignEarlyClosePreview).toHaveBeenCalledWith(7)
    expect(wrapper.get('[data-testid="dialog"]').text()).toContain('未领取奖励将失效')
    expect((wrapper.get('[data-testid="confirm-early-close"]').element as HTMLButtonElement).disabled).toBe(true)
  })

  it('does not offer an early-close write after the claim deadline', async () => {
    api.getReferralCampaign.mockResolvedValue({
      ...detail,
      campaign: { ...campaign, status: 'settling', version: 3, claim_deadline: '2020-01-01T00:00:00Z' },
    })
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.find('[data-testid="early-close-campaign"]').exists()).toBe(false)
    expect(api.getReferralCampaignEarlyClosePreview).not.toHaveBeenCalled()
  })

  it('requires an audited reason and explicit confirmation before TOTP-wrapped early close', async () => {
    api.getReferralCampaign.mockResolvedValue({ ...detail, campaign: { ...campaign, status: 'settling', version: 3 } })
    api.earlyCloseReferralCampaign.mockResolvedValue({ ...campaign, status: 'closed', version: 4 })
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="early-close-campaign"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="early-close-reason"]').setValue('  已完成运营核对  ')
    await wrapper.get('[data-testid="early-close-confirmation"]').setValue(true)
    await wrapper.get('[data-testid="confirm-early-close"]').trigger('click')
    await flushPromises()

    expect(stepUp.run).toHaveBeenCalledTimes(1)
    expect(api.earlyCloseReferralCampaign).toHaveBeenCalledWith(7, {
      expected_version: 3,
      reason: '已完成运营核对',
      confirmation: 'EARLY_CLOSE',
    })
    expect(store.showSuccess).toHaveBeenCalled()
  })

  it('does not turn a cancelled TOTP challenge into a failed operation', async () => {
    api.getReferralCampaign.mockResolvedValue({ ...detail, campaign: { ...campaign, status: 'settling', version: 3 } })
    stepUp.run.mockRejectedValueOnce({ code: 'STEP_UP_CANCELLED' })
    const wrapper = mountComponent()
    await flushPromises()

    await wrapper.get('[data-testid="early-close-campaign"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="early-close-reason"]').setValue('运营确认')
    await wrapper.get('[data-testid="early-close-confirmation"]').setValue(true)
    await wrapper.get('[data-testid="confirm-early-close"]').trigger('click')
    await flushPromises()

    expect(api.earlyCloseReferralCampaign).not.toHaveBeenCalled()
    expect(store.showError).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="dialog"]').exists()).toBe(true)
  })
})
