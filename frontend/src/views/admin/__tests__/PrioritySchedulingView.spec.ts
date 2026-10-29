import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PrioritySchedulingView from '../PrioritySchedulingView.vue'

const { getPriorityConfig, savePriorityConfig, getPrioritySnapshot } = vi.hoisted(() => ({
  getPriorityConfig: vi.fn(),
  savePriorityConfig: vi.fn(),
  getPrioritySnapshot: vi.fn(),
}))

vi.mock('@/api/admin/priorityScheduling', () => ({ getPriorityConfig, savePriorityConfig, getPrioritySnapshot }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountView() {
  return mount(PrioritySchedulingView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        DataTable: {
          props: ['columns', 'data'],
          template: '<div><div v-for="row in data" :key="row.account_id"><slot name="cell-account" :row="row" /><slot name="cell-state" :row="row" /></div><slot name="empty" /></div>',
        },
        Icon: true,
      },
    },
  })
}

describe('PrioritySchedulingView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getPriorityConfig.mockResolvedValue({ enabled: true, latency_penalty_mode: 'deprioritize', latency_penalty_minutes: 10, latency_percentile: 90, latency_measurement: 'legacy_forward' })
    getPrioritySnapshot.mockResolvedValue({ at: '2026-10-28T00:00:00Z', model: 'public-gpt-test', inbound_endpoint: '/v1/responses', group_id: 3, candidates: [{ account_id: 11, account_name: 'slow', tier: 'degraded', latency_stopped: true, latency_penalty_until: '2026-10-28T00:10:00Z', selected_ttft_ms: 900, samples: 20, reasons: ['latency_stopped'] }] })
    savePriorityConfig.mockResolvedValue({ enabled: true, latency_penalty_mode: 'stop', latency_penalty_minutes: 10, latency_percentile: 90, latency_measurement: 'legacy_forward' })
  })

  it('loads the real config and renders an explicit stopped candidate state', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(getPriorityConfig).toHaveBeenCalledOnce()
    expect(getPrioritySnapshot).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('admin.priorityScheduling.stopped')
    expect(wrapper.text()).toContain('admin.priorityScheduling.penaltyUntil')
    expect(wrapper.text()).toContain('/v1/responses')
  })

  it('saves the selected penalty mode without dropping the server config fields', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="latency-penalty-mode"]').setValue('stop')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(savePriorityConfig).toHaveBeenCalledWith(expect.objectContaining({
      enabled: true,
      latency_penalty_mode: 'stop',
      latency_penalty_minutes: 10,
      latency_measurement: 'legacy_forward',
    }))
  })
})
