import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useHomeLiveStats } from '@/composables/useHomeLiveStats'
import {
  emptyHomeStats,
  formatHomeStatsTimestamp,
  formatHomeStatLatency,
  formatHomeStatRequests,
  formatHomeStatUptime,
  isHomeStatsSnapshotStale,
  loadHomeStatsSnapshot,
  toHomeStatsValues,
} from '@/utils/homeLiveStats'

const { fetchPublicHomeStatsMock } = vi.hoisted(() => ({
  fetchPublicHomeStatsMock: vi.fn(),
}))

vi.mock('@/api/publicHomeStats', () => ({
  fetchPublicHomeStats: fetchPublicHomeStatsMock,
}))

describe('homeLiveStats', () => {
  beforeEach(() => {
    vi.useRealTimers()
    fetchPublicHomeStatsMock.mockReset()
    localStorage.clear()
  })

  it('never advances a real snapshot with elapsed time', () => {
    vi.useFakeTimers()
    const snapshot = toHomeStatsValues({
      total_requests: 11336,
      availability_pct: 99.5,
      avg_ttft_ms: 600,
      ops_data_through: '2026-07-16T01:00:00Z',
      computed_at: '2026-07-16T02:00:00Z',
    })
    expect(snapshot.requests).toBe(11336)
    vi.advanceTimersByTime(60 * 60 * 1000)
    expect(snapshot.requests).toBe(11336)
    vi.useRealTimers()
  })

  it('returns unavailable values without a real snapshot', () => {
    expect(emptyHomeStats()).toEqual({ requests: null, uptimePct: null, latencyMs: null })
  })

  it('formats null metrics as unavailable', () => {
    expect(formatHomeStatRequests(null)).toBe('--')
    expect(formatHomeStatUptime(null)).toBe('--')
    expect(formatHomeStatLatency(null)).toBe('--')
  })

  it('loads only a persisted real API snapshot', () => {
    const raw = JSON.stringify({
      total_requests: 99,
      availability_pct: null,
      avg_ttft_ms: 420,
      ops_data_through: null,
      computed_at: '2026-07-16T02:00:00Z',
    })
    expect(loadHomeStatsSnapshot(raw, Date.parse('2026-07-16T02:01:00Z'))?.total_requests).toBe(99)
    expect(loadHomeStatsSnapshot(raw, Date.parse('2026-07-16T02:03:00Z'))).toBeNull()
    expect(loadHomeStatsSnapshot(JSON.stringify({ anchorMs: 0, creditedMs: 5000 }))).toBeNull()
  })

  it('marks snapshots stale at the exact freshness boundary', () => {
    const snapshot = {
      total_requests: 99,
      availability_pct: 99.9,
      avg_ttft_ms: 420,
      ops_data_through: '2026-07-16T01:00:00+08:00',
      computed_at: '2026-07-16T02:00:00Z',
    }
    const computedAt = Date.parse(snapshot.computed_at)

    expect(isHomeStatsSnapshotStale(snapshot, computedAt + 3 * 60_000 - 1)).toBe(false)
    expect(isHomeStatsSnapshotStale(snapshot, computedAt + 3 * 60_000)).toBe(true)
  })

  it('rejects invalid and materially future snapshot timestamps', () => {
    const now = Date.parse('2026-07-16T02:00:00Z')
    const base = {
      total_requests: 99,
      availability_pct: null,
      avg_ttft_ms: null,
      ops_data_through: null,
    }

    expect(isHomeStatsSnapshotStale({ ...base, computed_at: 'not-a-date' }, now)).toBe(true)
    expect(isHomeStatsSnapshotStale({ ...base, computed_at: '2026-07-16T02:03:01Z' }, now)).toBe(true)
  })

  it('formats mixed-offset timestamps as one explicit UTC instant', () => {
    expect(formatHomeStatsTimestamp('2026-07-17T01:00:00+08:00', 'zh-CN')).toContain(
      '2026-07-16',
    )
    expect(formatHomeStatsTimestamp('2026-07-16T17:00:00Z', 'zh-CN')).toContain(
      '2026-07-16',
    )
  })

  it('reactively marks a mounted snapshot stale when the freshness boundary passes', async () => {
    vi.useFakeTimers()
    vi.setSystemTime('2026-07-16T02:00:00Z')
    fetchPublicHomeStatsMock.mockResolvedValue({
      total_requests: 99,
      availability_pct: 99.9,
      avg_ttft_ms: 420,
      ops_data_through: '2026-07-16T02:00:00Z',
      computed_at: '2026-07-16T02:00:00Z',
    })

    const wrapper = mount(defineComponent({
      setup() {
        return useHomeLiveStats()
      },
      template: '<div :data-stale="String(isStale)" />',
    }))
    await flushPromises()
    expect(wrapper.attributes('data-stale')).toBe('false')

    await vi.advanceTimersByTimeAsync(3 * 60_000)
    await flushPromises()
    expect(wrapper.attributes('data-stale')).toBe('true')

    wrapper.unmount()
    vi.useRealTimers()
  })
})
