import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '@/views/user/AffiliateView.vue'
enableAutoUnmount(afterEach)

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
  summary: vi.fn(),
  invitees: vi.fn(),
  earnings: vi.fn(),
  leaderboard: vi.fn(),
  history: vi.fn(),
}))

vi.mock('@/api/affiliateOperations', () => ({
  getAffiliateOperationsSummary: state.summary,
  listAffiliateInvitees: state.invitees,
  listAffiliateEarnings: state.earnings,
  getAffiliateLeaderboard: state.leaderboard,
  listAffiliateCampaignHistory: state.history,
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
  markReferralCampaignViewed: vi.fn(),
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
        Pagination: { props: ['total', 'page', 'pageSize'], template: '<button data-test="next-page" @click="$emit(\'update:page\', page + 1)">Next {{ total }}</button>' },
        RouterLink: { props: ['to'], template: '<a :data-to="JSON.stringify(to)"><slot /></a>' },
      },
    },
  })
}

async function openTab(wrapper: ReturnType<typeof mountView>, name: string) {
  await wrapper.get(`[data-test="affiliate-tab-${name}"]`).trigger('click')
  await flushPromises()
}

describe('AffiliateView', () => {
  afterEach(() => { vi.useRealTimers() })
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
    state.summary.mockReset().mockResolvedValue({ month: '2026-11', month_net_earned: 12, currency: 'USD', amounts: [], period_amounts: [], invited_count: 120, effective_paid_count: 8, pending_count: 2, available_quota: 5, frozen_quota: 3, gross_earned: 40, revoked_amount: 2, net_earned: 38, transferred_amount: 30, withdrawn_amount: 0, updated_at: '2026-11-08T10:00:00Z' })
    state.invitees.mockReset().mockResolvedValue({ items: [], total: 120, page: 1, page_size: 20 })
    state.earnings.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    state.leaderboard.mockReset().mockResolvedValue({ month: '2026-11', timezone: 'Asia/Shanghai', items: [], my_rank: null, updated_at: '2026-11-08T10:00:00Z' })
    state.history.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  })

  it('keeps sharing and transfer in overview with five local tabs and loads invitations using server pagination', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(5)
    expect(state.invitees).not.toHaveBeenCalled()
    expect(state.leaderboard).not.toHaveBeenCalled()
    await openTab(wrapper, 'invitees')
    expect(state.invitees).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(state.invitees).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
    expect(wrapper.findAll('code')).toHaveLength(0)
  })

  it('shows top ten separately from my rank and does not reload activities during leaderboard refresh', async () => {
    state.leaderboard.mockResolvedValue({ month: '2026-11', timezone: 'Asia/Shanghai', items: Array.from({ length: 10 }, (_, i) => ({ rank: i + 1, display_name: `masked-${i}`, effective_paid_count: 20 - i, is_me: false })), my_rank: { rank: 26, display_name: 'masked-me', effective_paid_count: 2, is_me: true }, updated_at: '2026-11-08T10:00:00Z' })
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'leaderboard')
    expect(wrapper.findAll('[data-test="leaderboard-row"]')).toHaveLength(10)
    expect(wrapper.get('[data-test="my-rank"]').text()).toContain('#26')
    const campaignCalls = state.listReferralCampaigns.mock.calls.length
    await wrapper.get('[data-test="leaderboard-refresh"]').trigger('click')
    await flushPromises()
    expect(state.listReferralCampaigns).toHaveBeenCalledTimes(campaignCalls)
    expect(wrapper.text()).toContain('affiliateOperations.honorOnly')
    wrapper.unmount()
  })

  it('rolls the live monthly selection across Shanghai midnight without refreshing activity details', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-30T15:59:58Z'))
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'leaderboard')
    expect(state.leaderboard).toHaveBeenLastCalledWith({ month: '2026-11' }, expect.any(AbortSignal))
    const activityCalls = state.listReferralCampaigns.mock.calls.length
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(state.leaderboard).toHaveBeenLastCalledWith({ month: '2026-12' }, expect.any(AbortSignal))
    expect((wrapper.get('input[type="month"]').element as HTMLInputElement).value).toBe('2026-12')
    expect(state.listReferralCampaigns).toHaveBeenCalledTimes(activityCalls)
    wrapper.unmount()
  })

  it('keeps an explicitly selected historical month when the current Shanghai month changes', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-30T15:59:58Z'))
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'leaderboard')
    await wrapper.get('input[type="month"]').setValue('2026-10')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(state.leaderboard).toHaveBeenLastCalledWith({ month: '2026-10' }, expect.any(AbortSignal))
    expect(wrapper.get('input[type="month"]').attributes('max')).toBe('2026-12')
    wrapper.unmount()
  })

  it('keeps failed invitations distinct from an empty result and supports retry', async () => {
    state.invitees.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'invitees')
    expect(wrapper.get('[role="alert"]').text()).toContain('affiliateOperations.loadFailed')
    expect(wrapper.text()).not.toContain('affiliateOperations.inviteesEmpty')
    await wrapper.get('[data-test="invitees-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('affiliateOperations.inviteesEmpty')
  })

  it('loads ended activity history through server pagination and retains USD and CNY reward amounts', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'campaigns')
    await wrapper.get('[data-test="campaign-scope"]').setValue('ended')
    await flushPromises()
    expect(state.history).toHaveBeenCalledWith(expect.objectContaining({ scope: 'ended', page: 1 }))
  })

  it('links affiliate earnings to the existing wallet instead of introducing another balance history', async () => {
    state.earnings.mockResolvedValue({ items: [{ id: 4, action: 'transfer', source_type: 'affiliate', amount: -3, status: 'completed', currency: 'USD', created_at: '2026-11-08T10:00:00Z' }], total: 1, page: 1, page_size: 20 })
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'earnings')
    expect(wrapper.text()).toContain('USD -3.00')
    expect(wrapper.get('[data-test="affiliate-wallet-link"]').attributes('data-to')).toContain('affiliate')
  })

  it('does not combine USD and CNY earnings or display a mixed-currency aggregate as zero', async () => {
    const base = await state.summary()
    state.summary.mockResolvedValue({ ...base, currency: 'mixed', net_earned: null, month_net_earned: null, amounts: [{ source_type: 'regular', currency: 'USD', gross_earned: 15, revoked_amount: 0, net_earned: 15 }, { source_type: 'campaign', currency: 'CNY', gross_earned: 100, revoked_amount: 0, net_earned: 100 }], period_amounts: [] })
    const wrapper = mountView(); await flushPromises()
    expect(wrapper.text()).toContain('affiliateOperations.byCurrency')
    expect(wrapper.text()).toContain('USD 15.00')
    expect(wrapper.text()).toContain('CNY 100.00')
    expect(wrapper.text()).not.toContain('USD 115.00')
  })

  it('prevents concurrent growth-reward claims and refreshes summary after a successful claim', async () => {
    state.getActiveCampaigns.mockResolvedValue([{ id: 9, name: 'Growth', end_at: '2026-12-01', rules: { reward_tiers: [{ tier: 1, required_amount: 1, reward_amount: 1 }, { tier: 2, required_amount: 2, reward_amount: 2 }] }, new_user_growth: { referral_campaign_id: 9, referral_version: 1, qualified_amount: 3, rewards: [{ tier: 1, reward_id: 11, status: 'claimable' }, { tier: 2, reward_id: 12, status: 'claimable' }] } }])
    let resolve!: () => void
    state.claim.mockImplementationOnce(() => new Promise<void>(done => { resolve = done }))
    const wrapper = mountView(); await flushPromises(); await openTab(wrapper, 'campaigns')
    const buttons = wrapper.findAll('button').filter(button => button.text() === 'affiliate.growth.claim')
    await buttons[0].trigger('click')
    expect(buttons.every(button => button.attributes('disabled') !== undefined)).toBe(true)
    await buttons[1].trigger('click')
    expect(state.claim).toHaveBeenCalledTimes(1)
    resolve(); await flushPromises()
    expect(state.summary.mock.calls.length).toBeGreaterThan(1)
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

	it('shows registered versus qualified conversion and a link to the unified activity leaderboard', async () => {
		state.listReferralCampaigns.mockResolvedValue([{ campaign: { id: 7, name: 'August', status: 'running', version: 3, pay_threshold: 50, usage_threshold: 20, claim_deadline: '2026-09-01' }, enrollment: { campaign_id: 7, user_id: 50, enrolled_at: '2026-08-01' }, tiers: [{ tier: 1, required_invites: 3, reward_amount: 100, currency: 'CNY' }], rewards: [], invited_count: 5, qualified_count: 2, leaderboard: [] }])

		const wrapper = mountView()
		await flushPromises()
		await openTab(wrapper, 'campaigns')

		expect(wrapper.text()).toContain('affiliate.campaign.invitedBreakdown:{"invited":5,"qualified":2}')
		expect(wrapper.text()).toContain('affiliateOperations.viewActivityLeaderboard')
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

    await openTab(wrapper, 'campaigns')

    expect(wrapper.text()).toContain('affiliate.growth.fundingConflict')
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
  })
  it('shows invitee qualification and the actual reward currency without fabricated personal progress', async () => {
    state.listReferralCampaigns.mockResolvedValue([{ campaign: { id: 7, name: 'August', status: 'running', version: 3, pay_threshold: 50, usage_threshold: 20, reward_mode: 'additive', risk_hold_hours: 48, claim_deadline: '2026-09-01' }, enrollment: { campaign_id: 7, user_id: 50 }, tiers: [{ tier: 1, required_invites: 3, reward_amount: 100, currency: 'CNY' }], rewards: [], invited_count: 5, qualified_count: 2, leaderboard: [] }])
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'campaigns')
    expect(wrapper.text()).toContain('CNY 100.00')
    expect(wrapper.text()).toContain('affiliate.campaign.details.rewardModes.additive')
    expect(wrapper.text()).not.toContain('affiliate.campaign.details.current')
    expect(wrapper.text()).not.toContain('USD 150.00')
  })

  it('shows a failed campaign load as an error rather than an empty campaign list', async () => {
    state.listReferralCampaigns.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountView()
    await flushPromises()
    await openTab(wrapper, 'campaigns')
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
    await openTab(wrapper, 'campaigns')
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
