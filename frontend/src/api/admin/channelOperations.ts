import { apiClient } from '../client'

export interface ChannelOperationUpstream {
  id: number
  name: string
  members: number
}

export interface ChannelOperationAccount {
  id: number
  name: string
  platform: string
  status: string
  schedulable: boolean
  concurrency: number
  upstream_id: number
  eligible: boolean
}

export interface ChannelOperationsCatalog {
  revision: number
  upstreams: ChannelOperationUpstream[]
  upstream_total: number
  accounts: ChannelOperationAccount[]
  total: number
}

export interface ChannelOperationsQuery {
  page: number
  page_size: number
  search: string
  upstream_page: number
  upstream_search: string
  upstream_id: number
  membership: 'all' | 'assigned' | 'unassigned'
}

export interface ChannelOperationsMutation {
  action: 'create_upstream' | 'update_upstream' | 'delete_upstream' | 'assign' | 'detach'
  revision: number
  upstream_id?: number
  account_id?: number
  expected_upstream_id?: number
  name?: string
  confirm?: boolean
}

export interface OperationsMetrics {
  requests: number
  samples: number
  unknown_samples: number
  duration_samples: number
  p50_ms: number | null
  p90_ms: number | null
  p95_ms: number | null
  duration_p90_ms: number | null
  latest_sample_at: string | null
}

export interface OperationsUsageRow extends OperationsMetrics { dimension_id: number; name: string }
export interface OperationsLineage {
  public_model: string
  recorded_model: string
  upstream_model: string | null
  model_mapping_chain: string | null
  samples: number
}
export interface OperationsUsage {
  source: string
  attribution: string
  latency_measurement: string
  summary: OperationsMetrics
  total: number
  rows: OperationsUsageRow[]
  lineage: OperationsLineage[]
  lineage_total: number
  error_rate: null
  error_scope: string
}
export interface OperationsQualityRound {
  id: number
  account_id: number
  plan_id: number
  measured_model: string | null
  observed_at: string
  probes: number
  passed_probes: number
  expected: number | null
  outcome: 'passed' | 'degraded' | 'inconclusive'
  round_complete: boolean
  round_probes: number
  round_expected: number | null
  actions: Array<string | null>
}
export interface OperationsQuality {
  source: string
  scope: string
  summary: { rounds: number; passed: number; degraded: number; inconclusive: number; latest_sample_at: string | null }
  rows: OperationsQualityRound[]
  has_more: boolean
}
export interface OperationsErrors {
  source: string
  scope: string
  records: number
  upstream_5xx: number
  latest_sample_at: string | null
  rows: Array<{ error_type: string; error_phase: string; status_code: number; records: number }>
  bucket_total: number
  error_rate: null
}
export interface OperationsCapacity {
  source: string
  scope: string
  accounts: number
  schedulable: number
  configured_concurrency: number
}
export interface OperationsSection<T> {
  status: 'ready' | 'query_failed' | 'unsupported_scope'
  data: T | null
  error?: string
  queried_at: string
}
export interface ChannelOperationsSnapshot {
  scope: { channel_id: number; upstream_id: number; account_id: number; group_id: number; model: string; model_scope: string; dimension: string }
  usage: OperationsSection<OperationsUsage>
  quality: OperationsSection<OperationsQuality>
  errors: OperationsSection<OperationsErrors>
  capacity: OperationsSection<OperationsCapacity>
  from: string
  until: string
  model_scope: string
}

export interface ChannelOperationsSnapshotQuery {
  channel_id: number
  upstream_id: number
  account_id: number
  group_id: number
  from: string
  until: string
  model: string
  model_scope: 'public' | 'recorded' | 'upstream'
  dimension: 'channel' | 'upstream' | 'account'
  page: number
  page_size: number
  quality_before: number
}

export async function getCatalog(
  channelId: number,
  params: ChannelOperationsQuery,
  signal?: AbortSignal
): Promise<ChannelOperationsCatalog> {
  const { data } = await apiClient.get<ChannelOperationsCatalog>(
    `/admin/channels/${channelId}/operations`,
    { params, signal }
  )
  return data
}

export async function mutate(
  channelId: number,
  body: ChannelOperationsMutation
): Promise<{ revision: number }> {
  const { data } = await apiClient.post<{ revision: number }>(
    `/admin/channels/${channelId}/operations`,
    body
  )
  return data
}

export async function getSnapshot(params: ChannelOperationsSnapshotQuery, signal?: AbortSignal): Promise<ChannelOperationsSnapshot> {
  const { data } = await apiClient.get<ChannelOperationsSnapshot>('/admin/channels/operations/snapshot', { params, signal })
  return data
}
