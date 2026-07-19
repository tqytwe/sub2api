import type {
  BatchImageReferenceImage,
  BatchImageSubmitItem,
} from '@/api/batchImage'

export interface BatchImageSubmissionAttempt {
  fingerprint: string
  key: string
}

export interface BatchImagePromptRowInput {
  custom_id?: string
  prompt: string
  output_count?: number
  reference_images?: BatchImageReferenceImage[]
}

export type BatchImageAvailabilityIssue =
  | 'runtimeNotReady'
  | 'disabled'
  | 'groupDisabled'
  | 'noCompatibleAccount'
  | 'noModelsHint'
  | 'pricingMissing'

const BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM = 4

export function createBatchImageIdempotencyKey(prefix = 'sub2api-ui'): string {
  const uuid = globalThis.crypto?.randomUUID?.()
  if (uuid) return `${prefix}-${uuid}`
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 12)}`
}

export function resolveBatchImageSubmissionAttempt(
  previous: BatchImageSubmissionAttempt | null,
  payload: unknown,
  createKey: () => string = () => createBatchImageIdempotencyKey(),
): BatchImageSubmissionAttempt {
  const fingerprint = JSON.stringify(payload)
  if (previous?.fingerprint === fingerprint) return previous
  return { fingerprint, key: createKey() }
}

export function normalizeBatchImageOutputCount(value: unknown): number {
  const parsed = Math.floor(Number(value || 1))
  if (!Number.isFinite(parsed)) return 1
  return Math.min(BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM, Math.max(1, parsed))
}

export function uniqueBatchImageCustomID(raw: string, used: Set<string>, index: number): string {
  const base = raw.replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') ||
    `img_${String(index + 1).padStart(3, '0')}`
  let candidate = base
  let suffix = 2
  while (used.has(candidate)) {
    candidate = `${base}_${suffix}`
    suffix += 1
  }
  used.add(candidate)
  return candidate
}

export function buildBatchImageSubmitItems(rows: BatchImagePromptRowInput[]): BatchImageSubmitItem[] {
  const used = new Set<string>()
  return rows
    .map((row, index) => {
      const customID = uniqueBatchImageCustomID(
        row.custom_id || `img_${String(index + 1).padStart(3, '0')}`,
        used,
        index,
      )
      const item: BatchImageSubmitItem = {
        custom_id: customID,
        prompt: String(row.prompt || '').trim(),
      }
      const outputCount = normalizeBatchImageOutputCount(row.output_count)
      if (outputCount > 1) item.output_count = outputCount
      if (row.reference_images?.length) item.reference_images = row.reference_images
      return item
    })
    .filter(item => item.prompt)
}

export function batchImageAvailabilityIssue(error: { code?: unknown; status?: unknown } | null | undefined): BatchImageAvailabilityIssue | null {
  const code = String(error?.code || '').trim()
  const status = Number(error?.status || 0)
  if (code === 'BATCH_IMAGE_NOT_READY' && status === 503) return 'runtimeNotReady'
  if (code === 'BATCH_IMAGE_DISABLED') return 'disabled'
  if (code === 'BATCH_IMAGE_GROUP_DISABLED') return 'groupDisabled'
  if (code === 'BATCH_IMAGE_NO_ACCOUNT_AVAILABLE') return 'noCompatibleAccount'
  if (code === 'BATCH_IMAGE_NO_MODEL_AVAILABLE') return 'noModelsHint'
  if (code === 'BATCH_IMAGE_PRICING_NOT_READY' || code === 'BATCH_IMAGE_SETTLEMENT_PRICING_MISSING') return 'pricingMissing'
  return null
}
