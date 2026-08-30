import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const getSubscriptionsProgress = vi.fn()
const getMySubscriptions = vi.fn()
const showError = vi.fn()
const push = vi.fn()

vi.mock('@/api/subscriptions', () => ({
  default: {
    getSubscriptionsProgress: (...args: unknown[]) => getSubscriptionsProgress(...args),
    getMySubscriptions: (...args: unknown[]) => getMySubscriptions(...args),
  },
  createSubscriptionProgressFallback: (subscription: { id: number; expires_at: string | null; group?: { name?: string } }) => ({
    id: subscription.id,
    groupName: subscription.group?.name ?? '',
    expiresAt: subscription.expires_at,
    expiresInDays: null,
    daily: null,
    weekly: null,
    monthly: null,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    showError,
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: ref('zh'),
      t: (key: string) => ({
        'userSubscriptions.status.active': '有效',
        'userSubscriptions.daily': '每日额度',
      }[key] ?? key),
    }),
  }
})

describe('SubscriptionsView progress contract', () => {
  beforeEach(() => {
    getSubscriptionsProgress.mockReset()
    getMySubscriptions.mockReset()
    showError.mockReset()
    push.mockReset()
    getMySubscriptions.mockResolvedValue([
      {
        id: 42,
        user_id: 9,
        group_id: 17,
        status: 'active',
        starts_at: '2026-08-01T00:00:00Z',
        daily_usage_usd: 999,
        weekly_usage_usd: 999,
        monthly_usage_usd: 999,
        daily_window_start: null,
        weekly_window_start: null,
        monthly_window_start: null,
        created_at: '2026-08-01T00:00:00Z',
        updated_at: '2026-08-01T00:00:00Z',
        expires_at: null,
      },
      {
        id: 43,
        user_id: 9,
        group_id: 18,
        status: 'expired',
        starts_at: '2026-07-01T00:00:00Z',
        daily_usage_usd: 888,
        weekly_usage_usd: 888,
        monthly_usage_usd: 888,
        daily_window_start: null,
        weekly_window_start: null,
        monthly_window_start: null,
        created_at: '2026-07-01T00:00:00Z',
        updated_at: '2026-08-01T00:00:00Z',
        expires_at: '2026-08-15T00:00:00Z',
      },
    ])
    getSubscriptionsProgress.mockResolvedValue([
      {
        subscription: {
          id: 42,
          user_id: 9,
          group_id: 17,
          status: 'active',
          starts_at: '2026-08-01T00:00:00Z',
          daily_usage_usd: 999,
          weekly_usage_usd: 999,
          monthly_usage_usd: 999,
          daily_window_start: null,
          weekly_window_start: null,
          monthly_window_start: null,
          created_at: '2026-08-01T00:00:00Z',
          updated_at: '2026-08-01T00:00:00Z',
          expires_at: null,
        },
        progress: {
          id: 42,
          groupName: '',
          expiresAt: null,
          expiresInDays: null,
          daily: {
            limitUsd: 10,
            usedUsd: 2.5,
            remainingUsd: 7.5,
            percentage: 25,
            windowStart: '2026-08-30T00:00:00Z',
            resetsAt: '2026-08-31T00:00:00Z',
            resetsInSeconds: 3600,
          },
          weekly: null,
          monthly: null,
        },
      },
    ])
  })

  it('renders normalized active progress without hiding expired subscriptions', async () => {
    const { default: SubscriptionsView } = await import('../SubscriptionsView.vue')
    const wrapper = mount(SubscriptionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(getMySubscriptions).toHaveBeenCalledOnce()
    expect(getSubscriptionsProgress).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('$2.50 / $10.00')
    expect(wrapper.text()).not.toContain('$999.00')
    expect(wrapper.text()).not.toContain('$888.00')
    expect(wrapper.text()).toContain('userSubscriptions.status.expired')
    expect(showError).not.toHaveBeenCalled()
  })
})
