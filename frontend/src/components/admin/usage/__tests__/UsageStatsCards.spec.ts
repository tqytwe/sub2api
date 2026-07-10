import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UsageStatsCards from '../UsageStatsCards.vue'

const messages: Record<string, string> = {
  'usage.totalRequests': 'Total Requests',
  'usage.inSelectedRange': 'in selected range',
  'usage.totalTokens': 'Total Tokens',
  'usage.in': 'In',
  'usage.out': 'Out',
  'usage.cacheTotal': 'Cache',
  'usage.cacheBreakdown': 'Cache Token Breakdown',
  'usage.cacheCreationTokensLabel': 'Cache Creation',
  'usage.cacheReadTokensLabel': 'Cache Read',
  'usage.totalCost': 'Total Cost',
  'usage.accountCost': 'Cost',
  'usage.standardCost': 'Standard',
  'usage.avgDuration': 'Avg Duration',
  'usage.savedThisMonth': 'Saved ${amount} this month',
  'usage.savedVsOfficial': 'Saved ${amount} vs official',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        const template = messages[key] ?? key
        if (!params) return template
        return Object.entries(params).reduce(
          (text, [name, value]) => text.replace(`\${${name}}`, value),
          template,
        )
      },
    }),
  }
})

const stats = {
  total_requests: 1,
  total_input_tokens: 100,
  total_output_tokens: 50,
  total_cache_tokens: 34,
  total_cache_creation_tokens: 12,
  total_cache_read_tokens: 22,
  total_tokens: 184,
  total_cost: 0.001,
  total_actual_cost: 0.001,
  total_account_cost: 0.001,
  average_duration_ms: 250,
}

describe('UsageStatsCards', () => {
  it('shows cache token breakdown values', () => {
    const wrapper = mount(UsageStatsCards, {
      props: {
        stats,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('Cache: 34')
    expect(text).toContain('Cache Token Breakdown')
    expect(text).toContain('Cache Creation')
    expect(text).toContain('12')
    expect(text).toContain('Cache Read')
    expect(text).toContain('22')
  })

  it('shows monthly savings when strikeStandardCost and savingsLabel is month', () => {
    const wrapper = mount(UsageStatsCards, {
      props: {
        stats: {
          ...stats,
          total_cost: 10,
          total_actual_cost: 7.5,
        },
        strikeStandardCost: true,
        savingsLabel: 'month',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Saved 2.50 this month')
  })

  it('hides savings when standard cost does not exceed actual cost', () => {
    const wrapper = mount(UsageStatsCards, {
      props: {
        stats: {
          ...stats,
          total_cost: 5,
          total_actual_cost: 5,
        },
        strikeStandardCost: true,
        savingsLabel: 'month',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).not.toContain('Saved')
  })

  it('hides savings when savingsLabel is null', () => {
    const wrapper = mount(UsageStatsCards, {
      props: {
        stats: {
          ...stats,
          total_cost: 10,
          total_actual_cost: 7.5,
        },
        strikeStandardCost: true,
        savingsLabel: null,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).not.toContain('Saved')
  })
})
