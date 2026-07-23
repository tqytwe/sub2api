import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from '@/views/HomeView.vue'

const {
  appState,
  authState,
  routeState,
  sanitizeHomeContentMock,
  recoverFromChunkLoadErrorMock,
  fetchPublicSettingsMock,
} = vi.hoisted(() => ({
  appState: {
    cachedPublicSettings: {
      site_name: '极速蹬',
      site_logo: '',
      doc_url: '',
      home_content: '',
    },
    siteName: '极速蹬',
    siteLogo: '',
    docUrl: '',
    supportContact: { enabled: false, contacts: [] },
    publicSettingsLoaded: true,
  },
  authState: {
    isAuthenticated: false,
    isAdmin: false,
  },
  routeState: {
    fullPath: '/',
    path: '/',
  },
  sanitizeHomeContentMock: vi.fn(),
  recoverFromChunkLoadErrorMock: vi.fn(),
  fetchPublicSettingsMock: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    ...appState,
    fetchPublicSettings: fetchPublicSettingsMock,
  }),
  useAuthStore: () => authState,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    push: vi.fn(),
  }),
  RouterLink: {
    props: ['to'],
    template: '<a><slot /></a>',
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const copy: Record<string, string> = {
    'home.jisudeng.hero.titleParts.brand': 'Jisudeng',
    'home.jisudeng.hero.titleParts.mid': 'One API',
    'home.jisudeng.hero.titleParts.tail': 'for AI models',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => copy[key] ?? key,
      tm: () => [],
      te: () => true,
      locale: { value: 'zh' },
    }),
  }
})

vi.mock('@/utils/homeContent', () => ({
  isHomeContentUrl: (content: string) => /^https?:\/\//.test(content.trim()),
  sanitizeHomeContent: sanitizeHomeContentMock,
}))

vi.mock('@/router/chunkRecovery', () => ({
  recoverFromChunkLoadError: recoverFromChunkLoadErrorMock,
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

vi.mock('@/components/home/HeroSphere.vue', () => ({
  default: { template: '<div data-test="hero-sphere" />' },
}))

vi.mock('@/components/home/ChannelTV.vue', () => ({
  default: { template: '<div data-test="channel-tv" />' },
}))

vi.mock('@/components/home/TerminalDemo.vue', () => ({
  default: { template: '<div data-test="terminal-demo" />' },
}))

vi.mock('@/components/home/WhyHoverCard.vue', () => ({
  default: { template: '<div data-test="why-hover-card" />' },
}))

vi.mock('@/components/home/LmspeedBadge.vue', () => ({
  default: { template: '<span data-test="lmspeed-badge" />' },
}))

vi.mock('@/components/common/PublicPageToolbar.vue', () => ({
  default: { template: '<div data-test="public-page-toolbar" />' },
}))

vi.mock('@/components/home/HomeStatOdometer.vue', () => ({
  default: { template: '<span data-test="home-stat-odometer" />' },
}))

describe('HomeView startup chunk behavior', () => {
  function mountHomeView() {
    return mount(HomeView, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a><slot /></a>',
          },
          'router-link': {
            props: ['to'],
            template: '<a><slot /></a>',
          },
        },
      },
    })
  }

  beforeEach(() => {
    vi.clearAllMocks()
    appState.cachedPublicSettings.home_content = ''
    appState.publicSettingsLoaded = true
    authState.isAuthenticated = false
    authState.isAdmin = false
    routeState.fullPath = '/'
    routeState.path = '/'
    sanitizeHomeContentMock.mockResolvedValue('')
    recoverFromChunkLoadErrorMock.mockReturnValue(false)
    fetchPublicSettingsMock.mockResolvedValue(null)
    Object.defineProperty(window, 'scrollTo', {
      configurable: true,
      value: vi.fn(),
    })
  })

  it('renders sanitized custom inline home HTML after the on-demand sanitizer resolves', async () => {
    appState.cachedPublicSettings.home_content = '<img src=x onerror="alert(1)"><strong>欢迎</strong>'
    sanitizeHomeContentMock.mockResolvedValue('<img src="x"><strong>欢迎</strong>')

    const wrapper = mountHomeView()
    await flushPromises()

    expect(sanitizeHomeContentMock).toHaveBeenCalledWith(appState.cachedPublicSettings.home_content)
    expect(wrapper.find('.custom-home-page').exists()).toBe(true)
    expect(wrapper.html()).toContain('<strong>欢迎</strong>')
    expect(wrapper.html()).not.toContain('onerror')
    expect(fetchPublicSettingsMock).not.toHaveBeenCalled()
  })

  it('uses chunk recovery when the on-demand sanitizer chunk cannot be loaded', async () => {
    const error = new Error('Failed to fetch dynamically imported module')
    appState.cachedPublicSettings.home_content = '<strong>欢迎</strong>'
    sanitizeHomeContentMock.mockRejectedValue(error)
    recoverFromChunkLoadErrorMock.mockReturnValue(true)

    const wrapper = mountHomeView()
    await flushPromises()

    expect(recoverFromChunkLoadErrorMock).toHaveBeenCalledWith(error, '/')
    expect(wrapper.find('.custom-home-page').exists()).toBe(true)
    expect(wrapper.html()).not.toContain('<strong>欢迎</strong>')
  })

  it('keeps URL-based custom home pages on iframe path without sanitizer work', async () => {
    appState.cachedPublicSettings.home_content = 'https://example.com/home'

    const wrapper = mountHomeView()
    await flushPromises()

    expect(sanitizeHomeContentMock).not.toHaveBeenCalled()
    expect(wrapper.find('iframe').attributes('src')).toBe('https://example.com/home')
  })

  it('uses the Chinese title structure on /home', async () => {
    routeState.fullPath = '/home'
    routeState.path = '/home'

    const wrapper = mountHomeView()
    await flushPromises()

    expect(wrapper.find('.hero-zh').exists()).toBe(true)
    expect(wrapper.find('.hero-en-title').exists()).toBe(false)
  })

  it('uses a spaced English title structure only on /en routes', async () => {
    routeState.fullPath = '/en'
    routeState.path = '/en'

    const wrapper = mountHomeView()
    await flushPromises()

    const title = wrapper.find('.hero-en-title')
    expect(title.exists()).toBe(true)
    expect(title.text()).toContain('Jisudeng')
    expect(title.text()).toContain('One API for AI models')
    expect(title.text()).not.toContain('JisudengOne')
  })
})
