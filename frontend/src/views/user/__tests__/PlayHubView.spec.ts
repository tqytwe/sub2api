import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PlayHubSummary } from '@/api/play'
import PlayHubView from '@/views/user/PlayHubView.vue'

const state = vi.hoisted(() => ({
  getPlayHub: vi.fn(),
  listReferralCampaigns: vi.fn(),
  refreshUser: vi.fn(),
  push: vi.fn(),
	locale: 'zh',
}))

vi.mock('@/api/play', () => ({
  default: {
    getPlayHub: state.getPlayHub,
  },
}))

vi.mock('@/api/referralCampaign', () => ({
  listReferralCampaigns: state.listReferralCampaigns,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 9.5 },
    refreshUser: state.refreshUser,
  }),
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: {
    playCheckin: 'playCheckin',
    playArena: 'playArena',
    playBlindbox: 'playBlindbox',
    playQuiz: 'playQuiz',
    playAgentTeam: 'playAgentTeam',
    affiliate: 'affiliate',
  },
  isFeatureFlagEnabled: () => true,
}))

vi.mock('@/utils/growthAnalytics', () => ({
  trackGrowthEvent: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: state.push }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
		locale: ref(state.locale),
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key}:${JSON.stringify(params)}`
      },
    }),
  }
})

function hubFixture(): PlayHubSummary {
  return {
    any_enabled: true,
    pending_actions: 4,
    growth: {
      balance: 42.3,
      total_recharged: 120,
      first_recharge_eligible: false,
      balance_low_warning: false,
      recharge_multiplier: 1,
      payment_enabled: true,
      vip: {
        tier: 1,
        label: 'V1',
        perks: ['priority_support', 'image_bonus'],
        next_tier: 2,
        next_label: 'V2',
        amount_to_next: 80,
      },
    },
    campaigns: [
      {
        id: 1,
        name: 'Summer Boost',
        start_at: '2026-07-01T00:00:00Z',
        end_at: '2026-07-31T23:59:59Z',
        rules: {
          recharge_bonus_pct: 20,
          blindbox_extra_opens: 1,
          name_i18n: { zh: '夏日加速', en: 'Summer Boost' },
        },
      },
    ],
    image_studio: {
      enabled: true,
      images_today: 2,
      has_completed_job: false,
    },
    quests: {
      enabled: true,
      energy: 60,
      level: 3,
      energy_to_next_level: 40,
      server_date: '2026-07-17',
      tasks: [
        { key: 'checkin', completed: true, energy: 10 },
        { key: 'image_studio', completed: false, energy: 30, cta_route: '/ai-creation-space' },
      ],
    },
    checkin: {
      enabled: true,
      checked_in_today: false,
      reward_amount: 1.5,
      server_date: '2026-07-17',
      streak_count: 6,
    },
    arena: {
      enabled: true,
      token_sum: 12345,
      rank: 8,
      tokens_to_prev_rank: 600,
    },
    blindbox: {
      enabled: true,
      cost_amount: 1,
      daily_limit: 3,
      opens_today: 1,
      can_open: true,
      server_date: '2026-07-17',
    },
    quiz: {
      enabled: true,
      questions: Array.from({ length: 5 }, (_, index) => ({
        id: index + 1,
        prompt: `Question ${index + 1}`,
        options: [],
      })),
      already_submitted: false,
      reward_per_correct: 0.1,
      server_date: '2026-07-17',
    },
    team: {
      enabled: true,
      team: {
        id: 1,
        name: 'Alpha',
        invite_code: 'ALPHA',
        captain_id: 1,
        member_count: 5,
        token_sum: 56000,
        members: [],
        current_month: '2026-07',
        team_spend: '120.00',
        reached_threshold: '100.00',
        reward_rate: '0.05',
        next_threshold: '500.00',
        estimated_pool: '6.00',
        reward_cap: '50.00',
        reward_tiers: [],
      },
    },
  }
}

function mountView() {
  return mount(PlayHubView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      },
    },
  })
}

describe('PlayHubView layout', () => {
  beforeEach(() => {
	state.locale = 'zh'
    state.getPlayHub.mockResolvedValue(hubFixture())
    state.listReferralCampaigns.mockResolvedValue([])
    state.refreshUser.mockResolvedValue(undefined)
    state.push.mockReset()
  })

  it('renders the hub on a full-width console workspace', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(state.getPlayHub).toHaveBeenCalledTimes(1)
    expect(state.refreshUser).not.toHaveBeenCalled()

    const shell = wrapper.get('[data-testid="play-hub-shell"]')
    expect(shell.classes()).toContain('w-full')
    expect(shell.classes()).toContain('max-w-none')
    expect(shell.classes()).not.toContain('max-w-[1440px]')
    expect(shell.classes()).not.toContain('gw-page--wide')
    expect(wrapper.find('.gw-page--wide').exists()).toBe(false)

    const entryGrid = wrapper.get('[data-testid="play-hub-entry-grid"]')
    expect(entryGrid.classes()).toContain('2xl:grid-cols-4')
  })

  it('keeps the overview, summaries, and play entries visible from hub data', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('$42.30')
    expect(wrapper.text()).toContain('playHub.pending')
    expect(wrapper.text()).toContain('V1')
    expect(wrapper.text()).toContain('夏日加速')
    expect(wrapper.text()).toContain('playHub.questsEnergy')
    expect(wrapper.text()).toContain('nav.aiCreationSpace')
    expect(wrapper.text()).toContain('nav.checkIn')
    expect(wrapper.text()).toContain('nav.blindbox')
    expect(wrapper.text()).toContain('nav.agentTeam')
  })

  it('shows the full quiz balance reward for the completed set, not the per-correct base', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('playHub.quizPending:{"reward":"0.50"}')
    expect(wrapper.text()).not.toContain('playHub.quizPending:{"reward":"0.10"}')
  })

  it('does not promise redeemable check-in or quiz rewards to explorer accounts', async () => {
    const explorerHub = hubFixture()
    explorerHub.checkin = {
      ...explorerHub.checkin!,
      growth_energy_enabled: true,
      redeemable_reward_eligible: false,
      growth_eligibility: {
        tier: 'explorer',
        reward_mode: 'energy',
        primary_reason: 'no_recent_activity',
        email_verified: true,
        account_age_days: 8,
        has_recent_usage: false,
        net_balance_recharge_30d: 0,
        has_active_subscription: false,
        progress: {
          email_verified: true, account_age_days: 8, minimum_account_age_days: 3,
          account_age_requirement_met: true, has_recent_usage: false,
          net_balance_recharge_30d: 0, minimum_recharge_cny: 10,
          has_active_subscription: false, activity_requirement_met: false,
          next_action: 'no_recent_activity',
        },
      },
    }
    explorerHub.quiz = {
      ...explorerHub.quiz!,
      growth_eligibility: {
        tier: 'explorer',
        reward_mode: 'energy',
        primary_reason: 'no_recent_activity',
        email_verified: true,
        account_age_days: 8,
        has_recent_usage: false,
        net_balance_recharge_30d: 0,
        has_active_subscription: false,
        progress: {
          email_verified: true, account_age_days: 8, minimum_account_age_days: 3,
          account_age_requirement_met: true, has_recent_usage: false,
          net_balance_recharge_30d: 0, minimum_recharge_cny: 10,
          has_active_subscription: false, activity_requirement_met: false,
          next_action: 'no_recent_activity',
        },
      },
    }
    state.getPlayHub.mockResolvedValueOnce(explorerHub)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('checkin.energyHint')
    expect(wrapper.text()).toContain('quiz.energyHint')
    expect(wrapper.text()).not.toContain('playHub.checkinPending')
    expect(wrapper.text()).not.toContain('playHub.quizPending')
  })

	 it('uses localized operational display content and a named CTA route while preserving the server order', async () => {
		const operationalHub = hubFixture()
		operationalHub.campaigns = [
			{
				id: 9,
				name: 'VIP upgrade week',
				start_at: '2026-07-20T00:00:00Z',
				end_at: '2026-07-27T00:00:00Z',
				rules: {
					campaign_type: 'operational_display',
					display_title_i18n: { zh: '普通用户限时福利', en: 'Ordinary user offer' },
					display_body_i18n: { zh: '充值即可解锁会员权益。', en: 'Recharge to unlock membership benefits.' },
					display_cta: 'recharge',
					display_priority: 120,
				},
			},
			...operationalHub.campaigns,
		]
		state.getPlayHub.mockResolvedValueOnce(operationalHub)

		const wrapper = mountView()
		await flushPromises()
		expect(wrapper.text()).toContain('普通用户限时福利')
		expect(wrapper.text()).toContain('充值即可解锁会员权益。')
		expect(wrapper.text()).toContain('夏日加速')

		const campaignCard = wrapper.findAll('section').find((section) => section.text().includes('普通用户限时福利'))
		const cta = campaignCard?.find('button')
		expect(cta).toBeDefined()
		await cta?.trigger('click')
		expect(state.push).toHaveBeenCalledWith({ name: 'PurchaseSubscription' })

		state.locale = 'en'
		state.getPlayHub.mockResolvedValueOnce(operationalHub)
		const englishWrapper = mountView()
		await flushPromises()
		expect(englishWrapper.text()).toContain('Ordinary user offer')
		expect(englishWrapper.text()).toContain('Recharge to unlock membership benefits.')
	})
})
