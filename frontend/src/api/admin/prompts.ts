import { apiClient } from '../client'
import type {
  PromptDetail,
  PromptPagination,
  PromptReferenceRequirement,
  PromptSourceAttribution,
  PromptVariableDefinition,
} from '../prompts'
import {
  normalizePublicPrompt,
} from '../prompts'

export type AdminPromptStatus =
  | 'draft'
  | 'pending_review'
  | 'published'
  | 'offline'

export interface AdminPromptVersion {
  version: number
  status?: AdminPromptStatus
  created_at?: string
  created_by?: string
  change_summary?: string
}

export interface AdminPrompt extends PromptDetail {
  status: AdminPromptStatus
  source_evidence_summary?: string
  source_evidence_verified?: boolean
  source_evidence_captured_at?: string
  source_author?: string
  source_url?: string
  review_note?: string
  versions?: AdminPromptVersion[]
  published_at?: string
}

export interface AdminPromptDraft {
  title: string
  purpose_description: string
  prompt_template: string
  variables: PromptVariableDefinition[]
  preview_image_url: string
  recommended_models: string[]
  recommended_sizes: string[]
  reference_requirement: PromptReferenceRequirement
  reference_instructions: string
  source_attribution: PromptSourceAttribution
  source_evidence_summary: string
  source_evidence_verified: boolean
  source_evidence_captured_at: string
  source_author: string
  source_url: string
  featured: boolean
  purpose: string
  style: string
  subject: string
  content_notice: string
}

export interface AdminPromptListQuery {
  q?: string
  status?: AdminPromptStatus | ''
  page?: number
  page_size?: number
}

export interface AdminPromptPagination extends Omit<PromptPagination, 'items'> {
  items: AdminPrompt[]
}

export interface PromptImportJob {
  id: string
  status: string
  source_name?: string
  total_items?: number
  pending_items?: number
  created_at?: string
}

export interface PromptImportJobInput {
  source_key: string
  items: UnknownRecord[]
  raw_payload?: UnknownRecord
}

export interface PromptImportItem {
  id: string
  job_id?: string
  title: string
  prompt_template?: string
  source_attribution: PromptSourceAttribution
  source_evidence_summary?: string
  source_author?: string
  source_url?: string
  status: 'pending' | 'approved' | 'rejected' | string
  created_at?: string
}

export interface PromptReport {
  id: string
  prompt_id: string
  prompt_title?: string
  reason: string
  description?: string
  status: 'pending' | 'resolved' | 'dismissed' | string
  reporter_name?: string
  created_at?: string
}

export interface AdminPagination<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

type UnknownRecord = Record<string, unknown>

function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as UnknownRecord
    : {}
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function promptSource(value: unknown): PromptSourceAttribution {
  if (value === 'original') return 'original'
  if (value === 'authorized') return 'authorized'
  if (value === 'community') return 'community'
  return 'curated'
}

function evidenceValue(source: UnknownRecord, key: string): string {
  return text(record(source.evidence)[key])
}

export function normalizeAdminPrompt(value: unknown): AdminPrompt {
  const row = record(value)
  const base = normalizePublicPrompt(row)
  const sources = Array.isArray(row.sources) ? row.sources.map(record) : []
  const source = sources[0] ?? {}
  const reviews = Array.isArray(row.reviews) ? row.reviews.map(record) : []
  return {
    ...base,
    status: (text(row.status) || 'draft') as AdminPromptStatus,
    source_attribution: promptSource(row.brand_type),
    source_evidence_summary: evidenceValue(source, 'summary'),
    source_evidence_verified: row.source_evidence_verified === true || source.evidence_verified === true,
    source_evidence_captured_at: evidenceValue(source, 'captured_at'),
    source_author: text(source.original_author),
    source_url: text(source.source_url),
    review_note: text(reviews[0]?.note),
    published_at: text(row.published_at) || undefined,
    content_notice: text(row.public_attribution_note) || base.content_notice,
  }
}

function authorizationFor(source: PromptSourceAttribution): PromptSourceAttribution {
  return source
}

export function toAdminPromptRequest(body: AdminPromptDraft): UnknownRecord {
  const variables = Object.fromEntries(body.variables.map((variable) => [
    variable.name,
    {
      label: variable.label,
      description: variable.description,
      type: variable.type,
      required: variable.required,
      default_value: variable.default_value,
      options: variable.options,
    },
  ]))
  const evidence = {
    summary: body.source_evidence_summary.trim(),
    captured_at: body.source_evidence_captured_at,
  }
  const hasSourceRecord = !!(
    body.source_url.trim()
    || body.source_author.trim()
    || body.source_evidence_summary.trim()
  )
  return {
    brand_type: body.source_attribution,
    provenance_type: body.source_attribution === 'original'
      ? 'internal'
      : body.source_attribution === 'community' ? 'community' : 'external',
    authorization_status: authorizationFor(body.source_attribution),
    source_evidence_verified: body.source_evidence_verified,
    title_zh: body.title,
    description_zh: body.purpose_description,
    purpose: body.purpose,
    style: body.style,
    subject: body.subject,
    featured: body.featured,
    prompt_text: body.prompt_template,
    variables,
    models: body.recommended_models,
    sizes: body.recommended_sizes,
    reference_requirement: body.reference_requirement,
    reference_instructions: body.reference_instructions,
    requires_reference: body.reference_requirement === 'required',
    public_attribution_note: body.content_notice,
    media: body.preview_image_url
      ? [{ media_type: 'image', url: body.preview_image_url, alt_zh: `${body.title}示例效果` }]
      : [],
    sources: hasSourceRecord ? [{
      source_key: 'admin-manual',
      source_url: body.source_url,
      original_author: body.source_author,
      evidence,
      authorization_status: authorizationFor(body.source_attribution),
      evidence_verified: body.source_evidence_verified,
    }] : [],
  }
}

function normalizePagination<T>(
  value: unknown,
  normalize: (item: unknown) => T,
): AdminPagination<T> {
  const data = record(value)
  const items = Array.isArray(data.items) ? data.items.map(normalize) : []
  return {
    items,
    total: Number(data.total) || 0,
    page: Number(data.page) || 1,
    page_size: Number(data.page_size) || 20,
    pages: Number(data.pages) || (items.length ? 1 : 0),
  }
}

export async function listAdminPrompts(query: AdminPromptListQuery = {}): Promise<AdminPromptPagination> {
  const { data } = await apiClient.get<unknown>('/admin/prompts', { params: query })
  return normalizePagination(data, normalizeAdminPrompt)
}

export async function getAdminPrompt(id: string): Promise<AdminPrompt> {
  const { data } = await apiClient.get<unknown>(`/admin/prompts/${encodeURIComponent(id)}`)
  return normalizeAdminPrompt(data)
}

export async function createAdminPrompt(body: AdminPromptDraft): Promise<AdminPrompt> {
  const { data } = await apiClient.post<unknown>('/admin/prompts', toAdminPromptRequest(body))
  return normalizeAdminPrompt(data)
}

export async function updateAdminPrompt(
  id: string,
  body: AdminPromptDraft,
  expectedVersion?: number,
): Promise<AdminPrompt> {
  const payload = toAdminPromptRequest(body)
  if (expectedVersion && expectedVersion > 0) {
    payload.expected_version = expectedVersion
    payload.current_version = expectedVersion
  }
  const { data } = await apiClient.put<unknown>(
    `/admin/prompts/${encodeURIComponent(id)}`,
    payload,
  )
  return normalizeAdminPrompt(data)
}

async function postPromptAction(id: string, action: string, body?: Record<string, unknown>): Promise<AdminPrompt> {
  const { data } = await apiClient.post<unknown>(
    `/admin/prompts/${encodeURIComponent(id)}/${action}`,
    body,
  )
  return normalizeAdminPrompt(data)
}

export const submitPromptReview = (id: string) => postPromptAction(id, 'submit-review')
export const approvePrompt = (id: string, review_note?: string) =>
  postPromptAction(id, 'approve', review_note ? { note: review_note } : undefined)
export const unpublishPrompt = (id: string) => postPromptAction(id, 'offline')
export const rollbackPrompt = (id: string, version: number) =>
  postPromptAction(id, 'rollback', { version })

export async function listImportJobs(query: { page?: number; page_size?: number } = {}): Promise<AdminPagination<PromptImportJob>> {
  const { data } = await apiClient.get<unknown>('/admin/prompts/import-jobs', {
    params: query,
  })
  return normalizePagination(data, (value) => {
    const row = record(value)
    return {
      id: String(row.id ?? ''),
      status: text(row.status),
      source_name: text(row.source_key),
      total_items: Number(row.item_count) || 0,
      pending_items: Number(row.item_count) || 0,
      created_at: text(row.created_at),
    }
  })
}

export async function createPromptImportJob(input: PromptImportJobInput): Promise<PromptImportJob> {
  const { data } = await apiClient.post<UnknownRecord>('/admin/prompts/import-jobs', input)
  return {
    id: String(data.id ?? ''),
    status: text(data.status),
    source_name: text(data.source_key),
    total_items: Number(data.item_count) || 0,
    pending_items: Number(data.item_count) || 0,
    created_at: text(data.created_at),
  }
}

export async function listImportItems(query: {
  status?: string
  page?: number
  page_size?: number
} = {}): Promise<AdminPagination<PromptImportItem>> {
  const { data } = await apiClient.get<unknown>('/admin/prompts/import-items', {
    params: query,
  })
  return normalizePagination(data, (value) => {
    const row = record(value)
    const payload = record(row.normalized_payload)
    const sourcePayload = record(payload.source_payload)
    const evidence = record(payload.evidence)
    return {
      id: String(row.id ?? ''),
      job_id: String(row.job_id ?? ''),
      title: text(payload.title_zh),
      prompt_template: text(payload.prompt_text),
      source_attribution: promptSource(payload.brand_type),
      source_evidence_summary: text(evidence.summary),
      source_author: text(payload.original_author) || text(sourcePayload.original_author),
      source_url: text(payload.source_url),
      status: text(row.status),
      created_at: text(row.created_at),
    }
  })
}

export async function approveImportItem(id: string): Promise<void> {
  await apiClient.post(`/admin/prompts/import-items/${encodeURIComponent(id)}/approve`)
}

export async function rejectImportItem(id: string, reason: string): Promise<void> {
  await apiClient.post(`/admin/prompts/import-items/${encodeURIComponent(id)}/reject`, { reason })
}

export async function listReports(query: {
  status?: string
  page?: number
  page_size?: number
} = {}): Promise<AdminPagination<PromptReport>> {
  const { data } = await apiClient.get<unknown>('/admin/prompts/reports', {
    params: query,
  })
  return normalizePagination(data, (value) => {
    const row = record(value)
    return {
      id: String(row.id ?? ''),
      prompt_id: String(row.prompt_id ?? ''),
      reason: text(row.reason),
      description: text(row.detail),
      status: text(row.status),
      created_at: text(row.created_at),
    }
  })
}

export async function resolvePromptReport(
  id: string,
  resolution: 'resolved' | 'dismissed',
  note: string,
): Promise<void> {
  await apiClient.post(`/admin/prompts/reports/${encodeURIComponent(id)}/resolve`, {
    status: resolution,
    resolution: note,
  })
}
