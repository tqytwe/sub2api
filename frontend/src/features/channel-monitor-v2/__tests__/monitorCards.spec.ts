import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import { candyDisplayState, candyHistoryLimit, candyHistorySlots, monitorCardTimeline, monitorRefreshSeconds } from '../monitorCards'

function row(): MonitorMatrixRow {
  return {
    platform: 'openai', group_id: 3, metrics: { request_count: 4, error_rate: 0.1, cache_rate: 0.2, ttft: { p50_ms: 30 } } as MonitorMatrixRow['metrics'],
    health: { overall: 'healthy', minimum_sample: 2 } as MonitorMatrixRow['health'], buckets: [
      { bucket_start: '2026-11-02T00:00:00Z', metrics: { request_count: 1 } as MonitorMatrixRow['metrics'], health: { overall: 'critical', minimum_sample: 2 } as MonitorMatrixRow['health'] },
      { bucket_start: '2026-11-02T00:05:00Z', metrics: { request_count: 3 } as MonitorMatrixRow['metrics'], health: { overall: 'warning', minimum_sample: 2 } as MonitorMatrixRow['health'] },
    ],
  }
}

describe('V2 card timeline and Candy presentation', () => {
  it('keeps time-window gaps and filters recent samples by configured minimum', () => {
    const coverage = { requested_start: '2026-11-02T00:00:00Z', requested_end: '2026-11-02T00:10:00Z', bucket_seconds: 300 } as MonitorCoverage
    const value = row()
    expect(monitorCardTimeline(value, coverage, 'windows')).toHaveLength(18)
    expect(monitorCardTimeline(value, coverage, 'windows')[0].state).toBe('critical')
    expect(monitorCardTimeline(value, coverage, 'samples')).toHaveLength(1)
    value.sample_buckets = value.buckets
    expect(monitorCardTimeline(value, undefined, 'samples').map((bar) => bar.state)).toEqual(['warning'])
  })

  it('caps Candy history to the configured limit and identifies stale checks', () => {
    const history = { model: 'm', reasoning_effort: 'medium', interval_minutes: 5, history_limit: 15, results: Array.from({ length: 20 }, (_, index) => ({ checked_at: new Date(index * 1000).toISOString(), verdict: 'correct' as const, latency_ms: 5 })) }
    expect(candyHistoryLimit(history)).toBe(15)
    expect(candyHistorySlots(history)).toHaveLength(15)
    expect(candyDisplayState(history, Date.now())).toBe('stale')
  })

  it('refreshes at the configured Candy cadence without polling faster than once per minute', () => {
    expect(monitorRefreshSeconds(300, [{ ...row(), candy: { model: 'm', reasoning_effort: 'medium', interval_minutes: 2, results: [] } }], false)).toBe(120)
    expect(monitorRefreshSeconds(300, [{ ...row(), candy: { model: 'm', reasoning_effort: 'medium', interval_minutes: 1, results: [] } }], false)).toBe(60)
    expect(monitorRefreshSeconds(300, [], true)).toBe(10)
  })
})
