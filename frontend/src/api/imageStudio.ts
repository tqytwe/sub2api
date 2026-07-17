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
  platform?: 'openai' | 'grok' | (string & {})
  capability_profile_id?: string
  capability_revision?: string
  operations?: Array<'create' | 'edit' | (string & {})>
  sizing_kind?: 'fixed' | 'custom' | 'aspect_resolution' | (string & {})
  supported_sizes?: string[]
  supported_aspect_ratios?: string[]
  supported_resolutions?: string[]
  supported_qualities?: string[]
  supported_backgrounds?: string[]
  supported_output_formats?: string[]
  supported_input_fidelities?: string[]
  input_fidelity_mode?: 'selectable' | 'fixed' | (string & {})
  supports_transparency?: boolean
  output_compression?: {
    min: number
    max: number
    formats: string[]
  }
  max_reference_images?: number
  default_size?: string
  default_aspect_ratio?: string
  default_resolution?: string
  default_quality?: string
  default_background?: string
  default_output_format?: string
  default_input_fidelity?: string
}

export interface ImageStudioAsset {
  id: string
  url?: string
  sort_order: number
  content_type?: string
  byte_size?: number
  width?: number
  height?: number
  aspect_ratio?: string
  thumbnail_url?: string
  preview_url?: string
  download_url?: string
}

export interface ImageStudioReference {
  id: string
  filename?: string
  content_type: string
  byte_size: number
  expires_at: string
}

export type ImageStudioJobStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'partial'
  | 'failed'
  | 'cancelled'
  | (string & {})

export interface ImageStudioJobItem {
  id?: string
  status?: string
  error?: string
  error_message?: string
  asset_id?: string
  asset?: ImageStudioAsset
  assets?: ImageStudioAsset[]
}

export interface ImageStudioJob {
  id: string
  template_id: string
  size: string
  count: number
  status: ImageStudioJobStatus
  estimated_cost: number
  actual_cost?: number
  error_message?: string
  success_count?: number
  fail_count?: number
  items?: ImageStudioJobItem[]
  created_at: string
  assets?: ImageStudioAsset[]
}

export interface ImageStudioJobPage {
  jobs: ImageStudioJob[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ImageStudioGenerateRequest {
  template_id: string
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
  mode?: 'create' | 'edit'
  reference_ids?: string[]
  background?: string
  output_format?: string
  output_compression?: number
  input_fidelity?: string
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
  reference_ids?: string[]
}): Promise<ImageStudioEstimate> {
  const { data } = await apiClient.get<ImageStudioEstimate>('/image-studio/estimate', { params })
  return data
}

export async function generateImageStudio(
  body: ImageStudioGenerateRequest,
  idempotencyKey: string,
): Promise<ImageStudioGenerateResult> {
  const { data } = await apiClient.post<ImageStudioGenerateResult>(
    '/image-studio/generate',
    body,
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
  return data
}

export async function uploadImageStudioReference(
  file: File,
  signal?: AbortSignal,
): Promise<ImageStudioReference> {
  const body = new FormData()
  body.append('image', file)
  const { data } = await apiClient.post<{ reference: ImageStudioReference }>(
    '/image-studio/references',
    body,
    signal ? { signal } : undefined,
  )
  return data.reference
}

export async function deleteImageStudioReference(id: string): Promise<void> {
  await apiClient.delete(`/image-studio/references/${encodeURIComponent(id)}`)
}

export async function getImageStudioJob(
  id: string,
  signal?: AbortSignal,
): Promise<ImageStudioJob> {
  const { data } = await apiClient.get<ImageStudioJob>(
    `/image-studio/jobs/${id}`,
    signal ? { signal } : undefined,
  )
  return data
}

export function isImageStudioJobActive(job: Pick<ImageStudioJob, 'status'>): boolean {
  return job.status === 'pending' || job.status === 'running'
}

export function isImageStudioJobTerminal(job: Pick<ImageStudioJob, 'status'>): boolean {
  return !isImageStudioJobActive(job)
}

function normalizeImageStudioJobs(
  data: ImageStudioJob[] | { jobs?: ImageStudioJob[]; job?: ImageStudioJob | null } | null | undefined,
): ImageStudioJob[] {
  if (Array.isArray(data)) return data
  const jobs = Array.isArray(data?.jobs) ? data.jobs : []
  if (jobs.length > 0) return jobs
  return data?.job ? [data.job] : []
}

function positiveInteger(value: unknown, fallback: number): number {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

function nonNegativeInteger(value: unknown, fallback: number): number {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : fallback
}

function normalizeImageStudioJobPage(
  data: ImageStudioJob[] | Partial<ImageStudioJobPage> | null | undefined,
  requestedPage: number,
  requestedPageSize: number,
): ImageStudioJobPage {
  const jobs = normalizeImageStudioJobs(data)
  const page = positiveInteger(Array.isArray(data) ? undefined : data?.page, requestedPage)
  const pageSize = positiveInteger(
    Array.isArray(data) ? undefined : data?.page_size,
    requestedPageSize,
  )
  const total = nonNegativeInteger(
    Array.isArray(data) ? undefined : data?.total,
    jobs.length,
  )
  const fallbackPages = total === 0 ? 0 : Math.ceil(total / pageSize)
  const pages = nonNegativeInteger(
    Array.isArray(data) ? undefined : data?.pages,
    fallbackPages,
  )
  return { jobs, total, page, page_size: pageSize, pages }
}

export async function getActiveImageStudioJobs(): Promise<ImageStudioJob[]> {
  const { data } = await apiClient.get<{
    jobs?: ImageStudioJob[]
    job?: ImageStudioJob | null
  }>('/image-studio/jobs/active')
  return normalizeImageStudioJobs(data).filter(isImageStudioJobActive)
}

export async function getActiveImageStudioJob(): Promise<ImageStudioJob | null> {
  return (await getActiveImageStudioJobs())[0] ?? null
}

function imageStudioPollAbortError(): Error {
  return new Error('IMAGE_STUDIO_POLL_ABORTED')
}

function throwIfImageStudioPollAborted(signal?: AbortSignal) {
  if (signal?.aborted) {
    throw imageStudioPollAbortError()
  }
}

function waitForImageStudioPoll(intervalMs: number, signal?: AbortSignal): Promise<void> {
  throwIfImageStudioPollAborted(signal)
  return new Promise((resolve, reject) => {
    const onAbort = () => {
      clearTimeout(timer)
      reject(imageStudioPollAbortError())
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, intervalMs)
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

export async function pollImageStudioJob(
  id: string,
  opts?: { intervalMs?: number; timeoutMs?: number; signal?: AbortSignal },
): Promise<ImageStudioJob> {
  const intervalMs = opts?.intervalMs ?? 2000
  const timeoutMs = opts?.timeoutMs ?? 180000
  const start = Date.now()
  for (;;) {
    throwIfImageStudioPollAborted(opts?.signal)
    let job: ImageStudioJob
    try {
      job = await getImageStudioJob(id, opts?.signal)
    } catch (error) {
      throwIfImageStudioPollAborted(opts?.signal)
      throw error
    }
    throwIfImageStudioPollAborted(opts?.signal)
    if (isImageStudioJobTerminal(job)) {
      return job
    }
    if (Date.now() - start > timeoutMs) {
      throw new Error('IMAGE_STUDIO_POLL_TIMEOUT')
    }
    await waitForImageStudioPoll(intervalMs, opts?.signal)
  }
}

export async function listImageStudioJobs(
  page = 1,
  pageSize = 12,
): Promise<ImageStudioJobPage> {
  const { data } = await apiClient.get<ImageStudioJob[] | Partial<ImageStudioJobPage>>(
    '/image-studio/jobs',
    { params: { page, page_size: pageSize } },
  )
  return normalizeImageStudioJobPage(data, page, pageSize)
}

export async function cancelImageStudioJob(id: string): Promise<ImageStudioJob> {
  const { data } = await apiClient.post<ImageStudioJob>(
    `/image-studio/jobs/${encodeURIComponent(id)}/cancel`,
  )
  return data
}

export async function deleteImageStudioJob(id: string): Promise<void> {
  await apiClient.delete(`/image-studio/jobs/${id}`)
}

async function validateImageStudioBlob(responseData: unknown, emptyMessage: string): Promise<Blob> {
  const blob = responseData as Blob
  const ctype = String(blob?.type || '').toLowerCase()
  if (!blob || blob.size === 0) {
    throw new Error(emptyMessage)
  }
  // Axios may surface JSON API errors as Blob when responseType is blob.
  if (ctype.includes('json') || ctype.includes('text')) {
    const text = await blob.text()
    throw new Error(text || emptyMessage)
  }
  return blob
}

export async function fetchImageStudioAssetBlob(
  assetId: string,
  mode: 'thumbnail' | 'content' | 'download' = 'content',
  signal?: AbortSignal,
): Promise<Blob> {
  const path = `/image-studio/assets/${encodeURIComponent(assetId)}/${mode}`
  const response = await apiClient.get(path, {
    responseType: 'blob',
    ...(signal ? { signal } : {}),
  })
  return validateImageStudioBlob(response.data, 'asset fetch failed')
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

export async function downloadImageStudioJob(id: string): Promise<void> {
  const response = await apiClient.get(
    `/image-studio/jobs/${encodeURIComponent(id)}/download`,
    { responseType: 'blob' },
  )
  const blob = await validateImageStudioBlob(response.data, 'job download failed')
  saveBlob(blob, `image-studio-${id.slice(0, 8)}.zip`)
}

export { extensionForContentType, filenameForAsset, saveBlob }

export const imageStudioAPI = {
  getImageStudioTemplates,
  getImageStudioCapabilities,
  listImageStudioModels,
  estimateImageStudio,
  generateImageStudio,
  uploadImageStudioReference,
  deleteImageStudioReference,
  getImageStudioJob,
  getActiveImageStudioJobs,
  getActiveImageStudioJob,
  pollImageStudioJob,
  listImageStudioJobs,
  cancelImageStudioJob,
  deleteImageStudioJob,
  fetchImageStudioAssetBlob,
  downloadImageStudioAsset,
  downloadImageStudioJob,
}

export default imageStudioAPI
