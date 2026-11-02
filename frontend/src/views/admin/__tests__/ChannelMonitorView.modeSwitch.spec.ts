import { reactive } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelMonitorView from '@/views/admin/ChannelMonitorView.vue'

const mocks = vi.hoisted(() => ({ fetchPublicSettings: vi.fn() }))
const store = vi.hoisted(() => ({ settings: null as null | { channel_monitor_enabled: boolean; channel_monitor_mode: string } }))

vi.mock('@/utils/featureFlags', () => ({
  getChannelMonitorMode: () => store.settings!.channel_monitor_mode,
  isChannelMonitorV1Mode: () => store.settings!.channel_monitor_mode === 'v1',
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    get cachedPublicSettings() { return store.settings },
    fetchPublicSettings: mocks.fetchPublicSettings,
  }),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { channelMonitor: { list: vi.fn().mockResolvedValue({ items: [], total: 0 }) } } }))
vi.mock('@/features/channel-monitor-v2/MonitorSettingsPanel.vue', () => ({ default: { template: '<div data-testid="v2-panel" />' } }))
vi.mock('@/features/channel-monitor-v3/V3SettingsPanel.vue', () => ({ default: { template: '<div data-testid="v3-panel" />' } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

beforeEach(() => {
  store.settings = reactive({ channel_monitor_enabled: true, channel_monitor_mode: 'v2' })
  vi.clearAllMocks()
})

describe('admin channel monitor tabs', () => {
  it('opens the configured mode tab and allows switching panels without changing site mode', async () => {
    const wrapper = mount(ChannelMonitorView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>', }, Icon: true, TablePageLayout: true, MonitorFormDialog: true, MonitorTemplateManagerDialog: true, MonitorRunResultDialog: true, RouterLink: true } },
    })

    expect(wrapper.find('[data-testid="v2-panel"]').exists()).toBe(true)
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs[0].attributes('aria-selected')).toBe('false')
    expect(tabs[1].attributes('aria-selected')).toBe('true')

    await tabs[0].trigger('click')
    expect(wrapper.find('[data-testid="v3-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="v2-panel"]').exists()).toBe(false)
    expect(store.settings!.channel_monitor_mode).toBe('v2')
    expect(mocks.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('opens V3 settings when the active site mode is V3', () => {
    store.settings!.channel_monitor_mode = 'v3'
    const wrapper = mount(ChannelMonitorView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true, TablePageLayout: true, MonitorFormDialog: true, MonitorTemplateManagerDialog: true, MonitorRunResultDialog: true, RouterLink: true } },
    })
    expect(wrapper.find('[data-testid="v3-panel"]').exists()).toBe(true)
    expect(wrapper.findAll('[role="tab"]')[0].attributes('aria-selected')).toBe('true')
  })
})
