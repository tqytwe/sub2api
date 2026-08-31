import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { MonitorConfig } from '@/api/channelMonitorV2'
import MonitorSettingsPanel from '../MonitorSettingsPanel.vue'

const {
  getConfigMock,
  getGroupsMock,
  updateConfigMock,
  showErrorMock,
  showSuccessMock,
} = vi.hoisted(() => ({
  getConfigMock: vi.fn(),
  getGroupsMock: vi.fn(),
  updateConfigMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        key === 'channelMonitorV2.filters.groupId'
          ? `分组 ${params?.id}`
          : key === 'channelMonitorV2.settings.refreshIntervalMinutes'
            ? `${params?.minutes} 分钟`
            : key,
      locale: { value: 'zh-CN' },
    }),
  }
})

vi.mock('@/api/channelMonitorV2', async () => {
  const actual = await vi.importActual<typeof import('@/api/channelMonitorV2')>('@/api/channelMonitorV2')
  return { ...actual, getConfig: getConfigMock, updateConfig: updateConfigMock }
})

vi.mock('@/api/admin', () => ({
  adminAPI: { groups: { getAllIncludingInactive: getGroupsMock } },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: showErrorMock, showSuccess: showSuccessMock, cachedPublicSettings: { channel_monitor_enabled: true } }),
}))

vi.mock('@/utils/featureFlags', () => ({
  getChannelMonitorMode: () => 'v2',
  isChannelMonitorV2Mode: () => true,
}))

vi.mock('@/utils/localizedEnum', () => ({ localizedEnumOrUnknown: (_t: unknown, key: string) => key }))
vi.mock('@/utils/platformColors', () => ({ platformLabel: (value: string) => value }))

const config: MonitorConfig = {
  version: 2,
  enabled: true,
  refresh_interval_seconds: 60,
  platforms: [{ platform: 'openai', enabled: true, models: ['gpt-5'] }],
  group_ids: [7],
  ignored_error_categories: [],
  health_thresholds: {
    minimum_sample: 50,
    warning_error_rate: 0.05,
    critical_error_rate: 0.2,
    target_ttft_ms: 3000,
    warning_ttft_ms: 3000,
    critical_ttft_ms: 10000,
    warning_cache_rate: 0.85,
    critical_cache_rate: 0.6,
    error_weight: 0.6,
    ttft_weight: 0.2,
    cache_weight: 0.2,
  },
}

function mountPanel() {
  return mount(MonitorSettingsPanel, {
    global: {
      stubs: {
        Icon: true,
        Toggle: { props: ['modelValue'], template: '<input type="checkbox" :checked="modelValue" />' },
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('MonitorSettingsPanel loading boundaries', () => {
  beforeEach(() => {
    getConfigMock.mockReset().mockResolvedValue(structuredClone(config))
    getGroupsMock.mockReset().mockResolvedValue([])
    updateConfigMock.mockReset().mockResolvedValue(structuredClone(config))
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('keeps the loaded configuration editable when the optional groups request fails', async () => {
    getGroupsMock.mockRejectedValueOnce(new Error('groups unavailable'))
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('channelMonitorV2.settings.groupsLoadFailed')
    expect(wrapper.find('[data-testid="channel-monitor-v2-groups-retry"]').exists()).toBe(true)
    expect((wrapper.vm as any).draft.group_ids).toEqual([7])

    ;(wrapper.vm as any).draft.enabled = false
    await wrapper.vm.$nextTick()
    expect(wrapper.find('button.btn-primary').attributes('disabled')).toBeUndefined()
  })

  it('shows a retryable page error when the required configuration request fails', async () => {
    getConfigMock.mockRejectedValueOnce(new Error('config unavailable'))
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('channelMonitorV2.settings.loadFailed')
    expect(wrapper.find('[data-testid="channel-monitor-v2-config-retry"]').exists()).toBe(true)
    expect(wrapper.find('button.btn-primary').exists()).toBe(false)
  })

  it('uses the localized group fallback alongside an internal group identifier', async () => {
    getGroupsMock.mockResolvedValueOnce([{ id: 7, name: '默认组', platform: 'openai' }])
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('openai · 分组 7')
    expect(wrapper.text()).not.toContain('openai · #7')
  })

  it('localizes refresh interval controls for the active locale', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('1 分钟')
    expect(wrapper.text()).toContain('5 分钟')
    expect(wrapper.text()).not.toContain('1 min')
    expect(wrapper.text()).not.toContain('5 min')
  })
})
