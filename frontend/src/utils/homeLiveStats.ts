import type { PublicHomeStatsResponse } from '@/api/publicHomeStats'

export const HOME_LIVE_STATS_STORAGE_KEY = 'home_live_stats_v2'

export interface HomeLiveStatsValues {
  requests: number | null
  uptimePct: number | null
  latencyMs: number | null
}

export function emptyHomeStats(): HomeLiveStatsValues {
  return {
    requests: null,
    uptimePct: null,
    latencyMs: null,
  }
}

export function toHomeStatsValues(snapshot: PublicHomeStatsResponse | null): HomeLiveStatsValues {
  if (!snapshot) return emptyHomeStats()
  return {
    requests: finiteNumberOrNull(snapshot.total_requests),
    uptimePct: finiteNumberOrNull(snapshot.availability_pct),
    latencyMs: finiteNumberOrNull(snapshot.avg_ttft_ms),
  }
}

export function loadHomeStatsSnapshot(raw: string | null): PublicHomeStatsResponse | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<PublicHomeStatsResponse>
    if (
      typeof parsed.total_requests !== 'number'
      || !Number.isFinite(parsed.total_requests)
      || typeof parsed.computed_at !== 'string'
      || parsed.computed_at.length === 0
    ) {
      return null
    }
    if (!nullableFiniteNumber(parsed.availability_pct) || !nullableFiniteNumber(parsed.avg_ttft_ms)) {
      return null
    }
    if (parsed.ops_data_through !== null && typeof parsed.ops_data_through !== 'string') {
      return null
    }
    return {
      total_requests: parsed.total_requests,
      availability_pct: parsed.availability_pct ?? null,
      avg_ttft_ms: parsed.avg_ttft_ms ?? null,
      ops_data_through: parsed.ops_data_through ?? null,
      computed_at: parsed.computed_at,
    }
  } catch {
    return null
  }
}

export function formatHomeStatRequests(value: number | null): string {
  if (value == null) return '--'
  return Math.max(0, Math.floor(value)).toLocaleString('en-US')
}

export function formatHomeStatUptime(value: number | null): string {
  if (value == null) return '--'
  return value.toFixed(2)
}

export function formatHomeStatLatency(value: number | null): string {
  if (value == null) return '--'
  return String(Math.round(value))
}

function finiteNumberOrNull(value: number | null): number | null {
  return value != null && Number.isFinite(value) ? value : null
}

function nullableFiniteNumber(value: unknown): boolean {
  return value === null || value === undefined || (typeof value === 'number' && Number.isFinite(value))
}
