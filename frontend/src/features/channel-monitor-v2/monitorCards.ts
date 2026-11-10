import type { HealthState, MonitorCandyHistory, MonitorCandyResult, MonitorCoverage, MonitorMatrixBucket, MonitorMatrixRow } from '@/api/channelMonitorV2'

export const CANDY_HISTORY_LIMIT_MIN = 15
export const CANDY_HISTORY_LIMIT_MAX = 100
export const CANDY_HISTORY_LIMIT_DEFAULT = 100
export const MONITOR_CARD_SLOT_COUNT = 18
export const MONITOR_CARD_SAMPLE_BUCKET_MS = 5 * 60 * 1000
export type MonitorCardTimelineMode = 'windows' | 'samples'

export function monitorCardTimelineMode(value?: string): MonitorCardTimelineMode {
  return value === 'samples' ? 'samples' : 'windows'
}

export function candyHistoryLimit(history?: { history_limit?: number } | null): number {
  const limit = history?.history_limit
  return typeof limit === 'number' && Number.isInteger(limit) && limit >= CANDY_HISTORY_LIMIT_MIN && limit <= CANDY_HISTORY_LIMIT_MAX
    ? limit
    : CANDY_HISTORY_LIMIT_DEFAULT
}

export function candyHistorySlots(history: MonitorCandyHistory): Array<MonitorCandyResult | null> {
  const limit = candyHistoryLimit(history)
  const results = history.results.slice(-limit)
  return [...Array<null>(limit - results.length).fill(null), ...results]
}

export function monitorCandyHistories(row: Pick<MonitorMatrixRow, 'candy' | 'candy_checks'>): MonitorCandyHistory[] {
  return row.candy_checks?.length ? row.candy_checks : row.candy ? [row.candy] : []
}

export function hasMonitorSamples(metric: MonitorMatrixBucket['metrics']): boolean {
  return metric.has_samples === true || metric.request_count > 0
}

export interface MonitorCardBar {
  start: number
  end: number
  state: HealthState
  buckets: MonitorMatrixBucket[]
}

function coverageBounds(coverage?: MonitorCoverage) {
  const start = Date.parse(coverage?.requested_start || '')
  const end = Date.parse(coverage?.requested_end || coverage?.data_through || '')
  return Number.isFinite(start) && Number.isFinite(end) && end > start ? { start, end } : null
}

export function monitorCardSampleTimeline(row: MonitorMatrixRow, coverage?: MonitorCoverage): MonitorCardBar[] {
  const source = row.sample_buckets || row.buckets
  const bounds = row.sample_buckets ? null : coverageBounds(coverage)
  const width = row.sample_buckets ? MONITOR_CARD_SAMPLE_BUCKET_MS : (coverage?.bucket_seconds || 0) * 1000
  return source
    .map((bucket) => ({ bucket, start: Date.parse(bucket.bucket_start) }))
    .filter(({ bucket, start }) => Number.isFinite(start) && hasMonitorSamples(bucket.metrics) && (!bounds || (start >= bounds.start && start < bounds.end)))
    .sort((a, b) => a.start - b.start)
    .slice(-MONITOR_CARD_SLOT_COUNT)
    .map(({ bucket, start }) => ({
      start,
      end: width > 0 ? start + width : start,
      // Health also scores latency/cache evidence; request volume alone cannot
      // override the server verdict. Truly insufficient evidence stays unknown.
      state: bucket.health.overall,
      buckets: [bucket],
    }))
}

export function monitorCardWindowTimeline(row: MonitorMatrixRow, coverage?: MonitorCoverage): MonitorCardBar[] {
  const bounds = coverageBounds(coverage)
  if (!bounds) return []
  const width = (bounds.end - bounds.start) / MONITOR_CARD_SLOT_COUNT
  const bars: MonitorCardBar[] = Array.from({ length: MONITOR_CARD_SLOT_COUNT }, (_, index) => ({
    start: bounds.start + index * width,
    end: bounds.start + (index + 1) * width,
    state: 'unknown',
    buckets: [],
  }))
  const severity: Record<HealthState, number> = { unknown: 0, healthy: 1, warning: 2, critical: 3 }
  for (const bucket of row.buckets) {
    const start = Date.parse(bucket.bucket_start)
    if (!Number.isFinite(start) || start < bounds.start || start >= bounds.end || !hasMonitorSamples(bucket.metrics)) continue
    const bar = bars[Math.floor((start - bounds.start) / width)]
    if (!bar) continue
    bar.buckets.push(bucket)
    if (severity[bucket.health.overall] > severity[bar.state]) bar.state = bucket.health.overall
  }
  for (const bar of bars) bar.buckets.sort((a, b) => Date.parse(a.bucket_start) - Date.parse(b.bucket_start))
  return bars
}

export function monitorCardTimeline(row: MonitorMatrixRow, coverage: MonitorCoverage | undefined, mode: MonitorCardTimelineMode) {
  return mode === 'samples' ? monitorCardSampleTimeline(row, coverage) : monitorCardWindowTimeline(row, coverage)
}

export function candyDisplayState(history: MonitorCandyHistory, now: number) {
  const result = history.results.at(-1)
  if (!result) return 'unknown' as const
  const age = now - Date.parse(result.checked_at)
  if (!Number.isFinite(age) || age > Math.max(180, history.interval_minutes * 60 + 120) * 1000) return 'stale' as const
  return result.verdict
}

export function monitorRefreshSeconds(configSeconds: number | undefined, rows: readonly MonitorMatrixRow[], bootstrap: boolean): number {
  if (bootstrap) return 10
  let seconds = configSeconds && Number.isFinite(configSeconds) && configSeconds > 0 ? configSeconds : 300
  for (const row of rows) {
    for (const history of monitorCandyHistories(row)) {
      if (history.interval_minutes > 0 && Number.isFinite(history.interval_minutes)) {
        seconds = Math.min(seconds, history.interval_minutes * 60)
      }
    }
  }
  return Math.max(60, seconds)
}
