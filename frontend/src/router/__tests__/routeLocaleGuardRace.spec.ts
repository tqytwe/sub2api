import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>,
) => Promise<void>

const routerHarness = vi.hoisted(() => ({
  guard: null as NavigationGuard | null,
}))

const localeHarness = vi.hoisted(() => ({
  deferredRouteName: '' as string,
  deferred: null as ReturnType<typeof createDeferred<void>> | null,
}))

const authHarness = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const adminSettingsHarness = vi.hoisted(() => ({
  loaded: true,
  customMenuItems: [] as Array<{ id: string; url: string; label: string }>,
  fetch: vi.fn(),
}))

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => {
      routerHarness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
  })),
}))

vi.mock('@/i18n', () => ({
  applyLocaleFromRoute: vi.fn().mockResolvedValue(true),
  ensureLocaleMessagesForRoute: vi.fn((routeName: string) => {
    if (routeName === localeHarness.deferredRouteName) {
      return localeHarness.deferred?.promise ?? Promise.resolve()
    }
    return Promise.resolve()
  }),
  getLocale: vi.fn(() => 'zh'),
  inheritedEnglishLocaleQuery: vi.fn(() => null),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authHarness,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    siteName: '极速蹬',
    backendModeEnabled: false,
    publicSettingsLoaded: true,
    cachedPublicSettings: null,
    fetchPublicSettings: vi.fn(),
  }),
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => adminSettingsHarness,
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

function runGuard(path: string, name: string, params: Record<string, string> = {}) {
  if (!routerHarness.guard) {
    throw new Error('router guard was not registered')
  }

  const next = vi.fn()
  const navigation = routerHarness.guard(
    {
      path,
      fullPath: path,
      name,
      params,
      query: {},
      meta: { requiresAuth: false },
    },
    { path: '/', query: {} },
    next,
  )
  return { navigation, next }
}

function canonicalHref(): string | null {
  return document.querySelector('link[rel="canonical"]')?.getAttribute('href') ?? null
}

describe('route locale guard latest-navigation ownership', () => {
  beforeAll(async () => {
    await import('@/router')
  })

  beforeEach(() => {
    document.head.replaceChildren()
    document.title = 'initial'
    document.documentElement.setAttribute('lang', 'zh-CN')
    localeHarness.deferred = createDeferred<void>()
    localeHarness.deferredRouteName = ''
    authHarness.isAuthenticated = false
    authHarness.isAdmin = false
    adminSettingsHarness.loaded = true
    adminSettingsHarness.customMenuItems = []
    adminSettingsHarness.fetch.mockReset()
  })

  it.each([
    {
      older: { path: '/docs', name: 'Docs' },
      newer: { path: '/en/docs', name: 'EnglishDocs' },
      expected: { lang: 'en', title: 'Jisudeng API Docs: OpenAI-Compatible Gateway, Models, Images', canonical: 'https://www.jisudeng.com/en/docs' },
    },
    {
      older: { path: '/en/docs', name: 'EnglishDocs' },
      newer: { path: '/docs', name: 'Docs' },
      expected: { lang: 'zh-CN', title: '极速蹬 API 文档 - OpenAI兼容接口、模型调用与计费指南', canonical: 'https://www.jisudeng.com/docs' },
    },
  ])('keeps the newer $newer.path document metadata after $older.path lazy locale work resolves', async ({ older, newer, expected }) => {
    localeHarness.deferredRouteName = older.name
    const olderGuard = runGuard(older.path, older.name)

    await vi.waitFor(() => {
      expect(localeHarness.deferred).not.toBeNull()
    })

    const newerGuard = runGuard(newer.path, newer.name)
    await newerGuard.navigation

    localeHarness.deferred?.resolve()
    await olderGuard.navigation

    expect(newerGuard.next).toHaveBeenCalledWith()
    expect(olderGuard.next).toHaveBeenCalledWith(false)
    expect(document.documentElement.lang).toBe(expected.lang)
    expect(document.title).toBe(expected.title)
    expect(canonicalHref()).toBe(expected.canonical)
    expect(document.querySelector('link[rel="alternate"][hreflang="en"]')).not.toBeNull()
    expect(document.querySelector('link[rel="alternate"][hreflang="zh-CN"]')).not.toBeNull()
  })

  it('waits for admin settings before resolving a direct admin docs custom page', async () => {
    const settingsDeferred = createDeferred<void>()
    authHarness.isAuthenticated = true
    authHarness.isAdmin = true
    adminSettingsHarness.loaded = false
    adminSettingsHarness.fetch.mockImplementation(() => settingsDeferred.promise)

    const navigation = runGuard('/custom/admin-docs', 'CustomPage', { id: 'admin-docs' })

    await vi.waitFor(() => {
      expect(adminSettingsHarness.fetch).toHaveBeenCalledTimes(1)
    })
    expect(navigation.next).not.toHaveBeenCalled()

    adminSettingsHarness.customMenuItems = [{
      id: 'admin-docs',
      url: 'https://www.jisudeng.com/docs',
      label: 'Admin docs',
    }]
    adminSettingsHarness.loaded = true
    settingsDeferred.resolve()
    await navigation.navigation

    expect(navigation.next).toHaveBeenCalledWith('/docs')
  })
})
