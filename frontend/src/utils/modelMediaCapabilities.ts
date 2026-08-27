export const MODEL_MEDIA_MODALITIES = ['chat', 'image', 'video', 'audio'] as const
export const MODEL_IMAGE_MEDIA_OPERATIONS = ['create', 'edit'] as const
export const MODEL_VIDEO_MEDIA_OPERATIONS = ['generate'] as const

export type ModelMediaModality = typeof MODEL_MEDIA_MODALITIES[number]

export interface ModelImageMediaCapabilities extends Record<string, unknown> {
  operations: string[]
  sizing_kind?: string
  supported_sizes?: string[]
  min_dimension?: number
  max_dimension?: number
  dimension_step?: number
  max_aspect_ratio?: number
  supported_aspect_ratios?: string[]
  supported_output_formats?: string[]
  max_reference_images?: number
}

export interface ModelVideoMediaCapabilities extends Record<string, unknown> {
  operations: string[]
  supported_resolutions?: string[]
  supported_aspect_ratios?: string[]
  durations_seconds?: number[]
  max_reference_assets?: number
}

/**
 * Persisted catalog metadata. Keep this deliberately data-shaped: provider
 * extensions survive an administrator edit rather than being erased by UI.
 */
export interface ModelMediaCapabilities extends Record<string, unknown> {
  version: string
  adapter: string
  // This stays open-ended so an administrator cannot erase a newer
  // server-declared modality before this frontend adds a dedicated control.
  modalities: string[]
  image?: ModelImageMediaCapabilities
  video?: ModelVideoMediaCapabilities
}

export type MediaCapabilityValidationCode =
  | 'modalities_required'
  | 'version_required'
  | 'adapter_required'
  | 'image_operations_required'
  | 'video_operations_required'
  | 'image_operations_invalid'
  | 'video_operations_invalid'
  | 'image_limits_invalid'
  | 'video_limits_invalid'
  | 'image_modality_required'
  | 'video_modality_required'

export function emptyMediaCapabilities(): ModelMediaCapabilities {
  return {
    version: 'v1',
    adapter: '',
    modalities: [],
  }
}

export function normalizeMediaCapabilities(value: unknown): ModelMediaCapabilities | null {
  if (!isRecord(value)) return null

  const result: ModelMediaCapabilities = {
    ...value,
    version: stringValue(value.version),
    adapter: stringValue(value.adapter),
    modalities: normalizeDeclaredModalities(value.modalities),
  }
  const image = normalizeImageCapabilities(value.image)
  const video = normalizeVideoCapabilities(value.video)
  if (image) result.image = image
  else delete result.image
  if (video) result.video = video
  else delete result.video
  return result
}

export function cloneMediaCapabilities(value: ModelMediaCapabilities | null | undefined): ModelMediaCapabilities {
  const normalized = normalizeMediaCapabilities(value)
  if (!normalized) return emptyMediaCapabilities()
  return JSON.parse(JSON.stringify(normalized)) as ModelMediaCapabilities
}

export function validateMediaCapabilities(value: ModelMediaCapabilities): MediaCapabilityValidationCode[] {
  const modalities = normalizeDeclaredModalities(value.modalities)
  if (modalities.length === 0) return ['modalities_required']

  const errors: MediaCapabilityValidationCode[] = []
  if (!value.version.trim()) errors.push('version_required')
  if (!value.adapter.trim()) errors.push('adapter_required')

  const hasImage = modalities.includes('image')
  const hasVideo = modalities.includes('video')
  if (hasImage) {
    if (!hasOperations(value.image)) errors.push('image_operations_required')
    else if (!hasOnlyCanonicalOperations(value.image?.operations, MODEL_IMAGE_MEDIA_OPERATIONS)) errors.push('image_operations_invalid')
    if (hasInvalidImageLimits(value.image)) errors.push('image_limits_invalid')
  }
  if (hasVideo) {
    if (!hasOperations(value.video)) errors.push('video_operations_required')
    else if (!hasOnlyCanonicalOperations(value.video?.operations, MODEL_VIDEO_MEDIA_OPERATIONS)) errors.push('video_operations_invalid')
    if (hasInvalidVideoLimits(value.video)) errors.push('video_limits_invalid')
  }
  if (!hasImage && value.image && hasOperations(value.image)) errors.push('image_modality_required')
  if (!hasVideo && value.video && hasOperations(value.video)) errors.push('video_modality_required')
  return errors
}

export function hasMediaModality(capabilities: ModelMediaCapabilities | null | undefined, modality: ModelMediaModality): boolean {
  return knownMediaModalities(capabilities).includes(modality)
}

/**
 * Only these stable modalities receive first-class editor controls. Unknown
 * persisted values remain in the declaration and are preserved on save.
 */
export function knownMediaModalities(capabilities: ModelMediaCapabilities | null | undefined): ModelMediaModality[] {
  if (!capabilities) return []
  return normalizeDeclaredModalities(capabilities.modalities)
    .filter((value): value is ModelMediaModality => isKnownMediaModality(value))
}

function normalizeImageCapabilities(value: unknown): ModelImageMediaCapabilities | null {
  if (!isRecord(value)) return null
  return {
    ...value,
    operations: stringArray(value.operations),
    supported_sizes: optionalStringArray(value.supported_sizes),
    supported_aspect_ratios: optionalStringArray(value.supported_aspect_ratios),
    supported_output_formats: optionalStringArray(value.supported_output_formats),
    sizing_kind: optionalString(value.sizing_kind),
    min_dimension: optionalNumber(value.min_dimension),
    max_dimension: optionalNumber(value.max_dimension),
    dimension_step: optionalNumber(value.dimension_step),
    max_aspect_ratio: optionalNumber(value.max_aspect_ratio),
    max_reference_images: optionalNumber(value.max_reference_images),
  }
}

function normalizeVideoCapabilities(value: unknown): ModelVideoMediaCapabilities | null {
  if (!isRecord(value)) return null
  return {
    ...value,
    operations: stringArray(value.operations),
    supported_resolutions: optionalStringArray(value.supported_resolutions),
    supported_aspect_ratios: optionalStringArray(value.supported_aspect_ratios),
    durations_seconds: optionalNumberArray(value.durations_seconds),
    max_reference_assets: optionalNumber(value.max_reference_assets),
  }
}

function normalizeDeclaredModalities(value: unknown): string[] {
  return stringArray(value)
}

function isKnownMediaModality(value: string): value is ModelMediaModality {
  return (MODEL_MEDIA_MODALITIES as readonly string[]).includes(value)
}

function hasOperations(value: { operations?: string[] } | undefined): boolean {
  const operations = value?.operations
  return Array.isArray(operations) && operations.some((operation) => operation.trim().length > 0)
}

function hasOnlyCanonicalOperations(operations: string[] | undefined, allowed: readonly string[]): boolean {
  if (!operations?.length) return false
  const seen = new Set<string>()
  for (const operation of operations) {
    const normalized = operation.trim().toLowerCase()
    if (!normalized || !allowed.includes(normalized) || seen.has(normalized)) return false
    seen.add(normalized)
  }
  return true
}

function hasInvalidImageLimits(image: ModelImageMediaCapabilities | undefined): boolean {
  if (!image) return false
  const values = [
    image.min_dimension,
    image.max_dimension,
    image.dimension_step,
    image.max_aspect_ratio,
    image.max_reference_images,
  ]
  if (values.some((value) => typeof value === 'number' && value < 0)) return true
  return image.min_dimension != null && image.max_dimension != null
    && image.min_dimension > 0 && image.max_dimension > 0
    && image.min_dimension > image.max_dimension
}

function hasInvalidVideoLimits(video: ModelVideoMediaCapabilities | undefined): boolean {
  if (!video) return false
  const referenceLimits = [
    video.max_reference_assets,
    video.max_reference_images,
    video.max_reference_videos,
    video.max_reference_audios,
  ]
  if (referenceLimits.some((value) => typeof value === 'number' && value < 0)) return true

  const durations = video.durations_seconds ?? []
  const seen = new Set<number>()
  for (const duration of durations) {
    if (!Number.isInteger(duration) || duration <= 0 || seen.has(duration)) return true
    seen.add(duration)
  }
  return false
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return [...new Set(value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean))]
}

function optionalStringArray(value: unknown): string[] | undefined {
  return Array.isArray(value) ? stringArray(value) : undefined
}

function optionalNumberArray(value: unknown): number[] | undefined {
  if (!Array.isArray(value)) return undefined
  return value.filter((item): item is number => typeof item === 'number' && Number.isFinite(item))
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
