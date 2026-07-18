export interface BatchImageSubmissionAttempt {
  fingerprint: string
  key: string
}

export type BatchImageAvailabilityIssue =
  | 'runtimeNotReady'
  | 'disabled'
  | 'groupDisabled'
  | 'noCompatibleAccount'
  | 'noModelsHint'
  | 'pricingMissing'

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
