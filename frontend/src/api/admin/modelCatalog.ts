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
  official_input_price: number | null
  official_output_price: number | null
  official_cache_read_price: number | null
  official_cache_write_price: number | null
  official_source: string
  official_updated_at: string | null
  price_multiplier: number | null
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

export type AdminCatalogRow = SiteModelCatalogEntry

export interface ModelDiscovery {
  id: number
  model_name: string
  platform: string
  source: string
  payload: Record<string, unknown>
  status: string
  discovered_at: string
}

export interface DiscoveryListResult {
  items: ModelDiscovery[]
  total: number
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
}): Promise<AdminCatalogRow[]> {
  const { data } = await apiClient.get<AdminCatalogRow[]>('/admin/model-catalog', { params })
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

export async function listDiscoveries(params?: {
  status?: string
  search?: string
  limit?: number
  offset?: number
}): Promise<DiscoveryListResult> {
  const { data } = await apiClient.get<DiscoveryListResult>('/admin/model-catalog/discoveries', { params })
  return data ?? { items: [], total: 0 }
}

export async function importDiscoveries(payload: { ids: number[]; to_catalog?: boolean; site_multiplier?: number }): Promise<number> {
  const { data } = await apiClient.post<{ imported: number }>('/admin/model-catalog/discoveries/import', {
    to_catalog: true,
    ...payload,
  })
  return data?.imported ?? 0
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
}

export default adminModelCatalogAPI
