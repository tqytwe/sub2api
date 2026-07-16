import { apiClient } from './client'

export interface PublicHomeStatsResponse {
  total_requests: number
  availability_pct: number | null
  avg_ttft_ms: number | null
  ops_data_through: string | null
  computed_at: string
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
