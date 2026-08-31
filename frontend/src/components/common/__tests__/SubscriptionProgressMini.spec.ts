import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useSubscriptionStore } from '@/stores/subscriptions'
import type { SubscriptionProgressEntry, UserSubscription } from '@/types'

let activeLocale: 'zh' | 'en' = 'zh'

const translations = {
  zh: {
    'subscriptionProgress.title': '我的订阅',
    'subscriptionProgress.viewDetails': '查看订阅详情',
    'subscriptionProgress.activeCount': '{count} 个有效订阅',
    'subscriptionProgress.groupFallback': '分组 #{id}',
    'subscriptionProgress.unlimited': '无限制',
    'subscriptionProgress.viewAll': '查看全部订阅',
  },
  en: {
    'subscriptionProgress.title': 'My Subscriptions',
    'subscriptionProgress.viewDetails': 'View subscription details',
    'subscriptionProgress.activeCount': '{count} active subscription(s)',
    'subscriptionProgress.groupFallback': 'Group #{id}',
    'subscriptionProgress.unlimited': 'Unlimited',
    'subscriptionProgress.viewAll': 'View all subscriptions',
  },
} as const

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: keyof typeof translations.zh, params?: Record<string, unknown>) =>
        translations[activeLocale][key]?.replace('{id}', String(params?.id ?? ''))
          .replace('{count}', String(params?.count ?? '')) ?? key,
    }),
  }
})

const getSubscriptionsProgress = vi.fn()

vi.mock('@/api/subscriptions', () => ({
  default: {
    getSubscriptionsProgress: (...args: unknown[]) => getSubscriptionsProgress(...args)
  }
}))

const subscription = {
  id: 1,
  user_id: 3,
  group_id: 17,
  status: 'active',
  starts_at: '2026-08-01T00:00:00Z',
  daily_usage_usd: 0,
  weekly_usage_usd: 0,
  monthly_usage_usd: 0,
  daily_window_start: null,
  weekly_window_start: null,
  monthly_window_start: null,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
  expires_at: null,
} satisfies UserSubscription

const progressEntry = {
  subscription,
  progress: {
    id: 1,
    groupName: '',
    expiresAt: null,
    expiresInDays: null,
    daily: {
      limitUsd: 10,
      usedUsd: 2.5,
      remainingUsd: 7.5,
      percentage: 25,
      windowStart: null,
      resetsAt: null,
      resetsInSeconds: null,
    },
    weekly: null,
    monthly: null,
  }
} satisfies SubscriptionProgressEntry

describe('SubscriptionProgressMini', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    activeLocale = 'zh'
    getSubscriptionsProgress.mockReset()
    getSubscriptionsProgress.mockResolvedValue([progressEntry])
    pinia = createPinia()
    setActivePinia(pinia)
  })

  it.each([
    ['zh', '分组 #17'],
    ['en', 'Group #17'],
  ] as const)('localizes a missing group name in %s', async (locale, expected) => {
    activeLocale = locale
    const { default: SubscriptionProgressMini } = await import('../SubscriptionProgressMini.vue')
    const wrapper = mount(SubscriptionProgressMini, {
      global: {
        plugins: [pinia],
        stubs: { Icon: true, 'router-link': true },
      },
    })

    await flushPromises()
    await wrapper.find('button').trigger('click')
    expect(wrapper.text()).toContain(expected)
    expect(wrapper.text()).toContain('$2.50/$10.00')
    expect(getSubscriptionsProgress).toHaveBeenCalledOnce()
    expect(wrapper.text()).not.toContain('Group #17' === expected ? '分组 #17' : 'Group #17')

    wrapper.unmount()
  })

  it('reacts when the shared progress store refreshes after mount', async () => {
    const { default: SubscriptionProgressMini } = await import('../SubscriptionProgressMini.vue')
    const wrapper = mount(SubscriptionProgressMini, {
      global: {
        plugins: [pinia],
        stubs: { Icon: true, 'router-link': true },
      },
    })

    await flushPromises()
    const store = useSubscriptionStore(pinia)
    expect(wrapper.text()).toContain('1')
    expect(wrapper.text()).not.toContain('$999.00')

    getSubscriptionsProgress.mockResolvedValue([
      {
        ...progressEntry,
        progress: {
          ...progressEntry.progress,
          daily: {
            ...progressEntry.progress.daily,
            usedUsd: 4,
            remainingUsd: 6,
            percentage: 40,
          },
        },
      },
    ])
    await store.fetchSubscriptionProgress(true)
    await flushPromises()
    await wrapper.find('button').trigger('click')

    expect(wrapper.text()).toContain('$4.00/$10.00')
    expect(getSubscriptionsProgress).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
