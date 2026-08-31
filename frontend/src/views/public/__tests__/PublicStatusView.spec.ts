import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { afterEach, describe, expect, it, vi } from 'vitest'

import PublicStatusView from '@/views/public/PublicStatusView.vue'

const fetchPublicStatusSummaryMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/publicStatus', () => ({
  fetchPublicStatusSummary: fetchPublicStatusSummaryMock,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ cachedPublicSettings: null, siteName: '极速蹬' }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/status' }),
}))

function mountStatus(locale: 'zh' | 'en' = 'zh') {
  const i18n = createI18n({
    legacy: false,
    locale,
    messages: {
      zh: {
        publicStatus: {
          eyebrow: () => '运行数据',
          title: () => '系统状态',
          lede: () => '数据口径清晰可见。',
          dataNormal: () => '数据正常',
          dataDelayed: () => '数据延迟',
          unavailable: () => '暂不可用',
          recordedRequests: () => '已记录调用',
          availability: () => '30 天请求成功率',
          ttftP50: () => '24 小时首字延迟 P50',
          ttftP95: () => '24 小时首字延迟 P95',
          samples: ({ named }: { named: (key: string) => string }) => `样本 ${named('count')}`,
          window: ({ named }: { named: (key: string) => string }) => `统计窗口 ${named('start')} 至 ${named('end')}`,
          windowUnavailable: () => '统计窗口暂不可用',
          through: ({ named }: { named: (key: string) => string }) => `数据截至 ${named('time')}`,
          noWatermark: () => '暂无数据水位',
          note: () => '指标按固定窗口生成。',
          retry: () => '重试',
        },
      },
      en: {
        publicStatus: {
          eyebrow: () => 'Operational data',
          title: () => 'System status',
          lede: () => 'Clear windows and data freshness.',
          dataNormal: () => 'Data current',
          dataDelayed: () => 'Data delayed',
          unavailable: () => 'Unavailable',
          recordedRequests: () => 'Recorded requests',
          availability: () => '30-day request availability',
          ttftP50: () => '24-hour TTFT P50',
          ttftP95: () => '24-hour TTFT P95',
          samples: ({ named }: { named: (key: string) => string }) => `Samples ${named('count')}`,
          window: ({ named }: { named: (key: string) => string }) => `Window ${named('start')} to ${named('end')}`,
          windowUnavailable: () => 'Measurement window unavailable',
          through: ({ named }: { named: (key: string) => string }) => `Data through ${named('time')}`,
          noWatermark: () => 'No data watermark available',
          note: () => 'Metrics are generated on fixed windows.',
          retry: () => 'Retry',
        },
      },
    },
  })
  return mount(PublicStatusView, {
    global: {
      plugins: [i18n],
      stubs: {
        RouterLink: { props: ['to'], template: '<a><slot /></a>' },
        PublicContentLayout: { template: '<div><slot /></div>' },
        SupportFloatingCard: true,
      },
    },
  })
}

describe('PublicStatusView', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('renders server-owned windows, samples and status without treating computed_at as freshness', async () => {
    fetchPublicStatusSummaryMock.mockResolvedValue({
      total_requests: 657629,
      availability: {
        value_pct: 95.26,
        sample_count: 650000,
        window_start: '2026-07-30T14:00:00Z',
        window_end: '2026-08-29T14:00:00Z',
      },
      ttft: {
        p50_ms: 6690,
        p95_ms: 11420,
        sample_count: 73000,
        window_start: '2026-08-28T14:00:00Z',
        window_end: '2026-08-29T14:00:00Z',
      },
      data_through: '2026-08-29T14:00:00Z',
      computed_at: '2026-08-29T14:05:00Z',
      freshness: 'delayed',
    })

    const wrapper = mountStatus()
    await flushPromises()

    expect(wrapper.text()).toContain('数据延迟')
    expect(wrapper.text()).toContain('657,629')
    expect(wrapper.text()).toContain('95.26%')
    expect(wrapper.text()).toContain('6.69s')
    expect(wrapper.text()).toContain('11.42s')
    expect(wrapper.text()).toContain('样本 73,000')
    expect(wrapper.text()).toContain('统计窗口 2026-07-30 14:00 UTC 至 2026-08-29 14:00 UTC')
    expect(wrapper.text()).toContain('统计窗口 2026-08-28 14:00 UTC 至 2026-08-29 14:00 UTC')
    expect(wrapper.findAll('[data-testid="status-window"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('数据截至')
    expect(wrapper.text()).not.toContain('2026-08-29T14:05:00Z')
  })

  it('keeps metric geometry stable and explicitly shows unavailable data', async () => {
    fetchPublicStatusSummaryMock.mockResolvedValue(null)
    const wrapper = mountStatus('en')
    await flushPromises()

    expect(wrapper.text()).toContain('Unavailable')
    expect(wrapper.findAll('[data-testid="status-metric"]')).toHaveLength(4)
    expect(wrapper.findAll('[data-testid="status-window"]')).toHaveLength(0)
    expect(wrapper.find('button').text()).toBe('Retry')
  })

  it('keeps the service response honest when a snapshot lacks a measurement window', async () => {
    fetchPublicStatusSummaryMock.mockResolvedValue({
      total_requests: 1,
      availability: { value_pct: 100, sample_count: 1, window_start: null, window_end: null },
      ttft: { p50_ms: 20, p95_ms: 20, sample_count: 1, window_start: null, window_end: null },
      data_through: '2026-08-29T14:00:00Z',
      computed_at: '2026-08-29T14:05:00Z',
      freshness: 'unavailable',
    })

    const wrapper = mountStatus()
    await flushPromises()

    expect(wrapper.text()).toContain('统计窗口暂不可用')
    expect(wrapper.findAll('[data-testid="status-window"]')).toHaveLength(3)
  })

  it('re-evaluates a cached fresh snapshot from the business watermark', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-29T22:00:00Z'))
    fetchPublicStatusSummaryMock.mockResolvedValue({
      total_requests: 4,
      availability: { value_pct: 100, sample_count: 4, window_start: '2026-08-28T14:00:00Z', window_end: '2026-08-29T14:00:00Z' },
      ttft: { p50_ms: 20, p95_ms: 30, sample_count: 4, window_start: '2026-08-29T13:00:00Z', window_end: '2026-08-29T14:00:00Z' },
      data_through: '2026-08-29T14:00:00Z',
      computed_at: '2026-08-29T14:05:00Z',
      freshness: 'fresh',
    })

    const wrapper = mountStatus('en')
    await flushPromises()

    expect(wrapper.text()).toContain('Unavailable')
    expect(wrapper.find('.public-status-state').classes()).toContain('is-unavailable')
  })
})
