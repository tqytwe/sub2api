import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchPublicHomeStats, type PublicHomeStatsResponse } from '@/api/publicHomeStats'
import {
  HOME_LIVE_STATS_STORAGE_KEY,
  formatHomeStatLatency,
  formatHomeStatRequests,
  formatHomeStatUptime,
  loadHomeStatsSnapshot,
  toHomeStatsValues,
} from '@/utils/homeLiveStats'

const LIVE_POLL_MS = 60_000

export function useHomeLiveStats() {
  const realSnapshot = ref<PublicHomeStatsResponse | null>(null)
  const isStale = ref(false)
  let pollTimer: ReturnType<typeof setInterval> | null = null

  function save(snapshot: PublicHomeStatsResponse) {
    try {
      localStorage.setItem(HOME_LIVE_STATS_STORAGE_KEY, JSON.stringify(snapshot))
    } catch {
      // A real in-memory snapshot remains usable when storage is unavailable.
    }
  }

  async function pullLive() {
    if (!navigator.onLine) {
      isStale.value = realSnapshot.value !== null
      return
    }
    const data = await fetchPublicHomeStats()
    if (!data) {
      isStale.value = realSnapshot.value !== null
      return
    }
    realSnapshot.value = data
    isStale.value = false
    save(data)
  }

  const values = computed(() => toHomeStatsValues(realSnapshot.value))

  const statItems = computed(() => {
    const value = values.value
    return [
      { key: 'requests' as const, value: formatHomeStatRequests(value.requests), unit: value.requests == null ? '' : '+' },
      { key: 'uptime' as const, value: formatHomeStatUptime(value.uptimePct), unit: value.uptimePct == null ? '' : '%' },
      { key: 'latency' as const, value: formatHomeStatLatency(value.latencyMs), unit: value.latencyMs == null ? '' : 'ms' },
    ]
  })

  onMounted(() => {
    realSnapshot.value = loadHomeStatsSnapshot(localStorage.getItem(HOME_LIVE_STATS_STORAGE_KEY))
    void pullLive()
    pollTimer = setInterval(() => void pullLive(), LIVE_POLL_MS)
  })

  onBeforeUnmount(() => {
    if (pollTimer) clearInterval(pollTimer)
  })

  return { statItems, values, isStale }
}
