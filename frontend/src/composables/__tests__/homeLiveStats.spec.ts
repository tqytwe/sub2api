import { describe, expect, it, vi } from 'vitest'
import {
  emptyHomeStats,
  formatHomeStatLatency,
  formatHomeStatRequests,
  formatHomeStatUptime,
  loadHomeStatsSnapshot,
  toHomeStatsValues,
} from '@/utils/homeLiveStats'

describe('homeLiveStats', () => {
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
    expect(loadHomeStatsSnapshot(raw)?.total_requests).toBe(99)
    expect(loadHomeStatsSnapshot(JSON.stringify({ anchorMs: 0, creditedMs: 5000 }))).toBeNull()
  })
})
