import { describe, expect, it } from 'vitest'
import {
  advanceOnline,
  defaultPersisted,
  markOffline,
  markOnline,
  mergeHomeLiveStats,
  syntheticRequests,
} from '@/utils/homeLiveStats'

describe('homeLiveStats', () => {
  it('advances requests while online', () => {
    let state = defaultPersisted(0)
    state = advanceOnline(state, 1000)
    expect(syntheticRequests(state.creditedMs)).toBeGreaterThan(12_847_360)
  })

  it('pauses while offline and catch-up on reconnect', () => {
    let state = defaultPersisted(0)
    state = advanceOnline(state, 5000)
    const mid = state.creditedMs

    state = markOffline(state, 10_000)
    state = advanceOnline(state, 3000)
    expect(state.creditedMs).toBe(mid)

    state = markOnline(state, 25_000)
    expect(state.creditedMs).toBe(mid + 15_000)
  })

  it('uses real requests when higher than synthetic', () => {
    const merged = mergeHomeLiveStats(60_000, {
      totalRequests: 20_000_000,
      availabilityPct: 99.5,
      avgTtftMs: 420,
      fetchedAtMs: Date.now(),
    })
    expect(merged.requests).toBe(20_000_000)
    expect(merged.latencyMs).toBeGreaterThan(0)
  })
})
