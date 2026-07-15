import type { PublicHomeStatsResponse } from '@/api/publicHomeStats'

export interface HomeLiveStatsValues {
  snapshotId: string
  source: PublicHomeStatsResponse['source']
  updatedAt: string
  requests24h: number
  activeUsers7d: number
	successRate30d: number | null
  tokensTotal: number
}

export function normalizeHomeLiveStats(data: PublicHomeStatsResponse): HomeLiveStatsValues {
  return {
    snapshotId: data.snapshot_id,
    source: data.source,
    updatedAt: data.updated_at,
    requests24h: Math.max(0, Math.floor(data.requests_24h || 0)),
    activeUsers7d: Math.max(0, Math.floor(data.active_users_7d || 0)),
		successRate30d: data.success_rate_30d == null ? null : clamp(data.success_rate_30d, 0, 100),
    tokensTotal: Math.max(0, Math.floor(data.tokens_total || 0)),
  }
}

export function formatHomeStatNumber(n: number): string {
  return Math.max(0, Math.floor(n)).toLocaleString('en-US')
}

export function formatHomeStatPercent(n: number | null | undefined): string {
	return n == null ? '—' : clamp(n, 0, 100).toFixed(2)
}

export function formatHomeStatCompact(n: number): string {
  const value = Math.max(0, n)
  if (value >= 1_000_000_000) return `${roundCompact(value / 1_000_000_000)}B`
  if (value >= 1_000_000) return `${roundCompact(value / 1_000_000)}M`
  if (value >= 1_000) return `${roundCompact(value / 1_000)}K`
  return String(Math.floor(value))
}

function roundCompact(n: number): string {
  return n >= 100 ? n.toFixed(0) : n >= 10 ? n.toFixed(1) : n.toFixed(2)
}

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, Number.isFinite(n) ? n : min))
}
