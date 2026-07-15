import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchPublicHomeStats } from '@/api/publicHomeStats'
import {
  formatHomeStatCompact,
  formatHomeStatNumber,
  formatHomeStatPercent,
  normalizeHomeLiveStats,
  type HomeLiveStatsValues,
} from '@/utils/homeLiveStats'

const LIVE_POLL_MS = 30_000

export function useHomeLiveStats() {
  const values = ref<HomeLiveStatsValues | null>(null)
  const loading = ref(true)
  let pollTimer: ReturnType<typeof setInterval> | null = null

  async function pullLive() {
    const data = await fetchPublicHomeStats()
    if (data) values.value = normalizeHomeLiveStats(data)
    loading.value = false
  }

  const statItems = computed(() => {
    const v = values.value
    return [
      { key: 'requests' as const, value: formatHomeStatNumber(v?.requests24h ?? 0), unit: '' },
      { key: 'users' as const, value: formatHomeStatNumber(v?.activeUsers7d ?? 0), unit: '' },
      { key: 'uptime' as const, value: formatHomeStatPercent(v?.successRate30d), unit: v?.successRate30d == null ? '' : '%' },
      { key: 'tokens' as const, value: formatHomeStatCompact(v?.tokensTotal ?? 0), unit: '' },
    ]
  })

  onMounted(() => {
    void pullLive()
    pollTimer = setInterval(() => void pullLive(), LIVE_POLL_MS)
  })

  onBeforeUnmount(() => {
    if (pollTimer) clearInterval(pollTimer)
  })

  return { statItems, values, loading, refresh: pullLive }
}
