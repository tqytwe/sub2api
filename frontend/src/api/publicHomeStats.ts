import { apiClient } from './client'

export interface PublicHomeStatsResponse {
	snapshot_id: string
	updated_at: string
	source: 'live' | 'estimated' | 'demo'
	requests_24h: number
	requests_total: number
	active_users_7d: number
	tokens_total: number
	success_rate_30d: number | null
	p50_ttft_ms: number | null
	p95_ttft_ms: number | null
}

export interface PublicActivityItem {
	id: number
	event_type: string
	actor: string
	payload: Record<string, unknown>
	created_at: string
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

export async function fetchPublicActivity(limit = 12): Promise<PublicActivityItem[]> {
	try {
		const { data } = await apiClient.get<PublicActivityItem[]>('/public/activity', { params: { limit } })
		return data ?? []
	} catch {
		return []
	}
}
