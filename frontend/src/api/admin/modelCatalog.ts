import { apiClient } from '../client'

export interface SiteModelCatalogEntry {
  id: number
  model_name: string
  platform: string
  display_name: string | null
  use_case: string | null
  sort_order: number
  visible_public: boolean
  visible_auth: boolean
  featured: boolean
  input_price: number | null
  output_price: number | null
  cache_read_price: number | null
  cache_write_price: number | null
  billing_mode: string
  source: string
  source_updated_at: string | null
  created_at: string
  updated_at: string
}

export interface ModelDiscovery {
  id: number
  model_name: string
  platform: string
  source: string
  payload: Record<string, unknown>
  status: string
  discovered_at: string
}

export interface ModelSyncJob {
  id: string
  kind: string
  status: string
  result?: {
    updated?: number
    discovered?: number
    retired?: number
    warnings?: string[]
    source?: string
  }
  error?: string
  started_at: string
  completed_at?: string
}

export async function listCatalog(params?: {
  platform?: string
  search?: string
  visible_public?: boolean
}): Promise<SiteModelCatalogEntry[]> {
  const { data } = await apiClient.get<SiteModelCatalogEntry[]>('/admin/model-catalog', { params })
  return data ?? []
}

export async function saveCatalogEntry(entry: Partial<SiteModelCatalogEntry>): Promise<SiteModelCatalogEntry> {
  const { data } = await apiClient.put<SiteModelCatalogEntry>('/admin/model-catalog', entry)
  return data
}

export async function deleteCatalogEntry(id: number): Promise<void> {
  await apiClient.delete(`/admin/model-catalog/${id}`)
}

export async function batchVisibility(payload: {
  ids: number[]
  visible_public?: boolean
  visible_auth?: boolean
}): Promise<number> {
  const { data } = await apiClient.post<{ updated: number }>('/admin/model-catalog/batch-visibility', payload)
  return data?.updated ?? 0
}

export async function batchPrices(payload: {
  ids: number[]
  multiplier?: number
  input_price?: number
  output_price?: number
}): Promise<number> {
  const { data } = await apiClient.post<{ updated: number }>('/admin/model-catalog/batch-prices', payload)
  return data?.updated ?? 0
}

export async function createSyncJob(): Promise<ModelSyncJob> {
  const { data } = await apiClient.post<ModelSyncJob>('/admin/model-catalog/sync-jobs')
  return data
}

export async function getSyncJob(id: string): Promise<ModelSyncJob> {
  const { data } = await apiClient.get<ModelSyncJob>(`/admin/model-catalog/sync-jobs/${id}`)
  return data
}

export async function listDiscoveries(status = 'new'): Promise<ModelDiscovery[]> {
  const { data } = await apiClient.get<ModelDiscovery[]>('/admin/model-catalog/discoveries', {
    params: { status },
  })
  return data ?? []
}

export async function importDiscoveries(payload: { ids?: number[]; to_catalog?: boolean }): Promise<number> {
  const { data } = await apiClient.post<{ imported: number }>('/admin/model-catalog/discoveries/import', {
    to_catalog: true,
    ...payload,
  })
  return data?.imported ?? 0
}

export async function fillFromLiteLLM(ids?: number[]): Promise<number> {
  const { data } = await apiClient.post<{ updated: number }>('/admin/model-catalog/fill-litellm', { ids: ids ?? [] })
  return data?.updated ?? 0
}

export const adminModelCatalogAPI = {
  listCatalog,
  saveCatalogEntry,
  deleteCatalogEntry,
  batchVisibility,
  batchPrices,
  createSyncJob,
  getSyncJob,
  listDiscoveries,
  importDiscoveries,
  fillFromLiteLLM,
}

export default adminModelCatalogAPI
