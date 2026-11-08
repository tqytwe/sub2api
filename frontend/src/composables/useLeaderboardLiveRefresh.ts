import { onBeforeUnmount, onMounted, ref, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'

export interface LeaderboardLiveRefreshOptions<T> {
  fetch: (signal: AbortSignal) => Promise<T>
  enabled?: MaybeRefOrGetter<boolean>
  /** Changing the selection invalidates any old response and refreshes immediately. */
  key?: MaybeRefOrGetter<unknown>
}

/** A visibility-aware, single-flight loop for a lightweight leaderboard endpoint. */
export function useLeaderboardLiveRefresh<T>(options: LeaderboardLiveRefreshOptions<T>) {
  const data = shallowRef<T | null>(null)
  const error = shallowRef<unknown>(null)
  const isRefreshing = ref(false)
  const nextRefreshMs = ref(5000)
  let mounted = false
  let generation = 0
  let failures = 0
  let pendingRefresh = false
  let controller: AbortController | null = null
  let inFlight: Promise<void> | null = null
  let timer: ReturnType<typeof setTimeout> | null = null
  const active = () => mounted && toValue(options.enabled ?? true) && document.visibilityState !== 'hidden'

  function clearTimer() {
    if (timer !== null) clearTimeout(timer)
    timer = null
  }

  function schedule() {
    clearTimer()
    if (active()) timer = setTimeout(() => { void refresh() }, nextRefreshMs.value)
  }

  function refresh(): Promise<void> {
    if (!active()) return Promise.resolve()
    if (inFlight) return inFlight
    clearTimer()
    const requestGeneration = generation
    const requestController = new AbortController()
    controller = requestController
    isRefreshing.value = true
    inFlight = (async () => {
      try {
        // Defer invocation so even a synchronous transport exception settles after
        // inFlight is assigned, preserving single-flight retry behavior.
        await Promise.resolve()
        if (!active() || requestGeneration !== generation) return
        const result = await options.fetch(requestController.signal)
        if (!active() || requestGeneration !== generation) return
        data.value = result
        error.value = null
        failures = 0
        nextRefreshMs.value = 5000
      } catch (cause) {
        if (!active() || requestGeneration !== generation || requestController.signal.aborted) return
        error.value = cause
        failures++
        nextRefreshMs.value = Math.min(failures * 10000, 30000)
      } finally {
        inFlight = null
        controller = null
        isRefreshing.value = false
        if (pendingRefresh && active()) {
          pendingRefresh = false
          void refresh()
        } else schedule()
      }
    })()
    return inFlight
  }

  function invalidate(clearData = false) {
    generation++
    clearTimer()
    controller?.abort()
    if (clearData) {
      data.value = null
      error.value = null
      failures = 0
      nextRefreshMs.value = 5000
    }
    pendingRefresh = active()
    if (!inFlight && pendingRefresh) {
      pendingRefresh = false
      void refresh()
    }
  }

  const onVisibilityChange = () => invalidate()
  watch([() => toValue(options.enabled ?? true), () => toValue(options.key)], (current, previous) => invalidate(current[1] !== previous[1]))
  onMounted(() => {
    mounted = true
    document.addEventListener('visibilitychange', onVisibilityChange)
    void refresh()
  })
  onBeforeUnmount(() => {
    mounted = false
    generation++
    pendingRefresh = false
    clearTimer()
    controller?.abort()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return { data, error, isRefreshing, nextRefreshMs, refresh }
}
