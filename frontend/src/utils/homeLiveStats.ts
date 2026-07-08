/** Marketing home stats: time-based growth with optional live API overlay. */

export const HOME_LIVE_STATS_STORAGE_KEY = 'home_live_stats_v1'

/** Anchor baseline (matches legacy static homepage). */
export const HOME_STATS_BASE_REQUESTS = 12_847_360
export const HOME_STATS_BASE_UPTIME = 99.97
export const HOME_STATS_BASE_LATENCY_MS = 386

/** ~48 req/s exaggerated growth while the tab is "active". */
export const HOME_STATS_REQUESTS_PER_MS = 48 / 1000

export interface HomeLiveStatsPersisted {
  /** Wall-clock anchor for first-run bootstrap. */
  anchorMs: number
  /** Milliseconds that advanced counters (includes offline catch-up). */
  creditedMs: number
  /** When the current offline spell started (null if online). */
  offlineSinceMs: number | null
}

export interface HomeLiveRealSnapshot {
  totalRequests: number
  availabilityPct: number | null
  avgTtftMs: number | null
  fetchedAtMs: number
}

export interface HomeLiveStatsValues {
  requests: number
  uptimePct: number
  latencyMs: number
}

export function defaultPersisted(now = Date.now()): HomeLiveStatsPersisted {
  return {
    anchorMs: now,
    creditedMs: 0,
    offlineSinceMs: null,
  }
}

export function loadPersisted(raw: string | null, now = Date.now()): HomeLiveStatsPersisted {
  if (!raw) return defaultPersisted(now)
  try {
    const parsed = JSON.parse(raw) as Partial<HomeLiveStatsPersisted>
    if (typeof parsed.anchorMs !== 'number' || typeof parsed.creditedMs !== 'number') {
      return defaultPersisted(now)
    }
    return {
      anchorMs: parsed.anchorMs,
      creditedMs: Math.max(0, parsed.creditedMs),
      offlineSinceMs: typeof parsed.offlineSinceMs === 'number' ? parsed.offlineSinceMs : null,
    }
  } catch {
    return defaultPersisted(now)
  }
}

/** Mark offline — counter pauses until back online. */
export function markOffline(state: HomeLiveStatsPersisted, now = Date.now()): HomeLiveStatsPersisted {
  if (state.offlineSinceMs != null) return state
  return { ...state, offlineSinceMs: now }
}

/** Resume online and credit the full offline duration (catch-up). */
export function markOnline(state: HomeLiveStatsPersisted, now = Date.now()): HomeLiveStatsPersisted {
  if (state.offlineSinceMs == null) return state
  const offlineMs = Math.max(0, now - state.offlineSinceMs)
  return {
    anchorMs: state.anchorMs,
    creditedMs: state.creditedMs + offlineMs,
    offlineSinceMs: null,
  }
}

/** Advance credited time while online (call each tick). */
export function advanceOnline(
  state: HomeLiveStatsPersisted,
  deltaMs: number,
  now = Date.now(),
): HomeLiveStatsPersisted {
  if (deltaMs <= 0 || state.offlineSinceMs != null) return state
  return {
    ...state,
    creditedMs: state.creditedMs + deltaMs,
  }
}

export function syntheticRequests(creditedMs: number): number {
  return HOME_STATS_BASE_REQUESTS + Math.floor(creditedMs * HOME_STATS_REQUESTS_PER_MS)
}

export function syntheticUptimePct(creditedMs: number): number {
  const hours = creditedMs / 3_600_000
  const wave = Math.sin(hours / 24) * 0.04 + Math.sin(hours / 6) * 0.015
  return clamp(homeStatsRound(HOME_STATS_BASE_UPTIME + wave, 2), 99.9, 99.99)
}

export function syntheticLatencyMs(creditedMs: number): number {
  const hours = creditedMs / 3_600_000
  const wave = Math.sin(hours / 12) * 14 + Math.sin(hours / 3) * 9
  return Math.round(clamp(HOME_STATS_BASE_LATENCY_MS + wave, 340, 430))
}

export function mergeHomeLiveStats(
  creditedMs: number,
  real: HomeLiveRealSnapshot | null,
): HomeLiveStatsValues {
  const synReq = syntheticRequests(creditedMs)
  const synUp = syntheticUptimePct(creditedMs)
  const synLat = syntheticLatencyMs(creditedMs)

  const realReq = real?.totalRequests ?? 0
  const requests = Math.max(synReq, realReq)

  let uptimePct = synUp
  if (real?.availabilityPct != null && Number.isFinite(real.availabilityPct)) {
    uptimePct = clamp(homeStatsRound(synUp * 0.35 + real.availabilityPct * 0.65, 2), 99.9, 99.99)
  }

  let latencyMs = synLat
  if (real?.avgTtftMs != null && real.avgTtftMs > 0) {
    latencyMs = Math.round(clamp(synLat * 0.4 + real.avgTtftMs * 0.6, 280, 520))
  }

  return { requests, uptimePct, latencyMs }
}

export function formatHomeStatRequests(n: number): string {
  return Math.max(0, Math.floor(n)).toLocaleString('en-US')
}

export function formatHomeStatUptime(pct: number): string {
  return homeStatsRound(pct, 2).toFixed(2)
}

export function formatHomeStatLatency(ms: number): string {
  return String(Math.round(ms))
}

export type HomeStatOdometerChar = {
  ch: string
  digit: number | null
  roll: number
}

export function buildOdometerChars(value: string): HomeStatOdometerChar[] {
  let roll = 0
  return value.split('').map((ch) => ({
    ch,
    digit: /\d/.test(ch) ? Number(ch) : null,
    roll: /\d/.test(ch) ? roll++ : 0,
  }))
}

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n))
}

function homeStatsRound(n: number, digits: number): number {
  const p = 10 ** digits
  return Math.round(n * p) / p
}
