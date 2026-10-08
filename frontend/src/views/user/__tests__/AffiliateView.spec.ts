import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '@/views/user/AffiliateView.vue'

const state = vi.hoisted(() => ({
  getAffiliateDetail: vi.fn(),
  transferAffiliateQuota: vi.fn(),
  getTeamMe: vi.fn(),
  getActiveCampaigns: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  refreshUser: vi.fn(),
  copyToClipboard: vi.fn(),
  listReferralCampaigns: vi.fn(),
  claim: vi.fn(),
  progress: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail: state.getAffiliateDetail,
    transferAffiliateQuota: state.transferAffiliateQuota,
  },
}))

vi.mock('@/api/play', () => ({
  getTeamMe: state.getTeamMe,
  getActiveCampaigns: state.getActiveCampaigns,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: state.showError,
    showSuccess: state.showSuccess,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: state.refreshUser,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: state.copyToClipboard,
  }),
}))

vi.mock('@/utils/format', () => ({
  formatCurrency: (value: number, currency = 'USD') => `${currency} ${value.toFixed(2)}`,
  formatDateTime: (value?: string) => value || '',
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
  extractI18nErrorMessage: (_error: unknown, _t: unknown, _prefix: string, fallback: string) => fallback,
}))

vi.mock('@/api/referralCampaign', () => ({
  claimReferralCampaignReward: state.claim,
  enrollReferralCampaign: vi.fn(),
  getReferralCampaignInviteToken: vi.fn(),
  getReferralCampaignProgress: state.progress,
  listReferralCampaigns: state.listReferralCampaigns,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key}:${JSON.stringify(params)}`
      },
    }),
  }
})

function affiliateFixture(affCode = 'XRFP2MCTF4DS') {
  return {
    user_id: 50,
    aff_code: affCode,
    inviter_id: null,
    aff_count: 18,
    aff_quota: 0,
    aff_frozen_quota: 0,
    aff_history_quota: 0,
    effective_rebate_rate_percent: 20,
    invitees: [],
  }
}

function mountView() {
  return mount(AffiliateView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
      },
    },
  })
}

describe('AffiliateView', () => {
  beforeEach(() => {
    state.getAffiliateDetail.mockReset()
    state.transferAffiliateQuota.mockReset()
    state.getTeamMe.mockReset()
    state.getActiveCampaigns.mockReset()
    state.showError.mockReset()
    state.showSuccess.mockReset()
    state.refreshUser.mockReset()
    state.copyToClipboard.mockReset()
    state.listReferralCampaigns.mockReset()
    state.claim.mockReset()
    state.progress.mockReset()

    state.getAffiliateDetail.mockResolvedValue(affiliateFixture())
    state.getTeamMe.mockResolvedValue({
      enabled: true,
      team: {
        invite_code: '8895EAB6',
      },
    })
    state.getActiveCampaigns.mockResolvedValue([])
    state.listReferralCampaigns.mockResolvedValue([])
  })

  it('includes the current Agent Team invite code in the register invite link', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(state.getTeamMe).toHaveBeenCalled()
    expect(wrapper.text()).toContain(
      `${window.location.origin}/register?ref=XRFP2MCTF4DS&team=8895EAB6`
    )
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'
    state.getAffiliateDetail.mockResolvedValue(affiliateFixture(affiliateCode))
    state.copyToClipboard.mockResolvedValue(true)

    const wrapper = mountView()
    await flushPromises()

    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toEqual(expect.arrayContaining([
        'min-w-0',
        'break-all',
        'sm:flex-1',
        'sm:truncate',
      ]))
      expect(Array.from(value.element.parentElement?.classList ?? [])).toEqual(expect.arrayContaining([
        'flex-col',
        'items-stretch',
        'sm:flex-row',
        'sm:items-center',
      ]))
    }

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toEqual(expect.arrayContaining([
        'w-full',
        'sm:w-auto',
        'sm:shrink-0',
      ]))
    }

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(state.copyToClipboard).toHaveBeenNthCalledWith(1, affiliateCode, 'affiliate.codeCopied')
    expect(state.copyToClipboard).toHaveBeenNthCalledWith(
      2,
      `${window.location.origin}/register?ref=${encodeURIComponent(affiliateCode)}&team=8895EAB6`,
      'affiliate.linkCopied',
    )
  })

	it('shows registered versus qualified conversion and an empty leaderboard state', async () => {
		state.listReferralCampaigns.mockResolvedValue([{ campaign: { id: 7, name: 'August', status: 'running', version: 3, pay_threshold: 50, usage_threshold: 20, claim_deadline: '2026-09-01' }, enrollment: { campaign_id: 7, user_id: 50, enrolled_at: '2026-08-01' }, tiers: [{ tier: 1, required_invites: 3, reward_amount: 100, currency: 'CNY' }], rewards: [], invited_count: 5, qualified_count: 2, leaderboard: [] }])

		const wrapper = mountView()
		await flushPromises()

		expect(wrapper.text()).toContain('affiliate.campaign.invitedBreakdown:{"invited":5,"qualified":2}')
		expect(wrapper.text()).toContain('affiliate.campaign.leaderboardEmpty')
	})

  it('explains why non-recharge credits do not count toward growth rewards', async () => {
    state.getActiveCampaigns.mockResolvedValue([{
      id: 9,
      name: 'New user growth',
      end_at: '2026-09-01',
      rules: { qualification_metric: 'actual_consumption', reward_tiers: [] },
      new_user_growth: { eligible: true, funding_conflict: true, qualified_amount: 0, rewards: [] },
    }])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('affiliate.growth.fundingConflict')
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
  })
  it('shows invitee qualification and the actual reward currency without fabricated personal progress', async () => {
    state.listReferralCampaigns.mockResolvedValue([{ campaign: { id: 7, name: 'August', status: 'running', version: 3, pay_threshold: 50, usage_threshold: 20, reward_mode: 'additive', risk_hold_hours: 48, claim_deadline: '2026-09-01' }, enrollment: { campaign_id: 7, user_id: 50 }, tiers: [{ tier: 1, required_invites: 3, reward_amount: 100, currency: 'CNY' }], rewards: [], invited_count: 5, qualified_count: 2, leaderboard: [] }])
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('CNY 100.00')
    expect(wrapper.text()).toContain('affiliate.campaign.details.rewardModes.additive')
    expect(wrapper.text()).not.toContain('affiliate.campaign.details.current')
    expect(wrapper.text()).not.toContain('USD 150.00')
  })

  it('shows a failed campaign load as an error rather than an empty campaign list', async () => {
    state.listReferralCampaigns.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('affiliate.campaign.loadFailed')
    expect(wrapper.text()).not.toContain('affiliate.campaign.empty')
  })

  it('prevents another campaign claim while a claim is pending and recovers after failure', async () => {
    const campaign = { campaign: { id: 7, name: 'August', status: 'running', version: 3, pay_threshold: 50, usage_threshold: 20, reward_mode: 'additive', risk_hold_hours: 48, claim_deadline: '2026-09-01' }, enrollment: { campaign_id: 7, user_id: 50 }, tiers: [{ tier: 1, required_invites: 3, reward_amount: 100, currency: 'CNY' }, { tier: 2, required_invites: 5, reward_amount: 200, currency: 'CNY' }], rewards: [{ id: 11, tier: 1, status: 'claimable' }, { id: 12, tier: 2, status: 'claimable' }], invited_count: 5, qualified_count: 5, leaderboard: [] }
    state.listReferralCampaigns.mockResolvedValue([campaign])
    let reject!: (reason: Error) => void
    state.claim.mockImplementationOnce(() => new Promise((_resolve, rejectPromise) => { reject = rejectPromise }))
    const wrapper = mountView()
    await flushPromises()
    const claims = wrapper.findAll('button').filter(button => button.text() === 'affiliate.campaign.claim')
    await claims[0].trigger('click')
    expect(claims.every(button => button.attributes('disabled') !== undefined)).toBe(true)
    await claims[1].trigger('click')
    expect(state.claim).toHaveBeenCalledTimes(1)
    reject(new Error('offline'))
    await flushPromises()
    expect(claims.every(button => button.attributes('disabled') === undefined)).toBe(true)
    expect(state.showError).toHaveBeenCalledWith('affiliate.campaign.claimFailed')
  })

})
