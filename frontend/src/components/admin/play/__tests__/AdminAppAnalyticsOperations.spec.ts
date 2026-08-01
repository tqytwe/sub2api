import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminAppAnalyticsOperations from '../AdminAppAnalyticsOperations.vue'

const api = vi.hoisted(() => ({ getAppAnalytics: vi.fn() }))

vi.mock('@/api/admin/play', () => ({ default: api }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'zh-CN' }, t: (key: string) => key }),
}))

describe('AdminAppAnalyticsOperations', () => {
  beforeEach(() => {
    api.getAppAnalytics.mockReset().mockResolvedValue({
      scans: 0,
      download_redirects: 0,
      first_launches: 0,
      installs: 0,
      registered_installs: 0,
      active_users: 0,
      dau: 0,
      wau: 0,
      mau: 0,
      funnel: [],
      versions: null,
    })
  })

  it('keeps the empty state visible when legacy analytics responses return null versions', async () => {
    const wrapper = mount(AdminAppAnalyticsOperations, { global: { stubs: { Icon: true } } })
    await flushPromises()

    expect(api.getAppAnalytics).toHaveBeenCalledWith({ period: '30d', version: undefined, channel: undefined })
    expect(wrapper.text()).toContain('admin.playOps.appAnalytics.empty')
  })
})
