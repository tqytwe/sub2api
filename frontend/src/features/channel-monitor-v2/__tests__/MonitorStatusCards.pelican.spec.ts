import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { MonitorMatrixRow } from '@/api/channelMonitorV2'
import MonitorStatusCards from '../MonitorStatusCards.vue'

const { getShowcaseMock, getShowcaseItemMock } = vi.hoisted(() => ({ getShowcaseMock: vi.fn(), getShowcaseItemMock: vi.fn() }))
vi.mock('@/api/pelicanShowcase', () => ({ getShowcase: getShowcaseMock, getShowcaseItem: getShowcaseItemMock }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh-CN' } }) }))
vi.mock('@/utils/platformColors', () => ({ platformLabel: (value: string) => value }))

const rows = [7, 8].map((id) => ({
  platform: 'openai', group_id: id, group_name: `Group ${id}`,
  metrics: { request_count: 1, error_rate: 0, cache_rate: 0, ttft: { p50_ms: 10 } },
  health: { overall: 'healthy', minimum_sample: 1 }, buckets: [],
})) as unknown as MonitorMatrixRow[]

describe('V2 Pelican group-card entry', () => {
  it('shows health and traffic indicators when volume counts are hidden from users', () => {
    const wrapper = mount(MonitorStatusCards, {
      props: {
        items: [{
          ...rows[0],
          metrics: { has_samples: true, request_count: 0, error_rate: 0.1, cache_rate: 0.4, cache_rate_denominator: 0, ttft: { p50_ms: 120 } },
          buckets: [{ bucket_start: '2026-11-03T10:00:00Z', metrics: { has_samples: true, request_count: 0 }, health: { overall: 'unknown', minimum_sample: 50 } }],
          sample_buckets: [{ bucket_start: '2026-11-03T10:00:00Z', metrics: { has_samples: true, request_count: 0 }, health: { overall: 'unknown', minimum_sample: 50 } }],
        } as unknown as MonitorMatrixRow],
        loading: false,
        now: Date.now(),
        timelineMode: 'samples',
      },
      global: { stubs: { Icon: true, PelicanGroupHistoryDialog: true } },
    })

    expect(wrapper.text()).toContain('90.0%')
    expect(wrapper.text()).toContain('40.0%')
    expect(wrapper.find('[role="img"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('channelMonitorV2.cards.noSamples')
  })

  it('shows the action only for configured groups and opens that group’s history', async () => {
    getShowcaseMock.mockReset().mockResolvedValue({ enabled: true, groups: [{ id: 7, name: 'Group 7', platform: 'openai', items: [] }] })
    // design-governance-allow: inline-svg - model artwork fixture verifies the group preview response, not a UI icon.
    getShowcaseItemMock.mockReset().mockResolvedValue({ id: 21, group_id: 7, response_text: '<svg></svg>' })
    const wrapper = mount(MonitorStatusCards, {
      props: { items: rows, loading: false, now: Date.now(), showcaseGroups: [{ id: 7, name: 'Group 7', platform: 'openai', items: [] }] },
      global: { stubs: { Icon: true, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, PelicanArtworkPreview: true } },
    })

    expect(wrapper.find('[data-testid="pelican-history-open-7"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pelican-history-open-8"]').exists()).toBe(false)
    await wrapper.find('[data-testid="pelican-history-open-7"]').trigger('click')
    await flushPromises()

    expect(getShowcaseMock).toHaveBeenCalledWith({ groupId: 7 })
    expect(getShowcaseMock).not.toHaveBeenCalledWith({ groupId: 8 })
    expect(wrapper.find('[data-testid="pelican-group-history"]').exists()).toBe(true)
  })
})
