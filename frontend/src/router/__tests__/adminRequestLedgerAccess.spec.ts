import { describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isSimpleMode: false,
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ backendModeEnabled: false, cachedPublicSettings: null }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ loaded: true, customMenuItems: [] }) }))
vi.mock('@/stores/adminCompliance', () => ({ useAdminComplianceStore: () => ({ initialized: true }) }))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn(), isLoading: { value: false } }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn(), cancelPendingPrefetch: vi.fn(), resetPrefetchState: vi.fn() }),
}))
vi.mock('@/api', () => ({ authAPI: { getCurrentUser: vi.fn(), logout: vi.fn() }, isTotp2FARequired: () => false }))
vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn() }))
vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))

describe('request ledger access contract', () => {
  it('exposes the request ledger only through the administrator route', async () => {
    const { default: router } = await import('@/router')
    const routes = router.getRoutes()

    expect(routes.find((route) => route.name === 'RequestLedger')).toBeUndefined()
    const adminRoute = routes.find((route) => route.name === 'AdminRequestLedger')
    expect(adminRoute?.path).toBe('/admin/requests')
    expect(adminRoute?.meta.requiresAdmin).toBe(true)
  })

  it('exposes quality operations only through the administrator route', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((item) => item.name === 'AdminAccountQuality')

    expect(route?.path).toBe('/admin/account-quality')
    expect(route?.meta.requiresAdmin).toBe(true)
  })
})
