import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { saveInternalSpeedTestPayload } from '@/utils/internalSpeedTest'
import SpeedTestView from '../SpeedTestView.vue'

const routerPushMock = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (!params) return key
      return `${key} ${JSON.stringify(params)}`
    },
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPushMock }),
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<main><slot /></main>' },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { template: '<span />' },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() }),
  useAuthStore: () => ({ isAuthenticated: true, isAdmin: false }),
}))

function mountView() {
  return mount(SpeedTestView, {
    global: { stubs: {} },
  })
}

describe('SpeedTestView', () => {
  beforeEach(() => {
    sessionStorage.clear()
    routerPushMock.mockReset()
    vi.unstubAllGlobals()
  })

  it('does not ask for or render an API key when opened without a launch payload', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('keys.speedTest.missing.title')
    expect(wrapper.text()).toContain('keys.speedTest.backToKeys')
    expect(wrapper.html()).not.toContain('sk-')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('loads models from the session payload without exposing the key in rendered markup', async () => {
    saveInternalSpeedTestPayload({
      apiKey: 'sk-view-secret',
      baseUrl: 'https://api.example.com',
      keyName: 'Primary Key',
    })
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: [{ id: 'gpt-5.5' }, { id: 'gpt-5.4-mini' }] }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mountView()
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.example.com/v1/models',
      expect.objectContaining({
        headers: { Authorization: 'Bearer sk-view-secret' },
      }),
    )
    expect(wrapper.text()).toContain('Primary Key')
    expect(wrapper.html()).not.toContain('sk-view-secret')
    expect(wrapper.text()).toContain('gpt-5.5')
  })
})
