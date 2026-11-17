import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import OpsDashboard from '../OpsDashboard.vue'
import LatencyObservabilityPanel from '@/components/admin/usage/LatencyObservabilityPanel.vue'

const mocks = vi.hoisted(() => ({
  latency: vi.fn().mockResolvedValue({ groups: [] }),
  snapshot: vi.fn().mockResolvedValue({
    overview: { start_time: '2026-11-17T00:00:00Z', end_time: '2026-11-17T01:00:00Z' },
    throughput_trend: { points: [] }, error_trend: { points: [] },
  }),
  replace: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('@/api/admin/latencyObservability', () => ({ getLatencyReport: mocks.latency }))
vi.mock('@/api/admin/ops', async importOriginal => ({ ...await importOriginal<typeof import('@/api/admin/ops')>(), opsAPI: {
  getAdvancedSettings: vi.fn().mockResolvedValue({ display_alert_events: false, display_openai_token_stats: false, auto_refresh_enabled: false, auto_refresh_interval_seconds: 30 }),
  getMetricThresholds: vi.fn().mockResolvedValue(null),
  getDashboardSnapshotV2: mocks.snapshot,
  getThroughputTrend: vi.fn().mockResolvedValue({ points: [] }),
  getLatencyHistogram: vi.fn().mockResolvedValue(null),
  getErrorDistribution: vi.fn().mockResolvedValue(null),
} }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
  useAdminSettingsStore: () => ({ opsMonitoringEnabled: true, fetch: vi.fn().mockResolvedValue(undefined) }),
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { platform: 'openai', group_id: '7' } }),
  useRouter: () => ({ replace: mocks.replace }),
}))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

afterEach(() => { vi.clearAllMocks(); localStorage.clear() })

it('uses the loaded Ops window and selected identity filters, with no report request before expansion', async () => {
  localStorage.clear()
  const wrapper = shallowMount(OpsDashboard, {
    global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      LatencyObservabilityPanel: false,
    } },
  })
  try {
    await flushPromises()
    const panel = wrapper.getComponent(LatencyObservabilityPanel)
    expect(mocks.latency).not.toHaveBeenCalled()
    expect(panel.props()).toMatchObject({ ops: true, collapsible: true, startDate: '2026-11-17T00:00:00Z', endDate: '2026-11-17T01:00:00Z', platform: 'openai', groupId: 7 })
    await panel.get('[data-testid="latency-panel-toggle"]').trigger('click')
    await flushPromises()
    expect(mocks.latency).toHaveBeenCalledWith(true, expect.objectContaining({ start_date: '2026-11-17T00:00:00Z', end_date: '2026-11-17T01:00:00Z', platform: 'openai', group_id: 7 }), expect.anything())
  } finally { wrapper.unmount() }
})
