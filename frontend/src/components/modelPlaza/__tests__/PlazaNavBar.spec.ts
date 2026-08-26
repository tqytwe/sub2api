import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaNavBar from '../PlazaNavBar.vue'

const localeState = { value: 'zh' }
const publicSettingsState = { site_name: '', site_logo: '' }
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: localeState }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: publicSettingsState }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: false, isAdmin: false }) }))

describe('PlazaNavBar', () => {
  beforeEach(() => {
    localeState.value = 'zh'
    publicSettingsState.site_name = ''
    publicSettingsState.site_logo = ''
  })

  it('uses the current branded fallback logo and shared public frame', () => {
    const wrapper = mount(PlazaNavBar, { global: { stubs: { RouterLink: true } } })
    const logo = wrapper.get('[data-testid="model-plaza-logo"]')
    expect(logo.attributes('src')).toBe('/logo.png')
    expect(logo.attributes('alt')).toBe('极速蹬')
    expect(logo.classes()).toContain('brand-logo-asset--deng')
    expect(wrapper.find('.public-content-frame').exists()).toBe(true)
  })

  it('localizes the empty configured site name on the English public surface', () => {
    localeState.value = 'en'
    const wrapper = mount(PlazaNavBar, { global: { stubs: { RouterLink: true } } })

    expect(wrapper.text()).toContain('Jisudeng')
    expect(wrapper.text()).not.toContain('极速蹬')
    expect(wrapper.get('[data-testid="model-plaza-logo"]').attributes('alt')).toBe('Jisudeng')
  })

  it('keeps a configured logo unscaled', () => {
    publicSettingsState.site_logo = '/tenant-logo.png'
    const wrapper = mount(PlazaNavBar, { global: { stubs: { RouterLink: true } } })

    const logo = wrapper.get('[data-testid="model-plaza-logo"]')
    expect(logo.attributes('src')).toBe('/tenant-logo.png')
    expect(logo.classes()).not.toContain('brand-logo-asset--deng')
  })
})
