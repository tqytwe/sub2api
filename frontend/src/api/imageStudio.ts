import { apiClient } from './client'
import type { PlayQuestToday } from './play'

export interface ImageStudioLocalizedText {
  zh: string
  en: string
}

export interface ImageStudioTemplate {
  id: string
  label: ImageStudioLocalizedText
  defaults: { size: string; count: number }
  compliance_hints?: string[]
  preview_emoji?: string
}

export interface ImageStudioIntent {
  id: string
  label: ImageStudioLocalizedText
  templates: ImageStudioTemplate[]
}

export interface ImageStudioCatalog {
  intents: ImageStudioIntent[]
}

export interface ImageStudioEstimate {
  estimated_cost: number
  balance: number
  sufficient: boolean
  model: string
  count: number
  size: string
}

export interface ImageStudioAsset {
  id: string
  url: string
  sort_order: number
}

export interface ImageStudioJob {
  id: string
  template_id: string
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
  user_prompt: string
  accent_color?: string
  size?: string
  count?: number
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

export async function estimateImageStudio(params: {
  template_id: string
  size?: string
  count?: number
  api_key_id?: number
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

export async function pollImageStudioJob(
  id: string,
  opts?: { intervalMs?: number; timeoutMs?: number },
): Promise<ImageStudioJob> {
  const intervalMs = opts?.intervalMs ?? 2000
  const timeoutMs = opts?.timeoutMs ?? 120000
  const start = Date.now()
  for (;;) {
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

export const imageStudioAPI = {
  getImageStudioTemplates,
  estimateImageStudio,
  generateImageStudio,
  getImageStudioJob,
  pollImageStudioJob,
  listImageStudioJobs,
  deleteImageStudioJob,
}

export default imageStudioAPI
