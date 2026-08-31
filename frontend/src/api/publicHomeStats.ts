import { apiClient } from './client'

export interface PublicHomeStatsResponse {
  total_requests: number
  availability_pct: number | null
  avg_ttft_ms: number | null
  ops_data_through: string | null
  computed_at: string
  freshness?: PublicStatusFreshness
}

export type PublicStatusFreshness = 'fresh' | 'delayed' | 'unavailable'

export interface PublicStatusSummaryResponse {
  total_requests: number
  availability: {
    value_pct: number | null
    sample_count: number
    window_start: string | null
    window_end: string | null
  }
  ttft: {
    p50_ms: number | null
    p95_ms: number | null
    sample_count: number
    window_start: string | null
    window_end: string | null
  }
  data_through: string | null
  computed_at: string
  freshness: PublicStatusFreshness
}

export async function fetchPublicHomeStats(): Promise<PublicHomeStatsResponse | null> {
  try {
    const { data } = await apiClient.get<PublicHomeStatsResponse>('/public/home-stats', {
      timeout: 8000,
    })
    return data
  } catch {
    return null
  }
}

export async function fetchPublicStatusSummary(): Promise<PublicStatusSummaryResponse | null> {
  try {
    const { data } = await apiClient.get<PublicStatusSummaryResponse>('/public/status-summary', {
      timeout: 8000,
    })
    return data
  } catch {
    return null
  }
}
