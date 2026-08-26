import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LocaleSwitcher from '../LocaleSwitcher.vue'

const { route, routerPush, locale, setLocale } = vi.hoisted(() => ({
  route: { path: '/dashboard', query: {} as Record<string, string> },
  routerPush: vi.fn(),
  locale: { value: 'zh' },
  setLocale: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: routerPush })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale })
}))

vi.mock('@/i18n', () => ({
  availableLocales: [
    { code: 'zh', name: '中文', flag: '中' },
    { code: 'en', name: 'English', flag: 'EN' }
  ],
  setLocale
}))

describe('LocaleSwitcher workspace routing', () => {
  beforeEach(() => {
    route.path = '/dashboard'
    route.query = {}
    locale.value = 'zh'
    routerPush.mockReset()
    routerPush.mockResolvedValue(undefined)
    setLocale.mockReset()
  })

  const mountSwitcher = () =>
    mount(LocaleSwitcher, {
      global: {
        stubs: { Icon: true }
      }
    })

  it('adds lang=en to the current workspace route', async () => {
    const wrapper = mountSwitcher()
    await wrapper.find('button').trigger('click')
    await wrapper.findAll('button')[2]!.trigger('click')
    await flushPromises()

    expect(routerPush).toHaveBeenCalledWith({
      path: '/dashboard',
      query: { lang: 'en' }
    })
  })

  it('removes explicit language query parameters when switching back to Chinese', async () => {
    route.query = { lang: 'en', locale: 'en', tab: 'usage' }
    locale.value = 'en'
    const wrapper = mountSwitcher()
    await wrapper.find('button').trigger('click')
    await wrapper.findAll('button')[1]!.trigger('click')
    await flushPromises()

    expect(routerPush).toHaveBeenCalledWith({
      path: '/dashboard',
      query: { tab: 'usage' }
    })
  })
})
