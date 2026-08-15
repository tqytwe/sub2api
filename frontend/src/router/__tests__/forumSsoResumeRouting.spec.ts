import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

import { FORUM_SSO_RESUME_PARAM } from '@/composables/useForumSsoResume'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>
) => Promise<void>

const routerHarness = vi.hoisted(() => ({
  guard: null as NavigationGuard | null,
}))

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: true,
  cachedPublicSettings: null as null | Record<string, unknown>,
  fetchPublicSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => {
      routerHarness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
    // /login is a Chinese-locale public path, so the guard's locale step runs
    // and reads the current route back off the router when setting the title.
    currentRoute: { value: { path: '/login', query: {}, params: {}, meta: {} } },
  })),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({
    initialized: true,
    fetchStatus: vi.fn(),
    requireAcknowledgement: vi.fn(),
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

async function runGuard(path: string, query: Record<string, string> = {}) {
  if (!routerHarness.guard) {
    throw new Error('router guard was not registered')
  }

  const next = vi.fn()
  await routerHarness.guard(
    {
      path,
      fullPath: path,
      name: 'GuardRoute',
      params: {},
      query,
      meta: { requiresAuth: false },
    },
    {},
    next
  )
  return next
}

describe('forum SSO resume route guard', () => {
  beforeAll(async () => {
    await import('@/router')
  })

  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    appStore.backendModeEnabled = false
  })

  it('lets an authenticated user reach /login when a forum handoff is pending', async () => {
    const next = await runGuard('/login', {
      [FORUM_SSO_RESUME_PARAM]:
        '/api/v1/sso/oauth/authorize?client_id=sub2api_forum&redirect_uri=https%3A%2F%2Fforum.example%2Fcb',
    })

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('lets an authenticated administrator reach /login for the same handoff', async () => {
    authStore.isAdmin = true

    const next = await runGuard('/login', {
      [FORUM_SSO_RESUME_PARAM]: '?client_id=sub2api_forum&redirect_uri=https%3A%2F%2Fforum.example%2Fcb',
    })

    expect(next).toHaveBeenCalledWith()
  })

  it('still redirects an authenticated user away from /login without the parameter', async () => {
    const next = await runGuard('/login')

    expect(next).toHaveBeenCalledWith('/dashboard')
  })

  it('still redirects an authenticated administrator away from /login without the parameter', async () => {
    authStore.isAdmin = true

    const next = await runGuard('/login')

    expect(next).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('does not extend the exemption to /register', async () => {
    const next = await runGuard('/register', {
      [FORUM_SSO_RESUME_PARAM]: '?client_id=sub2api_forum&redirect_uri=https%3A%2F%2Fforum.example%2Fcb',
    })

    expect(next).toHaveBeenCalledWith('/dashboard')
  })

  it('does not redirect a guest away from /login carrying a pending handoff', async () => {
    authStore.isAuthenticated = false

    const next = await runGuard('/login', {
      [FORUM_SSO_RESUME_PARAM]: '?client_id=sub2api_forum&redirect_uri=https%3A%2F%2Fforum.example%2Fcb',
    })

    expect(next).toHaveBeenCalledWith()
  })
})
