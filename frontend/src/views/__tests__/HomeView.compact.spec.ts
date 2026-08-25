import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { ref } from 'vue'

import HomeView from '../HomeView.vue'

const { appStore, authStore, routeState, router, sanitizeHomeContentMock } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
  routeState: {
    path: '/',
    fullPath: '/',
    query: {},
  },
  router: {
    push: vi.fn(),
  },
  sanitizeHomeContentMock: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => router,
}))

vi.mock('@/api/publicHomeStats', () => ({
  fetchPublicHomeStats: vi.fn().mockResolvedValue(null),
}))

vi.mock('@/utils/homeContent', () => ({
  isHomeContentUrl: (content: string) => /^https?:\/\//.test(content.trim()),
  sanitizeHomeContent: sanitizeHomeContentMock,
}))

vi.mock('@/composables/useHomeLiveStats', () => ({
  useHomeLiveStats: () => ({
    statItems: { value: [] },
    computedAt: { value: '' },
    opsDataThrough: { value: '' },
    isStale: { value: false },
  }),
}))

vi.mock('@/composables/usePublicGrowthTeaser', () => ({
  usePublicGrowthTeaser: () => ({
    perkLines: { value: [] },
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      tm: () => [],
      te: () => false,
      locale: ref('en'),
    }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        'router-link': RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
        PublicPageToolbar: { template: '<div data-testid="public-page-toolbar" />' },
        HomeStatOdometer: true,
        ChannelTV: true,
        HeroSphere: true,
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  const authLink = wrapper.findAllComponents(RouterLinkStub).find((link) => link.classes().includes('compact-home-auth-link'))
  return authLink?.props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    routeState.path = '/'
    routeState.fullPath = '/'
    localStorage.clear()
    sanitizeHomeContentMock.mockReset().mockResolvedValue('')
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  })

  it('renders custom HTML ahead of compact mode', async () => {
    sanitizeHomeContentMock.mockResolvedValue('<section id="custom-home">Custom home</section>')
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })
    await flushPromises()

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('.term').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toEqual({ name: 'Login' })
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toEqual({ name: 'Dashboard' })
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toEqual({ name: 'AdminDashboard' })
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('keeps the English compact documentation action on the internal English docs page', () => {
    routeState.path = '/en'
    routeState.fullPath = '/en'

    const wrapper = mountHome({
      compact_home_enabled: true,
      doc_url: 'https://docs.example.com',
    })
    const docsLink = wrapper.findAllComponents(RouterLinkStub).find((link) => {
      return (link.props('to') as { name?: string }).name === 'EnglishDocs'
    })

    expect(docsLink?.props('to')).toEqual({ name: 'EnglishDocs' })
  })

  it('exposes download and authentication actions in the mobile menu', async () => {
    const wrapper = mountHome()

    await wrapper.get('.mobile-menu-toggle').trigger('click')

    expect(wrapper.find('.mobile-nav-panel').exists()).toBe(true)
    expect(wrapper.find('.mobile-nav-panel').text()).toContain('home.jisudeng.nav.androidApp')
    expect(wrapper.find('.mobile-nav-panel').text()).toContain('home.jisudeng.nav.signIn')
    expect(wrapper.find('.mobile-nav-panel').text()).toContain('home.jisudeng.nav.signUp')
  })

  it('shows the model plaza link to anonymous visitors when public access is enabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link from anonymous visitors when sign-in is required', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('shows the model plaza link to authenticated visitors when sign-in is required', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link when the feature is disabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })
})
