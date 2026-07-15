import { describe, expect, it } from 'vitest'
import { formatHomeStatCompact, formatHomeStatPercent, normalizeHomeLiveStats } from '@/utils/homeLiveStats'

describe('homeLiveStats', () => {
  it('uses the canonical server snapshot without synthetic growth', () => {
    const values = normalizeHomeLiveStats({
      snapshot_id: '2026-07-15T10:30:00Z',
      updated_at: '2026-07-15T10:30:00Z',
      source: 'live',
      requests_24h: 42,
      requests_total: 100,
      active_users_7d: 7,
      tokens_total: 1_250_000,
      success_rate_30d: 99.92,
      p50_ttft_ms: 386,
      p95_ttft_ms: 820,
    })
    expect(values.snapshotId).toBe('2026-07-15T10:30:00Z')
    expect(values.requests24h).toBe(42)
    expect(values.activeUsers7d).toBe(7)
  })

  it('formats large token totals compactly', () => {
    expect(formatHomeStatCompact(1_250_000)).toBe('1.25M')
  })

  it('shows no success-rate value when the snapshot has no samples', () => {
    expect(formatHomeStatPercent(null)).toBe('—')
  })
})
