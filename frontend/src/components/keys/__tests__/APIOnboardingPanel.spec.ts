import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import APIOnboardingPanel from '../APIOnboardingPanel.vue'
import type { APIOnboardingConfig, Group } from '@/types'
import type { SubscriptionPlan } from '@/types/payment'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'keys.apiOnboarding.balanceHint': 'Current balance {balance}; recommended minimum is {required}.',
    'keys.apiOnboarding.buyPlan': 'Buy Plan',
    'keys.apiOnboarding.createKey': 'Create Key',
    'keys.apiOnboarding.defaultBuyPlanDescription': 'Plans unlock subscription groups.',
    'keys.apiOnboarding.defaultBuyPlanTitle': 'Subscribe to a plan',
    'keys.apiOnboarding.defaultCreateDescription': 'Choose an available group and create a key.',
    'keys.apiOnboarding.defaultCreateTitle': 'Create a stable key',
    'keys.apiOnboarding.defaultDocsDescription': 'Review API base URL and model calls.',
    'keys.apiOnboarding.defaultDocsTitle': 'Read setup docs',
    'keys.apiOnboarding.defaultRechargeDescription': 'Top up first when balance is low.',
    'keys.apiOnboarding.defaultRechargeTitle': 'Top up balance first',
    'keys.apiOnboarding.openDocs': 'Open Docs',
    'keys.apiOnboarding.recharge': 'Top Up',
    'keys.apiOnboarding.recommendedGroup': 'Recommended group',
    'keys.apiOnboarding.subtitle': 'Choose a group, top up, or subscribe before creating your first API key.',
    'keys.apiOnboarding.title': 'Recommended setup',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let message = messages[key] ?? key
        if (params) {
          Object.entries(params).forEach(([paramKey, value]) => {
            message = message.replace(`{${paramKey}}`, String(value))
          })
        }
        return message
      },
    }),
  }
})

function groupFixture(id: number, name = `Group ${id}`): Group {
  return {
    id,
    name,
    description: null,
    platform: 'anthropic',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'token',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    allow_image_generation: false,
    allow_batch_image_generation: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    batch_image_discount_multiplier: 1,
    batch_image_hold_multiplier: 1,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    video_price_480p: null,
    video_price_720p: null,
    video_price_1080p: null,
    web_search_price_per_call: null,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '2026-07-23T00:00:00Z',
    updated_at: '2026-07-23T00:00:00Z',
  }
}

function planFixture(id: number, groupId = id): SubscriptionPlan {
  return {
    id,
    group_id: groupId,
    name: `Plan ${id}`,
    description: '',
    price: 19,
    currency: 'USD',
    validity_days: 30,
    validity_unit: 'days',
    features: [],
    for_sale: true,
    sort_order: id,
  }
}

const config: APIOnboardingConfig = {
  enabled: true,
  title: 'Start with the right API path',
  subtitle: 'Admin maintained recommendations.',
  items: [
    {
      id: 'create',
      title: 'Claude Stable Key',
      description: 'Recommended first key.',
      badge: 'Starter',
      enabled: true,
      sort_order: 1,
      group_id: 12,
      plan_id: null,
      min_balance: 5,
      cta: 'create_key',
      audience: 'new_users',
    },
    {
      id: 'invalid-group',
      title: 'Hidden invalid group',
      description: '',
      badge: '',
      enabled: true,
      sort_order: 2,
      group_id: 404,
      plan_id: null,
      min_balance: 0,
      cta: 'create_key',
      audience: 'new_users',
    },
    {
      id: 'plan',
      title: 'Buy Pro Plan',
      description: '',
      badge: '',
      enabled: true,
      sort_order: 3,
      group_id: null,
      plan_id: 7,
      min_balance: 0,
      cta: 'buy_plan',
      audience: 'new_users',
    },
    {
      id: 'docs',
      title: 'Read Docs',
      description: '',
      badge: '',
      enabled: true,
      sort_order: 4,
      group_id: null,
      plan_id: null,
      min_balance: 0,
      cta: 'open_docs',
      audience: 'new_users',
    },
  ],
}

function mountPanel(overrides: Partial<InstanceType<typeof APIOnboardingPanel>['$props']> = {}) {
  return mount(APIOnboardingPanel, {
    props: {
      config,
      groups: [groupFixture(12, 'Claude Stable')],
      plans: [planFixture(7, 88)],
      balance: 0,
      docUrl: '/docs',
      isNewUser: true,
      ...overrides,
    },
    global: {
      stubs: {
        Icon: true,
        GroupBadge: {
          props: ['name'],
          template: '<span data-test="group-badge">{{ name }}</span>',
        },
      },
    },
  })
}

describe('APIOnboardingPanel', () => {
  it('renders only usable recommendations and emits the expected actions', async () => {
    const wrapper = mountPanel()

    expect(wrapper.text()).toContain('Start with the right API path')
    expect(wrapper.text()).toContain('Claude Stable Key')
    expect(wrapper.text()).toContain('Buy Pro Plan')
    expect(wrapper.text()).toContain('Read Docs')
    expect(wrapper.text()).not.toContain('Hidden invalid group')
    expect(wrapper.text()).toContain('Current balance 0.00; recommended minimum is 5.00.')

    await wrapper.findAll('button').find(button => button.text().includes('Create Key'))!.trigger('click')
    expect(wrapper.emitted('createKey')?.[0]).toEqual([12])

    await wrapper.findAll('button').find(button => button.text().includes('Buy Plan'))!.trigger('click')
    expect(wrapper.emitted('buyPlan')?.[0]).toEqual([expect.objectContaining({ id: 7, group_id: 88 })])

    await wrapper.findAll('button').find(button => button.text().includes('Open Docs'))!.trigger('click')
    expect(wrapper.emitted('openDocs')).toHaveLength(1)
  })

  it('hides docs and plan cards when the referenced resources are unavailable', () => {
    const wrapper = mountPanel({
      plans: [],
      docUrl: '',
    })

    expect(wrapper.text()).toContain('Claude Stable Key')
    expect(wrapper.text()).not.toContain('Buy Pro Plan')
    expect(wrapper.text()).not.toContain('Read Docs')
  })
})
