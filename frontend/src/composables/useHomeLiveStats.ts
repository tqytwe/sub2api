import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchPublicStatusSummary, type PublicHomeStatsResponse } from '@/api/publicHomeStats'
import {
  HOME_LIVE_STATS_STORAGE_KEY,
  formatHomeStatLatency,
  formatHomeStatRequests,
  formatHomeStatUptime,
  isHomeStatsSnapshotStale,
  loadHomeStatsSnapshot,
  freshnessFromSnapshot,
  statusSummaryToHomeStats,
  toHomeStatsValues,
} from '@/utils/homeLiveStats'

const LIVE_POLL_MS = 60_000
const DATA_DELAY_MS = 90 * 60_000
const DATA_UNAVAILABLE_MS = 6 * 60 * 60_000

export function useHomeLiveStats() {
  const realSnapshot = ref<PublicHomeStatsResponse | null>(null)
  const networkStale = ref(false)
  const freshness = ref<'fresh' | 'delayed' | 'unavailable'>('unavailable')
  const nowMs = ref(Date.now())
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let freshnessTimer: ReturnType<typeof setTimeout> | null = null

  function clearFreshnessTimer() {
    if (!freshnessTimer) return
    clearTimeout(freshnessTimer)
    freshnessTimer = null
  }

  function scheduleFreshnessRecheck(snapshot: PublicHomeStatsResponse | null) {
    clearFreshnessTimer()
    const throughMs = snapshot?.ops_data_through ? Date.parse(snapshot.ops_data_through) : NaN
    if (!Number.isFinite(throughMs)) return

    const current = Date.now()
    const boundaries = [throughMs + DATA_DELAY_MS, throughMs + DATA_UNAVAILABLE_MS]
    const boundary = boundaries.find((value) => value > current)
    if (!boundary) return

    freshnessTimer = setTimeout(() => {
      nowMs.value = Date.now()
      freshness.value = freshnessFromSnapshot(realSnapshot.value, nowMs.value)
      scheduleFreshnessRecheck(realSnapshot.value)
    }, Math.max(1, boundary - current + 1))
  }

  function save(snapshot: PublicHomeStatsResponse) {
    try {
      localStorage.setItem(HOME_LIVE_STATS_STORAGE_KEY, JSON.stringify(snapshot))
    } catch {
      // A real in-memory snapshot remains usable when storage is unavailable.
    }
  }

  async function pullLive() {
    if (!navigator.onLine) {
      networkStale.value = realSnapshot.value !== null
      return
    }
	const summary = await fetchPublicStatusSummary()
	const data = summary ? statusSummaryToHomeStats(summary) : null
    if (!data) {
      networkStale.value = realSnapshot.value !== null
      freshness.value = 'unavailable'
      return
    }
    realSnapshot.value = data
    networkStale.value = false
    freshness.value = freshnessFromSnapshot(data)
    save(data)
    scheduleFreshnessRecheck(data)
  }

  const values = computed(() => toHomeStatsValues(realSnapshot.value))
  const computedAt = computed(() => realSnapshot.value?.computed_at ?? null)
  const opsDataThrough = computed(() => realSnapshot.value?.ops_data_through ?? null)
  const isStale = computed(
    () =>
      networkStale.value
      || freshness.value !== 'fresh'
      || isHomeStatsSnapshotStale(realSnapshot.value, nowMs.value),
  )

  const statItems = computed(() => {
    const value = values.value
    return [
      { key: 'requests' as const, value: formatHomeStatRequests(value.requests), unit: value.requests == null ? '' : '+' },
      { key: 'uptime' as const, value: formatHomeStatUptime(value.uptimePct), unit: value.uptimePct == null ? '' : '%' },
      { key: 'latency' as const, value: formatHomeStatLatency(value.latencyMs), unit: value.latencyMs == null ? '' : 'ms' },
    ]
  })

  onMounted(() => {
    nowMs.value = Date.now()
    realSnapshot.value = loadHomeStatsSnapshot(localStorage.getItem(HOME_LIVE_STATS_STORAGE_KEY))
    freshness.value = freshnessFromSnapshot(realSnapshot.value, nowMs.value)
    scheduleFreshnessRecheck(realSnapshot.value)
    void pullLive()
    pollTimer = setInterval(() => {
      nowMs.value = Date.now()
      void pullLive()
    }, LIVE_POLL_MS)
  })

  onBeforeUnmount(() => {
    if (pollTimer) clearInterval(pollTimer)
    clearFreshnessTimer()
  })

  return { statItems, values, computedAt, opsDataThrough, isStale, freshness }
}
