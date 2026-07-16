import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { jisudengPagesEn } from '@/i18n/locales/jisudeng-pages.en'
import { jisudengPagesZh } from '@/i18n/locales/jisudeng-pages.zh'
import { useAuthStore } from '@/stores/auth'
import BlindboxView from '@/views/public/BlindboxView.vue'

const {
  getBlindboxPoolMock,
  getBlindboxStatusMock,
  getBlindboxRecentWinsMock,
  openBlindboxMock,
  refreshUserMock,
  showErrorMock,
  showInfoMock,
  showSuccessMock,
} = vi.hoisted(() => ({
  getBlindboxPoolMock: vi.fn(),
  getBlindboxStatusMock: vi.fn(),
  getBlindboxRecentWinsMock: vi.fn(),
  openBlindboxMock: vi.fn(),
  refreshUserMock: vi.fn(),
  showErrorMock: vi.fn(),
  showInfoMock: vi.fn(),
  showSuccessMock: vi.fn(),
}))

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({
    isAuthenticated: false,
    refreshUser: vi.fn(),
  })
  return {
    useAuthStore: () => state,
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showInfo: showInfoMock,
    showSuccess: showSuccessMock,
  }),
}))

vi.mock('@/api/play', () => ({
  default: {
    getBlindboxPool: (...args: unknown[]) => getBlindboxPoolMock(...args),
    getBlindboxStatus: (...args: unknown[]) => getBlindboxStatusMock(...args),
    getBlindboxRecentWins: (...args: unknown[]) => getBlindboxRecentWinsMock(...args),
    openBlindbox: (...args: unknown[]) => openBlindboxMock(...args),
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const configuredPool = {
  version: 'season-1-v1',
  cost: 0.5,
  rtp_cap: 0.9,
  tiers: [
    { amount: 0.05, weight: 4000 },
    { amount: 0.2, weight: 3000 },
    { amount: 0.5, weight: 1800 },
    { amount: 1, weight: 800 },
    { amount: 3, weight: 300 },
    { amount: 10, weight: 90 },
    { amount: 20, weight: 10 },
  ],
}

const authState = useAuthStore() as unknown as {
  isAuthenticated: boolean
  refreshUser: typeof refreshUserMock
}

enableAutoUnmount(afterEach)

function configuredStatus() {
  return {
    enabled: true,
    cost_amount: 0.5,
    pool: configuredPool,
    daily_limit: 3,
    effective_limit: 3,
    opens_today: 0,
    can_open: true,
    server_date: '2026-07-16',
  }
}

function mountView() {
  return mount(BlindboxView, {
    global: {
      stubs: {
        PublicPageToolbar: true,
        PublicPlayBackLink: true,
        SupportFloatingCard: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('BlindboxView', () => {
  beforeEach(() => {
    authState.isAuthenticated = false
    refreshUserMock.mockReset()
    authState.refreshUser = refreshUserMock
    getBlindboxPoolMock.mockReset()
    getBlindboxStatusMock.mockReset()
    getBlindboxRecentWinsMock.mockReset()
    openBlindboxMock.mockReset()
    showErrorMock.mockReset()
    showInfoMock.mockReset()
    showSuccessMock.mockReset()
    getBlindboxRecentWinsMock.mockResolvedValue([])
  })

  it('loads the public pool for guests and renders all seven API tiers', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      enabled: true,
      pool: configuredPool,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(getBlindboxPoolMock).toHaveBeenCalledTimes(1)
    expect(getBlindboxStatusMock).not.toHaveBeenCalled()

    const tiers = wrapper.findAll('.play-prize-tier')
    expect(tiers).toHaveLength(7)
    expect(wrapper.text()).toContain('$20.00')
    expect(wrapper.text()).toContain('0.1%')
    expect(wrapper.text()).not.toContain('$2.00')
  })

  it('loads authenticated status and renders the pool returned with it', async () => {
    authState.isAuthenticated = true
    getBlindboxStatusMock.mockResolvedValue(configuredStatus())

    const wrapper = mountView()
    await flushPromises()

    expect(getBlindboxStatusMock).toHaveBeenCalledTimes(1)
    expect(getBlindboxPoolMock).not.toHaveBeenCalled()
    expect(wrapper.findAll('.play-prize-tier')).toHaveLength(7)
    expect(wrapper.text()).toContain('$20.00')
  })

  it('reloads the authenticated status when a guest signs in without a page refresh', async () => {
    getBlindboxPoolMock.mockResolvedValue({ enabled: true, pool: configuredPool })
    getBlindboxStatusMock.mockResolvedValue(configuredStatus())
    const wrapper = mountView()
    await flushPromises()

    authState.isAuthenticated = true
    await flushPromises()

    expect(getBlindboxStatusMock).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('.play-prize-tier')).toHaveLength(7)
    expect(wrapper.find('.play-btn-primary').attributes('disabled')).toBeUndefined()
  })

  it('does not render a valid pool when the public feature is disabled', async () => {
    getBlindboxPoolMock.mockResolvedValue({ enabled: false, pool: configuredPool })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('.play-prize-tier')).toHaveLength(0)
    expect(wrapper.text()).toContain('blindbox.disabled')
    expect(wrapper.find('.play-actions a').exists()).toBe(false)
  })

  it('disables opening and shows unavailable when status has no valid pool', async () => {
    authState.isAuthenticated = true
    getBlindboxStatusMock.mockResolvedValue({
      enabled: true,
      cost_amount: 0.5,
      pool: {
        version: '',
        cost: 0,
        rtp_cap: 0,
        tiers: [],
      },
      daily_limit: 3,
      opens_today: 0,
      can_open: true,
      server_date: '2026-07-16',
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('blindbox.unavailable')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('.play-prize-tier')).toHaveLength(0)
  })

  it('handles top-level API error codes and refreshes status after the daily limit', async () => {
    authState.isAuthenticated = true
    getBlindboxStatusMock.mockResolvedValue(configuredStatus())
    openBlindboxMock.mockRejectedValue({ code: 'PLAY_BLINDBOX_DAILY_LIMIT' })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.play-btn-primary').trigger('click')
    await flushPromises()

    expect(showInfoMock).toHaveBeenCalledWith('blindbox.dailyLimit')
    expect(showErrorMock).not.toHaveBeenCalled()
    expect(getBlindboxStatusMock).toHaveBeenCalledTimes(2)
  })

  it('loads the public pool when an authenticated user signs out', async () => {
    authState.isAuthenticated = true
    getBlindboxStatusMock.mockResolvedValue(configuredStatus())
    getBlindboxPoolMock.mockResolvedValue({ enabled: true, pool: configuredPool })
    const wrapper = mountView()
    await flushPromises()

    authState.isAuthenticated = false
    await flushPromises()

    expect(getBlindboxPoolMock).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('.play-prize-tier')).toHaveLength(7)
  })

  it('ignores a stale guest pool response after the user signs in', async () => {
    let resolveGuestPool!: (value: { enabled: boolean; pool: typeof configuredPool }) => void
    getBlindboxPoolMock.mockReturnValue(new Promise((resolve) => {
      resolveGuestPool = resolve
    }))
    getBlindboxStatusMock.mockResolvedValue(configuredStatus())
    const wrapper = mountView()
    await Promise.resolve()

    authState.isAuthenticated = true
    await flushPromises()
    resolveGuestPool({ enabled: false, pool: configuredPool })
    await flushPromises()

    expect(wrapper.findAll('.play-prize-tier')).toHaveLength(7)
    expect(wrapper.text()).not.toContain('blindbox.disabled')
  })

  it('preserves configured sub-cent prize precision', async () => {
    getBlindboxPoolMock.mockResolvedValue({
      enabled: true,
      pool: {
        ...configuredPool,
        tiers: [{ amount: 0.005, weight: 10_000 }],
      },
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('$0.005')
    expect(wrapper.text()).not.toContain('$0.01')
  })

  it('shows unavailable instead of disabled when authenticated status loading fails', async () => {
    authState.isAuthenticated = true
    getBlindboxStatusMock.mockRejectedValue(new Error('network unavailable'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('blindbox.unavailable')
    expect(wrapper.text()).not.toContain('blindbox.disabled')
  })

  it('distinguishes a recent-wins request failure from a verified empty feed', async () => {
    getBlindboxPoolMock.mockResolvedValue({ enabled: true, pool: configuredPool })
    getBlindboxRecentWinsMock.mockRejectedValue(new Error('recent wins unavailable'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('blindbox.recentWinsUnavailable')
    expect(wrapper.text()).not.toContain('blindbox.recentWinsPlaceholder')
  })

  it('does not claim that VIP tiers can upgrade the prize pool', () => {
    expect(jisudengPagesZh.blindbox.prizePoolNote).not.toMatch(/VIP|升级奖池/)
    expect(jisudengPagesEn.blindbox.prizePoolNote).not.toMatch(/VIP|upgrade/i)
    expect(jisudengPagesZh.playHub.vipPerks.blindbox_pool_upgrade).toContain('暂未启用')
    expect(jisudengPagesEn.playHub.vipPerks.blindbox_pool_upgrade).toMatch(/not active/i)
    expect(jisudengPagesZh.docs.vipTiers.perks.blindbox_pool_upgrade).toContain('暂未启用')
    expect(jisudengPagesEn.docs.vipTiers.perks.blindbox_pool_upgrade).toMatch(/not active/i)
  })
})
