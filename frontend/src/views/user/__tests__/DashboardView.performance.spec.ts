import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardView from '../DashboardView.vue'

const { refreshUser, getDashboardStats, getDashboardTrend, getDashboardModels, getByDateRange, getMyPlatformQuotas } = vi.hoisted(() => ({
  refreshUser: vi.fn(),
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  getMyPlatformQuotas: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 10 },
    isSimpleMode: true,
    refreshUser,
  }),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats,
    getDashboardTrend,
    getDashboardModels,
    getByDateRange,
  },
}))

vi.mock('@/api/user', () => ({ getMyPlatformQuotas }))

function pendingPromise() {
  return new Promise<void>(() => {})
}

describe('user Dashboard startup requests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getDashboardStats.mockResolvedValue({ today_requests: 7 })
    getDashboardTrend.mockResolvedValue({ trend: [] })
    getDashboardModels.mockResolvedValue({ models: [] })
    getByDateRange.mockResolvedValue({ items: [] })
    getMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
  })

  it('shows statistics without waiting for the user refresh', async () => {
    refreshUser.mockReturnValue(pendingPromise())

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          LoadingSpinner: { template: '<span data-test="loading" />' },
          UserDashboardStats: { props: ['stats'], template: '<div data-test="stats">{{ stats.today_requests }}</div>' },
          UserDashboardCharts: true,
          UserDashboardRecentUsage: true,
          UserDashboardQuickActions: true,
        },
      },
    })
    await flushPromises()

    expect(refreshUser).toHaveBeenCalledTimes(1)
    expect(getDashboardStats).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="stats"]').text()).toBe('7')
  })

  it('keeps statistics available when the profile refresh fails', async () => {
    refreshUser.mockRejectedValue(new Error('profile failed'))
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          LoadingSpinner: true,
          UserDashboardStats: { props: ['stats'], template: '<div data-test="stats">{{ stats.today_requests }}</div>' },
          UserDashboardCharts: true,
          UserDashboardRecentUsage: true,
          UserDashboardQuickActions: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-test="stats"]').text()).toBe('7')
    expect(consoleError).toHaveBeenCalledWith('Failed to refresh dashboard user:', expect.any(Error))
  })
})
