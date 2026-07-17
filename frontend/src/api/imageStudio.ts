import { apiClient } from './client'
import type { PlayQuestToday } from './play'
import { extensionForContentType, filenameForAsset, saveBlob } from '@/utils/imageStudioBlob'

export interface ImageStudioLocalizedText {
  zh: string
  en: string
}

export interface ImageStudioTemplate {
  id: string
  label: ImageStudioLocalizedText
  description?: ImageStudioLocalizedText
  defaults: { size: string; count: number }
  compliance_hints?: string[]
  preview_emoji?: string
  preview_url?: string
}

export interface ImageStudioIntent {
  id: string
  label: ImageStudioLocalizedText
  templates: ImageStudioTemplate[]
}

export interface ImageStudioCatalog {
  intents: ImageStudioIntent[]
}

export interface ImageStudioAspectOption {
  id: string
  label: ImageStudioLocalizedText
}

export interface ImageStudioTierOption {
  id: string
  label: ImageStudioLocalizedText
}

export interface ImageStudioSizeOption {
  aspect: string
  tier: string
  size: string
  billing_tier: string
}

export interface ImageStudioCapabilities {
  aspects: ImageStudioAspectOption[]
  tiers: ImageStudioTierOption[]
  size_options: ImageStudioSizeOption[]
}

export interface ImageStudioEstimate {
  estimated_cost: number
  balance: number
  sufficient: boolean
  model: string
  count: number
  size: string
}

export interface ImageStudioModelOption {
  id: string
  display_name: string
  supported_sizes?: string[]
  supported_qualities?: string[]
  default_size?: string
  default_quality?: string
}

export interface ImageStudioAsset {
  id: string
  url?: string
  sort_order: number
  content_type?: string
  byte_size?: number
  preview_url?: string
  download_url?: string
}

export interface ImageStudioJob {
  id: string
  template_id: string
  prompt_id?: number
  prompt_version?: number
  model?: string
  quality?: string
  size: string
  count: number
  status: 'pending' | 'running' | 'completed' | 'failed' | string
  estimated_cost: number
  actual_cost?: number
  error_message?: string
  created_at: string
  assets?: ImageStudioAsset[]
}

export interface ImageStudioGenerateRequest {
  template_id: string
  prompt_id?: number
  prompt_version?: number
  user_prompt: string
  accent_color?: string
  size?: string
  aspect?: string
  tier?: string
  quality?: string
  count?: number
  model?: string
  expert_prompt?: string | null
  api_key_id: number
  retain_days?: number
}

export interface ImageStudioGenerateResult {
  job: ImageStudioJob
  async?: boolean
  poll?: string
  quest_progress?: PlayQuestToday
}

export async function getImageStudioTemplates(): Promise<ImageStudioCatalog> {
  const { data } = await apiClient.get<ImageStudioCatalog>('/image-studio/templates')
  return data
}

export async function getImageStudioCapabilities(): Promise<ImageStudioCapabilities> {
  const { data } = await apiClient.get<ImageStudioCapabilities>('/image-studio/capabilities')
  return data
}

export async function listImageStudioModels(apiKeyId: number): Promise<ImageStudioModelOption[]> {
  const { data } = await apiClient.get<{ models: ImageStudioModelOption[] }>('/image-studio/models', {
    params: { api_key_id: apiKeyId },
  })
  return data.models ?? []
}

export async function estimateImageStudio(params: {
  template_id: string
  size?: string
  count?: number
  api_key_id?: number
  model?: string
}): Promise<ImageStudioEstimate> {
  const { data } = await apiClient.get<ImageStudioEstimate>('/image-studio/estimate', { params })
  return data
}

export async function generateImageStudio(body: ImageStudioGenerateRequest): Promise<ImageStudioGenerateResult> {
  const { data } = await apiClient.post<ImageStudioGenerateResult>('/image-studio/generate', body)
  return data
}

export async function getImageStudioJob(id: string): Promise<ImageStudioJob> {
  const { data } = await apiClient.get<ImageStudioJob>(`/image-studio/jobs/${id}`)
  return data
}

export async function getActiveImageStudioJob(): Promise<ImageStudioJob | null> {
  const { data } = await apiClient.get<{ job: ImageStudioJob | null }>('/image-studio/jobs/active')
  return data.job ?? null
}

export async function pollImageStudioJob(
  id: string,
  opts?: { intervalMs?: number; timeoutMs?: number; signal?: AbortSignal },
): Promise<ImageStudioJob> {
  const intervalMs = opts?.intervalMs ?? 2000
  const timeoutMs = opts?.timeoutMs ?? 180000
  const start = Date.now()
  for (;;) {
    if (opts?.signal?.aborted) {
      throw new Error('IMAGE_STUDIO_POLL_ABORTED')
    }
    const job = await getImageStudioJob(id)
    if (job.status === 'completed' || job.status === 'failed') {
      return job
    }
    if (Date.now() - start > timeoutMs) {
      throw new Error('IMAGE_STUDIO_POLL_TIMEOUT')
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
}

export async function listImageStudioJobs(limit = 20): Promise<ImageStudioJob[]> {
  const { data } = await apiClient.get<{ jobs: ImageStudioJob[] }>('/image-studio/jobs', { params: { limit } })
  return data.jobs ?? []
}

export async function deleteImageStudioJob(id: string): Promise<void> {
  await apiClient.delete(`/image-studio/jobs/${id}`)
}

export async function fetchImageStudioAssetBlob(assetId: string, mode: 'content' | 'download' = 'content'): Promise<Blob> {
  const path = `/image-studio/assets/${encodeURIComponent(assetId)}/${mode}`
  const response = await apiClient.get(path, { responseType: 'blob' })
  const blob = response.data as Blob
  const ctype = String(blob?.type || '').toLowerCase()
  if (!blob || blob.size === 0) {
    throw new Error('empty asset')
  }
  // Axios may surface JSON API errors as Blob when responseType is blob.
  if (ctype.includes('json') || ctype.includes('text')) {
    const text = await blob.text()
    throw new Error(text || 'asset fetch failed')
  }
  return blob
}

export async function downloadImageStudioAsset(
  asset: ImageStudioAsset,
  jobId: string,
  index: number,
): Promise<void> {
  const blob = await fetchImageStudioAssetBlob(asset.id, 'download')
  const filename = filenameForAsset(jobId, index, blob.type || asset.content_type)
  saveBlob(blob, filename)
}

export { extensionForContentType, filenameForAsset, saveBlob }

export const imageStudioAPI = {
  getImageStudioTemplates,
  getImageStudioCapabilities,
  listImageStudioModels,
  estimateImageStudio,
  generateImageStudio,
  getImageStudioJob,
  getActiveImageStudioJob,
  pollImageStudioJob,
  listImageStudioJobs,
  deleteImageStudioJob,
  fetchImageStudioAssetBlob,
  downloadImageStudioAsset,
}

export default imageStudioAPI
