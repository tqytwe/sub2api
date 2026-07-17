import type { LocationQuery, LocationQueryRaw, Router } from 'vue-router'
import type {
  PromptReferenceRequirement,
  PromptSourceAttribution,
  PromptSort,
  PromptUseResult,
} from '@/api/prompts'
import type { AdminPromptDraft, AdminPromptStatus } from '@/api/admin/prompts'

export interface PromptFiltersState {
  q: string
  purpose: string
  style: string
  subject: string
  model: string
  size: string
  reference: PromptReferenceRequirement | ''
  featured: boolean
  favorite: boolean
  sort: PromptSort
  page: number
}

export const DEFAULT_PROMPT_FILTERS: PromptFiltersState = {
  q: '',
  purpose: '',
  style: '',
  subject: '',
  model: '',
  size: '',
  reference: '',
  featured: false,
  favorite: false,
  sort: 'featured',
  page: 1,
}

function firstQueryValue(value: LocationQuery[string] | unknown): string {
  if (Array.isArray(value)) return String(value[0] ?? '')
  return typeof value === 'string' ? value : ''
}

function positivePage(value: unknown): number {
  const parsed = Number.parseInt(firstQueryValue(value), 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1
}

function promptSort(value: unknown): PromptSort {
  const normalized = firstQueryValue(value)
  return normalized === 'latest' || normalized === 'popular' || normalized === 'featured'
    ? normalized
    : 'featured'
}

function referenceRequirement(value: unknown): PromptReferenceRequirement | '' {
  const normalized = firstQueryValue(value)
  return normalized === 'none' || normalized === 'optional' || normalized === 'required'
    ? normalized
    : ''
}

export function readPromptFilters(query: LocationQuery | Record<string, unknown>): PromptFiltersState {
  return {
    q: firstQueryValue(query.q).trim(),
    purpose: firstQueryValue(query.purpose),
    style: firstQueryValue(query.style),
    subject: firstQueryValue(query.subject),
    model: firstQueryValue(query.model),
    size: firstQueryValue(query.size),
    reference: referenceRequirement(query.reference),
    featured: firstQueryValue(query.featured) === 'true',
    favorite: firstQueryValue(query.favorite) === 'true',
    sort: promptSort(query.sort),
    page: positivePage(query.page),
  }
}

export function toPromptQuery(filters: PromptFiltersState): LocationQueryRaw {
  const query: LocationQueryRaw = {}
  const textEntries: Array<[keyof PromptFiltersState, string]> = [
    ['q', filters.q.trim()],
    ['purpose', filters.purpose],
    ['style', filters.style],
    ['subject', filters.subject],
    ['model', filters.model],
    ['size', filters.size],
    ['reference', filters.reference],
  ]

  for (const [key, value] of textEntries) {
    if (value) query[key] = value
  }
  if (filters.featured) query.featured = 'true'
  if (filters.favorite) query.favorite = 'true'
  if (filters.sort !== DEFAULT_PROMPT_FILTERS.sort || filters.featured) query.sort = filters.sort
  if (filters.page > 1) query.page = String(filters.page)
  return query
}

export function promptSessionStorageKey(id: string, version: number): string {
  return `prompt-library:${id}:${version}`
}

export function storePromptUsePayload(payload: PromptUseResult): void {
  const stored: PromptUseResult = {
    prompt_id: payload.prompt_id,
    version: payload.version,
    title: payload.title,
    prompt_template: payload.prompt_template,
    variables: payload.variables,
    recommended_models: payload.recommended_models,
    recommended_sizes: payload.recommended_sizes,
    reference_requirement: payload.reference_requirement,
  }
  sessionStorage.setItem(
    promptSessionStorageKey(payload.prompt_id, payload.version),
    JSON.stringify(stored),
  )
}

export async function openPromptInImageStudio(
  id: string,
  loadCurrentVersion: (promptId: string) => Promise<PromptUseResult>,
  router: Pick<Router, 'push'>,
): Promise<void> {
  const payload = await loadCurrentVersion(id)
  storePromptUsePayload(payload)
  await router.push(
    `/image-studio?prompt=${encodeURIComponent(payload.prompt_id)}&version=${encodeURIComponent(String(payload.version))}`,
  )
}

export const PROMPT_SOURCE_LABELS: Record<PromptSourceAttribution, string> = {
  original: '极速蹬原创',
  authorized: '极速蹬授权',
  curated: '极速蹬精选',
  community: '极速蹬社区精选',
}

export const PROMPT_STATUS_LABELS: Record<AdminPromptStatus, string> = {
  draft: '草稿',
  pending_review: '待审核',
  published: '已发布',
  offline: '已下线',
}

export function promptSourceLabel(source: PromptSourceAttribution): string {
  return PROMPT_SOURCE_LABELS[source]
}

export function promptStatusLabel(status: AdminPromptStatus): string {
  return PROMPT_STATUS_LABELS[status]
}

export function referenceRequirementLabel(requirement: PromptReferenceRequirement): string {
  const labels: Record<PromptReferenceRequirement, string> = {
    none: '无需参考图',
    optional: '可选参考图',
    required: '需要参考图',
  }
  return labels[requirement]
}

export function createDefaultAdminPromptDraft(): AdminPromptDraft {
  return {
    title: '',
    purpose_description: '',
    prompt_template: '',
    variables: [],
    preview_image_url: '',
    recommended_models: [],
    recommended_sizes: [],
    reference_requirement: 'none',
    reference_instructions: '',
    source_attribution: 'curated',
    source_evidence_summary: '',
    source_evidence_verified: false,
    source_evidence_captured_at: '',
    source_author: '',
    source_url: '',
    featured: false,
    purpose: '',
    style: '',
    subject: '',
    content_notice: '',
  }
}
