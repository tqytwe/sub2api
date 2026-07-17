import { apiClient } from './client'

export type PromptSourceAttribution = 'original' | 'authorized' | 'curated' | 'community'
export type PromptReferenceRequirement = 'none' | 'optional' | 'required'
export type PromptSort = 'featured' | 'latest' | 'popular'
export type PromptCategoryDimension = 'purpose' | 'style' | 'subject' | 'model' | 'size'

export interface PromptVariableDefinition {
  name: string
  label: string
  description?: string
  type?: 'text' | 'number' | 'select' | 'color' | string
  required?: boolean
  default_value?: string | number | null
  options?: Array<string | { label: string; value: string }>
}

export interface PromptCategory {
  id: string | number
  name: string
  slug: string
  dimension: PromptCategoryDimension
  sort_order?: number
}

export interface PromptExampleImage {
  id?: string | number
  url: string
  alt?: string
  width?: number
  height?: number
}

export interface PromptSummary {
  id: string
  slug?: string
  title: string
  purpose_description: string
  prompt_template: string
  variables: PromptVariableDefinition[]
  preview_image_url?: string
  preview_image_alt?: string
  recommended_models: string[]
  recommended_sizes: string[]
  reference_requirement: PromptReferenceRequirement
  reference_instructions?: string
  source_attribution: PromptSourceAttribution
  featured: boolean
  favorite_count?: number
  use_count?: number
  is_favorited?: boolean
  version: number
  purpose?: string
  style?: string
  subject?: string
  created_at?: string
  updated_at?: string
}

export interface PromptDetail extends PromptSummary {
  example_images?: PromptExampleImage[]
  content_notice?: string
  source_notice?: string
}

export interface PromptPagination {
  items: PromptSummary[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface PromptListQuery {
  q?: string
  purpose?: string
  style?: string
  subject?: string
  model?: string
  size?: string
  reference?: PromptReferenceRequirement | ''
  featured?: boolean
  favorite?: boolean
  sort?: PromptSort
  page?: number
  page_size?: number
}

export interface PromptUseResult {
  prompt_id: string
  version: number
  title: string
  prompt_template: string
  variables: PromptVariableDefinition[]
  recommended_models: string[]
  recommended_sizes: string[]
  reference_requirement: PromptReferenceRequirement
}

export interface PromptFavoriteResult {
  favorited: boolean
  favorite_count?: number
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

function stringList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

function sourceAttribution(value: unknown): PromptSourceAttribution {
  if (value === '极速蹬原创' || value === 'original') return 'original'
  if (value === '极速蹬授权' || value === 'authorized') return 'authorized'
  if (value === '极速蹬社区精选' || value === 'community') return 'community'
  return 'curated'
}

function variableFromRecord(name: string, value: unknown): PromptVariableDefinition {
  const item = record(value)
  const label = text(item.label) || text(item.label_zh) || name
  const variable: PromptVariableDefinition = { name, label }
  const description = text(item.description) || text(item.description_zh)
  if (description) variable.description = description
  if (typeof item.type === 'string') variable.type = item.type
  if (typeof item.required === 'boolean') variable.required = item.required
  if (
    typeof item.default_value === 'string'
    || typeof item.default_value === 'number'
    || item.default_value === null
  ) {
    variable.default_value = item.default_value
  }
  if (Array.isArray(item.options)) {
    variable.options = item.options.filter((option): option is string | { label: string; value: string } =>
      typeof option === 'string'
      || (
        !!option
        && typeof option === 'object'
        && typeof (option as UnknownRecord).label === 'string'
        && typeof (option as UnknownRecord).value === 'string'
      ))
  }
  return variable
}

export function normalizePromptVariables(value: unknown): PromptVariableDefinition[] {
  if (Array.isArray(value)) {
    return value.flatMap((item) => {
      const row = record(item)
      const name = text(row.name).trim()
      return name ? [variableFromRecord(name, row)] : []
    })
  }
  const source = record(value)
  if (Array.isArray(source.items)) return normalizePromptVariables(source.items)
  return Object.entries(source).map(([name, definition]) => variableFromRecord(name, definition))
}

function referenceRequirement(value: unknown): PromptReferenceRequirement {
  if (value === 'required' || value === true) return 'required'
  if (value === 'optional') return 'optional'
  return 'none'
}

export function normalizePublicPrompt(value: unknown): PromptDetail {
  const row = record(value)
  const media = Array.isArray(row.media) ? row.media.map(record) : []
  const firstImage = media.find((item) => text(item.media_type) !== 'video') ?? {}
  const id = String(row.id ?? '')
  const purposeDescription = text(row.purpose_description) || text(row.description)
  const promptTemplate = text(row.prompt_template) || text(row.prompt_text)
  const brand = row.source_attribution ?? row.brand_type ?? row.brand_label
  const exampleImages = media
    .filter((item) => !!text(item.url) && text(item.media_type) !== 'video')
    .map((item) => ({
      id: item.id === undefined ? undefined : String(item.id),
      url: text(item.url),
      alt: text(item.alt) || text(item.alt_zh),
    }))

  return {
    id,
    slug: text(row.slug) || undefined,
    title: text(row.title) || text(row.title_zh),
    purpose_description: purposeDescription,
    prompt_template: promptTemplate,
    variables: normalizePromptVariables(row.variables),
    preview_image_url: text(row.preview_image_url) || text(firstImage.url) || undefined,
    preview_image_alt: text(row.preview_image_alt) || text(firstImage.alt_zh) || undefined,
    recommended_models: stringList(row.recommended_models ?? row.models),
    recommended_sizes: stringList(row.recommended_sizes ?? row.sizes),
    reference_requirement: referenceRequirement(
      row.reference_requirement ?? row.requires_reference,
    ),
    reference_instructions: text(row.reference_instructions) || undefined,
    source_attribution: sourceAttribution(brand),
    featured: row.featured === true,
    favorite_count: Number(row.favorite_count) || 0,
    use_count: Number(row.use_count) || 0,
    is_favorited: row.is_favorited === true || row.favorited === true,
    version: Number(row.version ?? row.published_version ?? row.current_version) || 1,
    purpose: text(row.purpose) || undefined,
    style: text(row.style) || undefined,
    subject: text(row.subject) || undefined,
    created_at: text(row.created_at) || undefined,
    updated_at: text(row.updated_at) || undefined,
    example_images: exampleImages,
    content_notice: text(row.content_notice) || undefined,
    source_notice: text(row.public_attribution_note) || undefined,
  }
}

export function normalizePromptUseResult(value: unknown): PromptUseResult {
  const row = record(value)
  return {
    prompt_id: String(row.prompt_id ?? ''),
    version: Number(row.version) || 1,
    title: text(row.title),
    prompt_template: text(row.prompt_template) || text(row.prompt_text),
    variables: normalizePromptVariables(row.variables),
    recommended_models: stringList(row.recommended_models ?? row.models),
    recommended_sizes: stringList(row.recommended_sizes ?? row.sizes),
    reference_requirement: referenceRequirement(
      row.reference_requirement ?? row.requires_reference,
    ),
  }
}

export function normalizePromptCategory(value: unknown): PromptCategory {
  const row = record(value)
  const rawSlug = text(row.slug)
  const [prefix, ...slugParts] = rawSlug.split(':')
  const dimensions: PromptCategoryDimension[] = ['purpose', 'style', 'subject', 'model', 'size']
  const explicitDimension = text(row.dimension)
  const dimension = dimensions.includes(explicitDimension as PromptCategoryDimension)
    ? explicitDimension as PromptCategoryDimension
    : dimensions.includes(prefix as PromptCategoryDimension)
      ? prefix as PromptCategoryDimension
      : 'purpose'
  return {
    id: typeof row.id === 'number' || typeof row.id === 'string' ? row.id : '',
    name: text(row.name) || text(row.name_zh),
    slug: slugParts.length ? slugParts.join(':') : rawSlug,
    dimension,
    sort_order: Number(row.sort_order) || 0,
  }
}

export async function listPrompts(query: PromptListQuery = {}): Promise<PromptPagination> {
  const { data } = await apiClient.get<UnknownRecord>('/prompts', { params: query })
  const items = Array.isArray(data.items) ? data.items.map(normalizePublicPrompt) : []
  return {
    items,
    total: Number(data.total) || 0,
    page: Number(data.page) || 1,
    page_size: Number(data.page_size) || 20,
    pages: Number(data.pages) || (items.length ? 1 : 0),
  }
}

export async function getPrompt(id: string): Promise<PromptDetail> {
  const { data } = await apiClient.get<unknown>(`/prompts/${encodeURIComponent(id)}`)
  return normalizePublicPrompt(data)
}

export async function listPromptCategories(): Promise<PromptCategory[]> {
  const { data } = await apiClient.get<unknown>('/prompt-categories')
  const rows = Array.isArray(data) ? data : record(data).items
  return Array.isArray(rows) ? rows.map(normalizePromptCategory) : []
}

function normalizePromptFavoriteResult(value: unknown, fallback: boolean): PromptFavoriteResult {
  const row = record(value)
  const out: PromptFavoriteResult = {
    favorited: typeof row.favorited === 'boolean' ? row.favorited : fallback,
  }
  if (typeof row.favorite_count === 'number') out.favorite_count = row.favorite_count
  return out
}

export async function favoritePrompt(id: string): Promise<PromptFavoriteResult> {
  const { data } = await apiClient.post<unknown>(`/prompts/${encodeURIComponent(id)}/favorite`)
  return normalizePromptFavoriteResult(data, true)
}

export async function unfavoritePrompt(id: string): Promise<PromptFavoriteResult> {
  const { data } = await apiClient.delete<unknown>(`/prompts/${encodeURIComponent(id)}/favorite`)
  return normalizePromptFavoriteResult(data, false)
}

export async function usePrompt(id: string): Promise<PromptUseResult> {
  const { data } = await apiClient.post<unknown>(`/prompts/${encodeURIComponent(id)}/use`)
  return normalizePromptUseResult(data)
}

export async function reportPrompt(id: string, reason: string, detail: string): Promise<void> {
  await apiClient.post(`/prompts/${encodeURIComponent(id)}/report`, { reason, detail })
}

const promptsAPI = {
  list: listPrompts,
  get: getPrompt,
  categories: listPromptCategories,
  favorite: favoritePrompt,
  unfavorite: unfavoritePrompt,
  use: usePrompt,
  report: reportPrompt,
}

export default promptsAPI
