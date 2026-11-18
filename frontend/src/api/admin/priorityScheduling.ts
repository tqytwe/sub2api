import { apiClient } from '../client'

export type LatencyPenaltyMode = 'deprioritize' | 'stop'

export interface PrioritySchedulingConfig {
  enabled: boolean
  cache_weight?: number
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
  avg_tps?: number | null
  avg_ttft_ms?: number | null
  avg_ttft_samples?: number
  cache_percent?: number | null
  cache_samples?: number
}

export interface PrioritySnapshot {
  at: string
  model: string
  inbound_endpoint?: string
  group_id: number | null
  history_status?: string
  candidates: PriorityCandidate[]
}

interface PrioritySnapshotEnvelope {
  history?: PrioritySnapshot | null
  candidates?: PriorityCandidate[] | null
}

export async function getPriorityConfig(): Promise<PrioritySchedulingConfig> {
  return (await apiClient.get<PrioritySchedulingConfig>('/admin/priority-scheduling/config')).data
}

export async function savePriorityConfig(config: PrioritySchedulingConfig): Promise<PrioritySchedulingConfig> {
  return (await apiClient.put<PrioritySchedulingConfig>('/admin/priority-scheduling/config', config)).data
}

export async function getPrioritySnapshot(): Promise<PrioritySnapshot | null> {
  const { data } = await apiClient.get<{ snapshot: PrioritySnapshot | PrioritySnapshotEnvelope | null }>('/admin/priority-scheduling/snapshot')
  const payload = data.snapshot
  if (!payload) return null

  // The scheduler endpoint may return an envelope with the latest observation
  // nested under `history`. A cold scheduler has no observation yet; do not
  // mistake the envelope itself for a snapshot (it has no timestamp and its
  // candidate list may be null).
  const snapshot = 'history' in payload ? payload.history : payload
  if (!snapshot) return null

  // Keep older or partially populated server responses safe for table callers.
  return { ...snapshot, candidates: Array.isArray(snapshot.candidates) ? snapshot.candidates : [] } as PrioritySnapshot
}

export interface PriorityAnalysis {
  snapshot: PrioritySnapshot
  latency?: PriorityLatencySeries
}

export interface PriorityLatencyBucket {
  account_id: number
  bucket: string
  samples: number
  p50_ttft_ms: number
  p90_ttft_ms: number
  slow_samples: number
  error_samples?: number | null
}

export interface PriorityLatencySeries {
  at: string
  model: string
  group_id?: number | null
  bucket_minutes: number
  accounts: Array<{ account_id: number; account_name: string; samples: number; peak_bucket_p90_ttft_ms: number; slow_samples: number; error_samples?: number | null; buckets: PriorityLatencyBucket[] }>
}

export interface PrioritySchedulingDiffRow {
  account_id: number
  account_name: string
  old_rank: number
  new_rank: number
  old_score: number
  new_score: number
  old_tier?: string
  new_tier?: string
  old_samples: number
  new_samples: number
  changed: boolean
  reasons?: string[]
}

export interface PrioritySchedulingRecomputeResult {
  before: PrioritySnapshot
  after: PrioritySnapshot
  diff: PrioritySchedulingDiffRow[]
}

export interface StickyBindings {
  group_id: number
  total: number
  by_account: Array<{ account_id: number; count: number }>
}

export async function getPriorityModels(groupId?: number): Promise<string[]> {
  const { data } = await apiClient.get<{ models?: string[] }>('/admin/priority-scheduling/models', { params: groupId ? { group_id: groupId } : undefined })
  return Array.isArray(data.models) ? data.models : []
}

export async function getPriorityAnalysis(model: string, minutes = 60, groupId?: number): Promise<PriorityAnalysis> {
  return (await apiClient.get<PriorityAnalysis>('/admin/priority-scheduling/analysis', { params: { model, minutes, ...(groupId ? { group_id: groupId } : {}) } })).data
}

export async function getPriorityLatency(minutes = 60, bucketMinutes = 5): Promise<PriorityLatencySeries> {
  return (await apiClient.get<PriorityLatencySeries>('/admin/priority-scheduling/latency', { params: { minutes, bucket_minutes: bucketMinutes } })).data
}

export async function recomputePriorityScheduling(): Promise<PrioritySchedulingRecomputeResult> {
  return (await apiClient.post<PrioritySchedulingRecomputeResult>('/admin/priority-scheduling/recompute')).data
}

export async function getStickyBindings(groupId: number): Promise<StickyBindings> {
  return (await apiClient.get<StickyBindings>('/admin/priority-scheduling/sticky-bindings', { params: { group_id: groupId } })).data
}

export async function clearStickyBindings(groupId: number, accountIds?: number[], dryRun = false): Promise<{ deleted: number; remaining?: number }> {
  return (await apiClient.post('/admin/priority-scheduling/sticky-bindings/clear', { group_id: groupId, account_ids: accountIds, dry_run: dryRun })).data
}
