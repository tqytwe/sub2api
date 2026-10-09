import type { UsageLog } from '@/types'

type UsageTpsRow = Partial<Pick<UsageLog,
  'output_tps' | 'output_tokens' | 'duration_ms' | 'first_token_ms' |
  'image_count' | 'image_output_tokens' | 'video_count' | 'billing_mode' | 'media_type'
>>

// The API owns the per-record calculation and media eligibility. In particular,
// do not replace null with a locally recomputed value during rolling upgrades.
export function usageOutputTps(row: UsageTpsRow | null | undefined): number | null {
  if (!row) return null
  if (row.output_tps !== undefined) {
    const value = row.output_tps
    return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : null
  }

  // Old API responses omit the field. Apply the same eligibility rules only in
  // that case; an explicit null from a newer server remains authoritative.
  const { output_tokens: tokens, duration_ms: duration } = row
  if (typeof tokens !== 'number' || !Number.isFinite(tokens) || tokens < 2 ||
      typeof duration !== 'number' || !Number.isFinite(duration) || duration <= 0 ||
      (row.image_count ?? 0) > 0 || (row.image_output_tokens ?? 0) > 0 || (row.video_count ?? 0) > 0 ||
      row.billing_mode === 'image' || row.billing_mode === 'video' ||
      row.media_type === 'image' || row.media_type === 'video') return null
  const value = tokens / (duration / 1000)
  return Number.isFinite(value) && value > 0 ? value : null
}

export function formatUsageOutputTps(row: UsageTpsRow | null | undefined): string | null {
  const value = usageOutputTps(row)
  if (value == null) return null
  if (value < 0.1) return '<0.1 t/s'
  return `${value >= 100 ? Math.round(value) : value.toFixed(1)} t/s`
}
