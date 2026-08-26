import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaNavBar from '../PlazaNavBar.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { site_name: '', site_logo: '' } }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: false, isAdmin: false }) }))

describe('PlazaNavBar', () => {
  it('uses the current branded fallback logo and shared public frame', () => {
    const wrapper = mount(PlazaNavBar, { global: { stubs: { RouterLink: true } } })
    expect(wrapper.get('[data-testid="model-plaza-logo"]').attributes('src')).toBe('/logo.png')
    expect(wrapper.find('.public-content-frame').exists()).toBe(true)
  })
})
