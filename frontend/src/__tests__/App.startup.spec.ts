import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from '@/App.vue'

const {
  routeState,
  routerReplaceMock,
  routerAfterEachMock,
  getSetupStatusMock,
  fetchPublicSettingsMock,
  fetchActiveSubscriptionsMock,
  fetchAnnouncementsMock,
  fetchAdminComplianceStatusMock,
  requireAdminComplianceMock,
  resetAdminComplianceMock,
  resetAnnouncementsMock,
  clearSubscriptionsMock,
  updateFaviconMock,
} = vi.hoisted(() => ({
  routeState: {
    path: '/login',
    fullPath: '/login',
    meta: {},
  },
  routerReplaceMock: vi.fn(),
  routerAfterEachMock: vi.fn(),
  getSetupStatusMock: vi.fn(),
  fetchPublicSettingsMock: vi.fn(),
  fetchActiveSubscriptionsMock: vi.fn(),
  fetchAnnouncementsMock: vi.fn(),
  fetchAdminComplianceStatusMock: vi.fn(),
  requireAdminComplianceMock: vi.fn(),
  resetAdminComplianceMock: vi.fn(),
  resetAnnouncementsMock: vi.fn(),
  clearSubscriptionsMock: vi.fn(),
  updateFaviconMock: vi.fn(),
}))

const appStore = {
  publicSettingsLoaded: true,
  cachedPublicSettings: null as null | { custom_menu_items?: unknown[] },
  siteName: '极速蹬',
  siteLogo: '',
  fetchPublicSettings: fetchPublicSettingsMock,
}

const authStore = {
  isAuthenticated: false,
  isAdmin: false,
}

const subscriptionStore = {
  fetchActiveSubscriptions: fetchActiveSubscriptionsMock,
  startPolling: vi.fn(),
  clear: clearSubscriptionsMock,
}

const announcementStore = {
  currentPopup: null,
  fetchAnnouncements: fetchAnnouncementsMock,
  reset: resetAnnouncementsMock,
}

const adminComplianceStore = {
  initialized: false,
  shouldShow: false,
  fetchStatus: fetchAdminComplianceStatusMock,
  requireAcknowledgement: requireAdminComplianceMock,
  reset: resetAdminComplianceMock,
}

const adminSettingsStore = {
  customMenuItems: [],
}

vi.mock('vue-router', () => ({
  RouterView: { template: '<main data-test="router-view" />' },
  useRouter: () => ({
    replace: routerReplaceMock,
    afterEach: routerAfterEachMock,
  }),
  useRoute: () => routeState,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useSubscriptionStore: () => subscriptionStore,
  useAnnouncementStore: () => announcementStore,
  useAdminComplianceStore: () => adminComplianceStore,
  useAdminSettingsStore: () => adminSettingsStore,
}))

vi.mock('@/api/setup', () => ({
  getSetupStatus: getSetupStatusMock,
}))

vi.mock('@/router/title', () => ({
  resolveRouteDocumentTitle: () => '极速蹬',
}))

vi.mock('@/utils/branding', () => ({
  updateFavicon: updateFaviconMock,
}))

vi.mock('@/components/common/Toast.vue', () => ({
  default: { template: '<div data-test="toast" />' },
}))

vi.mock('@/components/common/NavigationProgress.vue', () => ({
  default: { template: '<div data-test="navigation-progress" />' },
}))

vi.mock('@/components/admin/AdminComplianceDialog.vue', () => ({
  default: { template: '<div data-test="admin-compliance-dialog" />' },
}))

vi.mock('@/components/common/AnnouncementPopup.vue', () => ({
  default: { template: '<div data-test="announcement-popup" />' },
}))

describe('App startup performance gates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeState.path = '/login'
    routeState.fullPath = '/login'
    routeState.meta = {}
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = null
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    getSetupStatusMock.mockResolvedValue({ needs_setup: false, step: 'done' })
    fetchPublicSettingsMock.mockResolvedValue(null)
    fetchActiveSubscriptionsMock.mockResolvedValue(null)
    fetchAnnouncementsMock.mockResolvedValue(null)
    fetchAdminComplianceStatusMock.mockResolvedValue(null)
    window.__APP_CONFIG__ = { site_name: '极速蹬' } as typeof window.__APP_CONFIG__
  })

  it('does not probe setup status or refetch public settings when HTML injected settings are already applied', async () => {
    mount(App)
    await flushPromises()

    expect(getSetupStatusMock).not.toHaveBeenCalled()
    expect(fetchPublicSettingsMock).not.toHaveBeenCalled()
    expect(routerReplaceMock).not.toHaveBeenCalled()
  })

  it('keeps the static fallback setup probe and redirects when setup is required', async () => {
    appStore.publicSettingsLoaded = false
    delete window.__APP_CONFIG__
    getSetupStatusMock.mockResolvedValue({ needs_setup: true, step: 'database' })

    mount(App)
    await flushPromises()

    expect(getSetupStatusMock).toHaveBeenCalledOnce()
    expect(routerReplaceMock).toHaveBeenCalledWith('/setup')
    expect(fetchPublicSettingsMock).not.toHaveBeenCalled()
  })

  it('fetches public settings once after a normal static fallback setup probe', async () => {
    appStore.publicSettingsLoaded = false
    delete window.__APP_CONFIG__
    getSetupStatusMock.mockResolvedValue({ needs_setup: false, step: 'done' })

    mount(App)
    await flushPromises()

    expect(getSetupStatusMock).toHaveBeenCalledOnce()
    expect(fetchPublicSettingsMock).toHaveBeenCalledOnce()
    expect(routerReplaceMock).not.toHaveBeenCalled()
  })
})
