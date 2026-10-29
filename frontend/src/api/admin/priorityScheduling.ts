import { apiClient } from '../client'

export type LatencyPenaltyMode = 'deprioritize' | 'stop'

export interface PrioritySchedulingConfig {
  enabled: boolean
  latency_penalty_mode?: LatencyPenaltyMode
  latency_penalty_minutes?: number
  latency_percentile?: 50 | 70 | 90 | 95
  latency_measurement?: 'legacy_forward' | 'attributed_v1'
  [field: string]: unknown
}

export interface PriorityCandidate {
  account_id: number
  account_name: string
  tier: string
  latency_stopped?: boolean
  latency_penalty_until?: string
  latency_percentile?: number
  selected_ttft_ms?: number | null
  samples: number
  reasons: string[]
}

export interface PrioritySnapshot {
  at: string
  model: string
  group_id: number | null
  history_status?: string
  candidates: PriorityCandidate[]
}

export async function getPriorityConfig(): Promise<PrioritySchedulingConfig> {
  return (await apiClient.get<PrioritySchedulingConfig>('/admin/priority-scheduling/config')).data
}

export async function savePriorityConfig(config: PrioritySchedulingConfig): Promise<PrioritySchedulingConfig> {
  return (await apiClient.put<PrioritySchedulingConfig>('/admin/priority-scheduling/config', config)).data
}

export async function getPrioritySnapshot(): Promise<PrioritySnapshot | null> {
  const { data } = await apiClient.get<{ snapshot: PrioritySnapshot | null }>('/admin/priority-scheduling/snapshot')
  return data.snapshot
}
