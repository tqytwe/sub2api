import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PlayHubSummary } from '@/api/play'
import PlayHubView from '@/views/user/PlayHubView.vue'

const state = vi.hoisted(() => ({
  getPlayHub: vi.fn(),
  refreshUser: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/api/play', () => ({
  default: {
    getPlayHub: state.getPlayHub,
  },
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
      locale: ref('zh'),
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
        { key: 'image_studio', completed: false, energy: 30, cta_route: '/image-studio' },
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
      questions: [],
      already_submitted: false,
      reward_per_correct: 0.25,
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
    state.getPlayHub.mockResolvedValue(hubFixture())
    state.refreshUser.mockResolvedValue(undefined)
    state.push.mockReset()
  })

  it('renders the hub on a full-width console canvas', async () => {
    const wrapper = mountView()
    await flushPromises()

    const shell = wrapper.get('[data-testid="play-hub-shell"]')
    expect(shell.classes()).toContain('max-w-[1440px]')
    expect(shell.classes()).not.toContain('gw-page--wide')
    expect(wrapper.find('.gw-page--wide').exists()).toBe(false)
  })

  it('keeps the overview, summaries, and play entries visible from hub data', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('$42.30')
    expect(wrapper.text()).toContain('playHub.pending')
    expect(wrapper.text()).toContain('V1')
    expect(wrapper.text()).toContain('夏日加速')
    expect(wrapper.text()).toContain('playHub.questsEnergy')
    expect(wrapper.text()).toContain('nav.imageStudio')
    expect(wrapper.text()).toContain('nav.checkIn')
    expect(wrapper.text()).toContain('nav.blindbox')
    expect(wrapper.text()).toContain('nav.agentTeam')
  })
})
