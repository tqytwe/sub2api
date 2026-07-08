import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchPublicHomeStats } from '@/api/publicHomeStats'
import {
  HOME_LIVE_STATS_STORAGE_KEY,
  advanceOnline,
  buildOdometerChars,
  defaultPersisted,
  formatHomeStatLatency,
  formatHomeStatRequests,
  formatHomeStatUptime,
  loadPersisted,
  markOffline,
  markOnline,
  mergeHomeLiveStats,
  type HomeLiveRealSnapshot,
  type HomeLiveStatsPersisted,
} from '@/utils/homeLiveStats'

const TICK_MS = 1000
const LIVE_POLL_MS = 60_000

export function useHomeLiveStats() {
  const persisted = ref<HomeLiveStatsPersisted>(defaultPersisted())
  const realSnapshot = ref<HomeLiveRealSnapshot | null>(null)
  const tick = ref(0)

  let timer: ReturnType<typeof setInterval> | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let lastTickMs = Date.now()

  function save() {
    try {
      localStorage.setItem(HOME_LIVE_STATS_STORAGE_KEY, JSON.stringify(persisted.value))
    } catch {
      /* ignore quota */
    }
  }

  function applyConnectivity(online: boolean) {
    const now = Date.now()
    persisted.value = online ? markOnline(persisted.value, now) : markOffline(persisted.value, now)
    lastTickMs = now
    save()
    tick.value++
  }

  function onTick() {
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
    if (!navigator.onLine) return

    const now = Date.now()
    const delta = now - lastTickMs
    lastTickMs = now
    persisted.value = advanceOnline(persisted.value, delta)
    save()
    tick.value++
  }

  async function pullLive() {
    if (!navigator.onLine) return
    const data = await fetchPublicHomeStats()
    if (!data) return
    realSnapshot.value = {
      totalRequests: data.total_requests ?? 0,
      availabilityPct: data.availability_pct,
      avgTtftMs: data.avg_ttft_ms,
      fetchedAtMs: Date.now(),
    }
    tick.value++
  }

  const values = computed(() => {
    void tick.value
    return mergeHomeLiveStats(persisted.value.creditedMs, realSnapshot.value)
  })

  const statItems = computed(() => {
    const v = values.value
    const defs = [
      { key: 'requests' as const, value: formatHomeStatRequests(v.requests), unit: '+' },
      { key: 'uptime' as const, value: formatHomeStatUptime(v.uptimePct), unit: '%' },
      { key: 'latency' as const, value: formatHomeStatLatency(v.latencyMs), unit: 'ms' },
    ]
    return defs.map((d) => ({
      ...d,
      chars: buildOdometerChars(d.value),
    }))
  })

  onMounted(() => {
    persisted.value = loadPersisted(localStorage.getItem(HOME_LIVE_STATS_STORAGE_KEY))
    lastTickMs = Date.now()
    applyConnectivity(navigator.onLine)

    void pullLive()
    timer = setInterval(onTick, TICK_MS)
    pollTimer = setInterval(() => void pullLive(), LIVE_POLL_MS)

    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    document.addEventListener('visibilitychange', onVisibility)
  })

  function onOnline() {
    applyConnectivity(true)
    void pullLive()
  }

  function onOffline() {
    applyConnectivity(false)
  }

  function onVisibility() {
    if (document.visibilityState === 'visible' && navigator.onLine) {
      lastTickMs = Date.now()
    }
  }

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
    if (pollTimer) clearInterval(pollTimer)
    window.removeEventListener('online', onOnline)
    window.removeEventListener('offline', onOffline)
    document.removeEventListener('visibilitychange', onVisibility)
  })

  return { statItems, values }
}
